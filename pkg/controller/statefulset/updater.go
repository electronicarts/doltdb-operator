// Copyright (c) 2025 Electronic Arts Inc. All rights reserved.

package statefulset

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"time"

	doltv1alpha "github.com/electronicarts/doltdb-operator/api/v1alpha"
	"github.com/electronicarts/doltdb-operator/pkg/builder"
	"github.com/electronicarts/doltdb-operator/pkg/dolt"
	doltsql "github.com/electronicarts/doltdb-operator/pkg/dolt/sql"
	"github.com/electronicarts/doltdb-operator/pkg/health"
	podpkg "github.com/electronicarts/doltdb-operator/pkg/pod"
	"github.com/electronicarts/doltdb-operator/pkg/wait"
	"github.com/go-logr/logr"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	klabels "k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

const (
	// maxReplicationLagMillis is the maximum acceptable replication lag (in ms) before
	// allowing primary pod deletion during rolling updates.
	maxReplicationLagMillis = 5000
)

func shouldReconcileUpdates(doltdb *doltv1alpha.DoltDB) bool {
	// Skip updates if DoltDB is in a transitional state
	if doltdb.IsResizingStorage() || doltdb.IsSwitchingPrimary() {
		return false
	}
	// Only automatically reconcile updates for ReplicasFirstPrimaryLast strategy
	// - RollingUpdate: Kubernetes handles updates natively (no operator intervention)
	// - OnDelete: User must manually delete pods to trigger updates
	// - Never: No automatic updates at all
	return doltdb.Spec.UpdateStrategy == "" ||
		doltdb.Spec.UpdateStrategy == doltv1alpha.ReplicasFirstPrimaryLastUpdateType
}

func (r *Reconciler) reconcileUpdates(ctx context.Context, doltdb *doltv1alpha.DoltDB) (ctrl.Result, error) {
	if !shouldReconcileUpdates(doltdb) {
		return ctrl.Result{}, nil
	}
	doltdbKey := client.ObjectKeyFromObject(doltdb)
	logger := log.FromContext(ctx).WithName("update")

	stsUpdateRevision, err := GetRevision(ctx, r.Client, doltdb)
	if err != nil {
		return ctrl.Result{}, err
	}
	if stsUpdateRevision == "" {
		logger.V(1).Info("StatefulSet status.updateRevision not set. Requeuing...")
		return ctrl.Result{RequeueAfter: 1 * time.Second}, nil
	}

	var podsByRole podRoleSet
	if result, err := r.getPodsByRole(ctx, doltdb, &podsByRole, logger); !result.IsZero() || err != nil {
		return result, err
	}

	stalePodNames := podsByRole.getStalePodNames(stsUpdateRevision)
	if len(stalePodNames) == 0 {
		return ctrl.Result{}, nil
	}
	logger.V(1).Info("Detected stale Pods that need updating", "pods", stalePodNames)

	if result, err := r.waitForReadyStatus(ctx, doltdb, logger); !result.IsZero() || err != nil {
		return result, err
	}

	for _, replicaPod := range podsByRole.replicas {
		if podpkg.PodUpdated(&replicaPod, stsUpdateRevision) {
			logger.V(1).Info("Replica Pod up to date", "pod", replicaPod.Name)
			continue
		}
		logger.Info("Updating replica Pod", "pod", replicaPod.Name)
		if err := r.updatePod(ctx, doltdbKey, &replicaPod, stsUpdateRevision, logger); err != nil {
			return ctrl.Result{}, fmt.Errorf("error updating replica Pod '%s': %v", replicaPod.Name, err)
		}

		return ctrl.Result{Requeue: true}, nil
	}

	if result, err := r.waitForReplicationCaughtUp(ctx, doltdb, logger); !result.IsZero() || err != nil {
		return result, err
	}

	primaryPod := podsByRole.primary
	if podpkg.PodUpdated(&primaryPod, stsUpdateRevision) {
		logger.V(1).Info("Primary Pod up to date", "pod", primaryPod.Name)
		return ctrl.Result{}, nil
	}

	logger.Info("Updating primary Pod", "pod", primaryPod.Name)
	if err := r.gracefulPrimaryUpdate(ctx, doltdb, doltdbKey, &primaryPod, stsUpdateRevision, logger); err != nil {
		return ctrl.Result{}, fmt.Errorf("error updating primary Pod '%s': %v", primaryPod.Name, err)
	}
	return ctrl.Result{}, nil
}

