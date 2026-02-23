// Copyright (c) 2025 Electronic Arts Inc. All rights reserved.

package replication

import (
	"context"
	"fmt"
	"time"

	"github.com/go-logr/logr"
	"github.com/hashicorp/go-multierror"
	doltv1alpha "github.com/electronicarts/doltdb-operator/api/v1alpha"
	"github.com/electronicarts/doltdb-operator/pkg/builder"
	"github.com/electronicarts/doltdb-operator/pkg/conditions"
	"github.com/electronicarts/doltdb-operator/pkg/health"
	"github.com/electronicarts/doltdb-operator/pkg/refresolver"
	"github.com/electronicarts/doltdb-operator/pkg/statefulset"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/tools/record"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

// PodReadinessController reconciles a Pod object
type PodReadinessController struct {
	client.Client
	recorder    record.EventRecorder
	builder     *builder.Builder
	refResolver *refresolver.RefResolver
	replConfig  *ReplicationConfig
}

// NewPodReadinessController creates a new PodReadinessController
func NewPodReadinessController(client client.Client, recorder record.EventRecorder, builder *builder.Builder,
	refResolver *refresolver.RefResolver, replConfig *ReplicationConfig) *PodReadinessController {
	return &PodReadinessController{
		Client:      client,
		recorder:    recorder,
		builder:     builder,
		refResolver: refResolver,
		replConfig:  replConfig,
	}
}

// shouldReconcile checks if the DoltDB should be reconciled
func shouldReconcile(doltdb *doltv1alpha.DoltDB) bool {
	primaryRepl := ptr.Deref(doltdb.Replication().Primary, doltv1alpha.PrimaryReplication{})
	return doltdb.Replication().Enabled && *primaryRepl.AutomaticFailover && doltdb.IsReplicationConfigured()
}

// ReconcilePodNotReady reconciles a Pod that is not in a Ready state
func (r *PodReadinessController) ReconcilePodNotReady(ctx context.Context, pod corev1.Pod, doltdb *doltv1alpha.DoltDB) error {
	if !shouldReconcile(doltdb) {
		return nil
	}
	logger := log.FromContext(ctx).WithName("pod-not-ready")

	if doltdb.Status.CurrentPrimaryPodIndex == nil {
		logger.V(1).Info("'status.currentPrimaryPodIndex' must be set. Skipping")
		return nil
	}

	logger.V(1).Info("Reconciling Pod in non Ready state", "pod", pod.Name)

	index, err := statefulset.PodIndex(pod.Name)
	if err != nil {
		return fmt.Errorf("error getting Pod index: %v", err)
	}
	if *index != *doltdb.Status.CurrentPrimaryPodIndex {
		return nil
	}

	fromIndex := doltdb.Status.CurrentPrimaryPodIndex

	// Select failover candidate using replication state when possible.
	// Query dolt_cluster_status from each healthy standby to find the one
	// with the most recent replication data, preventing promotion of empty
	// or stale pods that could cause data loss.
	toIndex, err := r.selectFailoverCandidate(ctx, doltdb, logger)
	if err != nil {
		logger.Info("Unable to select failover candidate via replication state, falling back to pod readiness",
			"error", err)
		toIndex, err = health.HealthyDoltDBReplica(ctx, r, doltdb)
		if err != nil {
			return fmt.Errorf("error getting healthy Dolt replica: %v", err)
		}
	}

	var errBundle *multierror.Error
	err = r.patch(ctx, doltdb, func(doltdb *doltv1alpha.DoltDB) {
		doltdb.Replication().Primary.PodIndex = toIndex
	})
	errBundle = multierror.Append(errBundle, err)

	err = r.patchStatus(ctx, doltdb, func(status *doltv1alpha.DoltDBStatus) {
		conditions.SetPrimarySwitching(status, doltdb)
	})
	errBundle = multierror.Append(errBundle, err)

	if err := errBundle.ErrorOrNil(); err != nil {
		return fmt.Errorf("error patching DoltDB: %v", err)
	}

	logger.Info("Switching primary", "from-index", *fromIndex, "to-index", *toIndex)
	r.recorder.Eventf(doltdb, corev1.EventTypeNormal, doltv1alpha.ReasonPrimarySwitching,
		"Switching primary from index '%d' to index '%d'", *fromIndex, *toIndex)

	return nil
}

// selectFailoverCandidate picks the best standby for promotion by checking
// replication state via dolt_cluster_status. It prefers the standby with the
// most recently updated data and refuses to select pods with no user databases.
func (r *PodReadinessController) selectFailoverCandidate(
	ctx context.Context,
	doltdb *doltv1alpha.DoltDB,
	logger logr.Logger,
) (*int, error) {
	healthyStandbys, err := health.HealthyDoltDBStandbys(ctx, r, doltdb)
	if err != nil {
		return nil, fmt.Errorf("error listing healthy standbys: %v", err)
	}
	if len(healthyStandbys) == 0 {
		return nil, fmt.Errorf("no healthy standbys available")
	}

	clientSet := NewReplicationClientSet(doltdb, r.refResolver)
	defer func() {
		if err := clientSet.Close(); err != nil {
			logger.V(1).Error(err, "error closing client set")
		}
	}()

	var bestIndex *int
	var bestLastUpdate time.Time

	for _, standbyPod := range healthyStandbys {
		podIndex, err := statefulset.PodIndex(standbyPod.Name)
		if err != nil {
			logger.V(1).Info("Error getting pod index, skipping", "pod", standbyPod.Name, "error", err)
			continue
		}

		client, err := clientSet.ClientForIndex(ctx, *podIndex)
		if err != nil {
			logger.V(1).Info("Unable to connect to standby, skipping", "pod", standbyPod.Name, "error", err)
			continue
		}

		// Check replication freshness via the standby's own cluster status.
		// A standby that has been actively replicating will have last_update
		// timestamps. An empty/new pod that never replicated will have none.
		dbState, err := client.GetDBState(ctx)
		if err != nil {
			logger.V(1).Info("Unable to get DB state, skipping", "pod", standbyPod.Name, "error", err)
			continue
		}

		// Find the oldest LastUpdate across all databases on this standby.
		// A standby is only as fresh as its least-replicated database.
		var oldestUpdate time.Time
		for _, status := range dbState.Status {
			if status.LastUpdate.Valid {
				if oldestUpdate.IsZero() || status.LastUpdate.Time.Before(oldestUpdate) {
					oldestUpdate = status.LastUpdate.Time
				}
			}
		}
		if oldestUpdate.IsZero() {
			logger.Info("Standby has no replication timestamps, skipping as failover candidate", "pod", standbyPod.Name)
			continue
		}

		// Prefer the standby with the most recent oldest-update (best worst-case freshness)
		if bestIndex == nil || oldestUpdate.After(bestLastUpdate) {
			bestIndex = podIndex
			bestLastUpdate = oldestUpdate
		}
	}

	if bestIndex == nil {
		return nil, fmt.Errorf("no standby with replicated data found for failover")
	}

	logger.Info("Selected failover candidate based on replication state",
		"pod-index", *bestIndex, "last-update", bestLastUpdate)
	return bestIndex, nil
}

// patch applies a patch to the DoltDB
func (r *PodReadinessController) patch(ctx context.Context, doltdb *doltv1alpha.DoltDB,
	patcher func(*doltv1alpha.DoltDB)) error {
	patch := client.MergeFrom(doltdb.DeepCopy())
	patcher(doltdb)

	if err := r.Patch(ctx, doltdb, patch); err != nil {
		return fmt.Errorf("error patching DoltDB: %v", err)
	}
	return nil
}

// patchStatus applies a status patch to the DoltDB
func (r *PodReadinessController) patchStatus(ctx context.Context, doltdb *doltv1alpha.DoltDB,
	patcher func(*doltv1alpha.DoltDBStatus)) error {
	patch := client.MergeFrom(doltdb.DeepCopy())
	patcher(&doltdb.Status)

	if err := r.Client.Status().Patch(ctx, doltdb, patch); err != nil {
		return fmt.Errorf("error patching DoltDB status: %v", err)
	}
	return nil
}
