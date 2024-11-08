package replication

import (
	"context"
	"fmt"

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

	//TODO: implement minCaughtUpStandbys logic https://github.com/dolthub/doltclusterctl/blob/main/commands.go#L144
	// minCaughtUpStandbys := ptr.Deref(doltdb.Replication().Primary.MinCaughtUpStandbys, -1)
	// if minCaughtUpStandbys != -1 {
	// 	toIndex, err := r.replConfig.GetNextPrimary(ctx, doltdb, nil, *doltdb.Status.ReplicationEpoch)
	// 	if err != nil {
	// 		return fmt.Errorf("error getting next primary: %v", err)
	// 	}
	// }
	// clientSet := NewReplicationClientSet(doltdb, r.refResolver)
	// defer clientSet.close()

	// dbstates := GetDBStates(ctx, doltdb, clientSet)

	// var toIndex *int
	// toIndex = ptr.To(dolt.PickNextPrimary(dbstates))

	// if toIndex == fromIndex || *toIndex < 0 {
	toIndex, err := health.HealthyDoltDBReplica(ctx, r, doltdb)
	if err != nil {
		return fmt.Errorf("error getting healthy Dolt replica: %v", err)
	}
	// }

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