func (r *Reconciler) waitForReadyStatus(ctx context.Context, doltdb *doltv1alpha.DoltDB, logger logr.Logger) (ctrl.Result, error) {
	var sts appsv1.StatefulSet
	if err := r.Get(ctx, client.ObjectKeyFromObject(doltdb), &sts); err != nil {
		return ctrl.Result{}, err
	}
	if sts.Status.ReadyReplicas != doltdb.Spec.Replicas {
		logger.V(1).Info("Waiting for all Pods to be ready to proceed with the update. Requeuing...")
		return ctrl.Result{RequeueAfter: 1 * time.Second}, nil
	}

	return ctrl.Result{}, nil
}

// waitForReplicationCaughtUp checks that replication is configured and that standbys
// have caught up with the primary before allowing primary pod deletion.
func (r *Reconciler) waitForReplicationCaughtUp(
	ctx context.Context,
	doltdb *doltv1alpha.DoltDB,
	logger logr.Logger,
) (ctrl.Result, error) {
	if !doltdb.Replication().Enabled {
		return ctrl.Result{}, nil
	}
	if !doltdb.IsReplicationConfigured() {
		logger.V(1).Info("Waiting for Pods to have configured replication.")
		return ctrl.Result{}, ErrSkipReconciliationPhase
	}

	if doltdb.Status.CurrentPrimaryPodIndex == nil {
		logger.V(1).Info("Waiting for primary pod index to be set.")
		return ctrl.Result{RequeueAfter: 1 * time.Second}, nil
	}

	// Verify at least one standby pod is Ready before proceeding.
	// A just-restarted standby may not appear in dolt_cluster_status yet,
	// so we check pod readiness first to avoid a false "all caught up".
	healthyStandbys, err := health.HealthyDoltDBStandbys(ctx, r, doltdb)
	if err != nil {
		logger.V(1).Info("Unable to list healthy standbys, requeuing", "error", err)
		return ctrl.Result{RequeueAfter: 2 * time.Second}, nil
	}
	if len(healthyStandbys) == 0 {
		logger.V(0).Info("No healthy standby pods found, waiting before updating primary")
		return ctrl.Result{RequeueAfter: 2 * time.Second}, nil
	}

	primaryClient, err := doltsql.NewInternalClientWithPodIndex(
		ctx, doltdb, r.refResolver, *doltdb.Status.CurrentPrimaryPodIndex,
	)
	if err != nil {
		logger.V(1).Info("Unable to connect to primary to check replication lag, proceeding", "error", err)
		return ctrl.Result{}, nil
	}
	defer func() { _ = primaryClient.Close() }()

	statuses, err := primaryClient.GetClusterStatus(ctx)
	if err != nil {
		logger.V(1).Info("Unable to query cluster status, proceeding", "error", err)
		return ctrl.Result{}, nil
	}

	// If the primary reports no standby statuses, the standby hasn't connected yet.
	if len(statuses) == 0 {
		logger.V(0).Info("No replication statuses reported by primary, standby may still be connecting")
		return ctrl.Result{RequeueAfter: 2 * time.Second}, nil
	}

	for _, status := range statuses {
		if status.ReplicationLag.Valid && status.ReplicationLag.Int64 > maxReplicationLagMillis {
			logger.V(0).Info(
				"Replication lag too high, waiting before updating primary",
				"remote", status.Remote,
				"lag_ms", status.ReplicationLag.Int64,
				"max_ms", maxReplicationLagMillis,
			)
			return ctrl.Result{RequeueAfter: 2 * time.Second}, nil
		}
		if status.CurrentError.Valid && status.CurrentError.String != "" {
			logger.V(0).Info(
				"Replication has errors, waiting before updating primary",
				"remote", status.Remote,
				"error", status.CurrentError.String,
			)
			return ctrl.Result{RequeueAfter: 2 * time.Second}, nil
		}
	}

	logger.V(1).Info("Replication is caught up, safe to update primary.")
	return ctrl.Result{}, nil
}

// gracefulPrimaryUpdate performs a graceful switchover before deleting the primary pod.
// It transitions the primary to standby (draining replication), promotes a standby,
// updates pod labels, and then deletes the (now standby) pod for update.
func (r *Reconciler) gracefulPrimaryUpdate(
	ctx context.Context,
	doltdb *doltv1alpha.DoltDB,
	doltdbKey types.NamespacedName,
	primaryPod *corev1.Pod,
	updateRevision string,
	logger logr.Logger,
) error {
	if !doltdb.Replication().Enabled || doltdb.Status.CurrentPrimaryPodIndex == nil {
		return r.updatePod(ctx, doltdbKey, primaryPod, updateRevision, logger)
	}

	primaryIndex := *doltdb.Status.CurrentPrimaryPodIndex
	primaryClient, err := doltsql.NewInternalClientWithPodIndex(ctx, doltdb, r.refResolver, primaryIndex)
	if err != nil {
		logger.Info("Unable to connect to primary for graceful switchover, falling back to direct update", "error", err)
		return r.updatePod(ctx, doltdbKey, primaryPod, updateRevision, logger)
	}
	defer func() { _ = primaryClient.Close() }()

	_, epoch, err := primaryClient.GetRoleAndEpoch(ctx)
	if err != nil {
		logger.Info("Unable to get role/epoch for graceful switchover, falling back to direct update", "error", err)
		return r.updatePod(ctx, doltdbKey, primaryPod, updateRevision, logger)
	}

	standbyHosts, err := health.StandbyHostFQDNs(ctx, r, doltdb)
	if err != nil {
		logger.Info("Unable to resolve standby hosts, falling back to direct update", "error", err)
		return r.updatePod(ctx, doltdbKey, primaryPod, updateRevision, logger)
	}

	nextEpoch := epoch + 1
	logger.Info("Gracefully transitioning primary to standby before update (draining replication)")

	// Use a longer timeout for TransitionToStandby since Dolt needs to verify
	// that standbys are caught up across all databases, which can take time
	// if the standby just restarted.
	transitionCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	transitionOpts := doltsql.TransitionStandbyOpts{
		Epoch:               nextEpoch,
		MinCaughtUpStandbys: 1,
		Hosts:               standbyHosts,
	}
	caughtUpIdx, err := primaryClient.TransitionToStandby(transitionCtx, transitionOpts)
	if err != nil {
		logger.Info(
			"Graceful transition failed, falling back to direct update",
			"error", err,
			"hint", "standby may not have finished catching up after restart",
		)
		return r.updatePod(ctx, doltdbKey, primaryPod, updateRevision, logger)
	}

	// Promote the caught-up standby (with data safety check)
	newPrimaryClient, err := doltsql.NewInternalClientWithPodIndex(ctx, doltdb, r.refResolver, caughtUpIdx)
	if err != nil {
		return fmt.Errorf("error connecting to new primary at index %d: %v", caughtUpIdx, err)
	}
	defer func() { _ = newPrimaryClient.Close() }()

	if err := newPrimaryClient.AssumeRole(ctx, doltsql.AssumeRoleOpts{
		Epoch: nextEpoch,
		Role:  dolt.PrimaryRoleValue,
	}); err != nil {
		return fmt.Errorf("error promoting standby at index %d to primary: %v", caughtUpIdx, err)
	}

	// Update pod labels: old primary → standby, new primary → primary
	if err := dolt.MarkRoleStandby(ctx, primaryPod, r.Client); err != nil {
		return fmt.Errorf("error marking old primary pod '%s' as standby: %v", primaryPod.Name, err)
	}
	newPrimaryPod, _, err := health.IsDoltDBReplicaHealthy(ctx, r, doltdb, caughtUpIdx)
	if err != nil {
		return fmt.Errorf("error getting new primary pod at index %d: %v", caughtUpIdx, err)
	}
	if err := dolt.MarkRolePrimary(ctx, newPrimaryPod, r); err != nil {
		return fmt.Errorf("error marking new primary pod '%s': %v", newPrimaryPod.Name, err)
	}

	logger.Info(
		"Switchover complete before update, deleting old primary (now standby)",
		"old-primary", primaryPod.Name,
		"new-primary", newPrimaryPod.Name,
		"epoch", nextEpoch,
	)

	return r.updatePod(ctx, doltdbKey, primaryPod, updateRevision, logger)
}

func (r *Reconciler) updatePod(ctx context.Context, doltdbKey types.NamespacedName, pod *corev1.Pod, updateRevision string,
	logger logr.Logger) error {
	if err := r.Delete(ctx, pod); err != nil {
		return fmt.Errorf("error deleting Pod '%s': %v", pod.Name, err)
	}

	updateCtx, cancel := context.WithTimeout(ctx, 1*time.Minute)
	defer cancel()
	if err := r.pollUntilPodUpdated(updateCtx, doltdbKey, client.ObjectKeyFromObject(pod), updateRevision, logger); err != nil {
		return fmt.Errorf("error waiting for Pod '%s' to be updated: %v", pod.Name, err)
	}
	return nil
}

func (r *Reconciler) pollUntilPodUpdated(ctx context.Context, doltdbKey, podKey types.NamespacedName, updateRevision string,
	logger logr.Logger) error {
	return wait.PollWithDoltDB(ctx, doltdbKey, r.Client, logger, func(ctx context.Context) error {
		var pod corev1.Pod
		if err := r.Get(ctx, podKey, &pod); err != nil {
			return fmt.Errorf("error getting Pod '%s': %v", podKey.Name, err)
		}
		if podpkg.PodUpdated(&pod, updateRevision) {
			return nil
		}
		return errors.New("pod is stale")
	})
}

type podRoleSet struct {
	replicas []corev1.Pod
	primary  corev1.Pod
}

func (p *podRoleSet) getStalePodNames(updateRevision string) []string {
	var podNames []string
	for _, r := range p.replicas {
		if !podpkg.PodUpdated(&r, updateRevision) {
			podNames = append(podNames, r.Name)
		}
	}
	if !podpkg.PodUpdated(&p.primary, updateRevision) {
		podNames = append(podNames, p.primary.Name)
	}
	return podNames
}

func (r *Reconciler) getPodsByRole(ctx context.Context, doltdb *doltv1alpha.DoltDB, podsByRole *podRoleSet,
	logger logr.Logger) (ctrl.Result, error) {
	currentPrimary := ptr.Deref(doltdb.Status.CurrentPrimary, "")
	if currentPrimary == "" {
		logger.V(1).Info("DoltDB status.currentPrimary not set. Requeuing...")
		return ctrl.Result{RequeueAfter: 1 * time.Second}, nil
	}
	if doltdb.Spec.Replicas == 0 {
		logger.V(1).Info("DoltDB is downscaled. Requeuing...")
		return ctrl.Result{RequeueAfter: 5 * time.Second}, nil
	}

	list := corev1.PodList{}
	listOpts := &client.ListOptions{
		LabelSelector: klabels.SelectorFromSet(
			builder.NewLabelsBuilder().
				WithDoltSelectorLabels(doltdb).
				Build(),
		),
		Namespace: doltdb.GetNamespace(),
	}
	if err := r.List(ctx, &list, listOpts); err != nil {
		return ctrl.Result{}, fmt.Errorf("error listing Pods: %v", err)
	}

	numPods := len(list.Items)
	numReplicas := int(doltdb.Spec.Replicas)
	if len(list.Items) != int(doltdb.Spec.Replicas) {
		logger.V(1).Info("Number of Pods does not match DoltDB replicas. Requeuing...", "pods", numPods, "doltdb-replicas", numReplicas)
		// TODO: revisit time
		return ctrl.Result{RequeueAfter: 1 * time.Second}, nil
	}

	var replicas []corev1.Pod
	var primary *corev1.Pod
	for _, pod := range list.Items {
		if pod.Name == currentPrimary {
			primary = &pod
		} else {
			replicas = append(replicas, pod)
		}
	}
	if len(replicas) == 0 && doltdb.Spec.Replicas > 1 {
		return ctrl.Result{}, errors.New("no replica Pods found")
	}
	if primary == nil {
		return ctrl.Result{}, errors.New("primary Pod not found")
	}
	sort.Slice(replicas, func(i, j int) bool {
		return replicas[i].Name > replicas[j].Name
	})

	if podsByRole == nil {
		podsByRole = &podRoleSet{}
	}
	podsByRole.replicas = replicas
	podsByRole.primary = *primary

	return ctrl.Result{}, nil
}
