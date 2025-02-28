package replication

import (
	"context"
	"errors"
	"fmt"

	"github.com/go-logr/logr"
	doltv1alpha "github.com/electronicarts/doltdb-operator/api/v1alpha"
	"github.com/electronicarts/doltdb-operator/pkg/conditions"
	"github.com/electronicarts/doltdb-operator/pkg/dolt"
	"github.com/electronicarts/doltdb-operator/pkg/dolt/sql"
	"github.com/electronicarts/doltdb-operator/pkg/health"
	"github.com/electronicarts/doltdb-operator/pkg/metrics"
	"github.com/electronicarts/doltdb-operator/pkg/statefulset"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// Removes routing of traffic to the current primary.
// Makes the current primary assume role standby.
// Makes a different chosen standby server assume role primary.
// Starts routing traffic to the new primary.

type doltClusterContext struct {
	states         []dolt.DBState
	currentPrimary int
	nextEpoch      int
}

type gracefulFailoverPhase struct {
	name      string
	reconcile func(context.Context, *doltv1alpha.DoltDB, *ReplicationClientSet, *doltClusterContext, logr.Logger) error
}

func shouldReconcileSwitchover(logger logr.Logger, doltdb *doltv1alpha.DoltDB) bool {
	if doltdb.IsResizingStorage() {
		logger.V(0).Info("skipping switchover, DoltDB is resizing storage")
		return false
	}
	if doltdb.Status.CurrentPrimaryPodIndex == nil {
		logger.V(0).Info("skipping switchover, 'status.currentPrimaryPodIndex' must be set")
		return false
	}
	currentPodIndex := ptr.Deref(doltdb.Status.CurrentPrimaryPodIndex, 0)
	desiredPodIndex := ptr.Deref(doltdb.Replication().Primary.PodIndex, 0)
	logger.V(0).Info("checking if switchover is needed", "currentPodIndex", currentPodIndex, "desiredPodIndex", desiredPodIndex)
	return currentPodIndex != desiredPodIndex
}

func (r *ReplicationReconciler) reconcileSwitchover(ctx context.Context, req *reconcileRequest, switchoverLogger logr.Logger) error {
	fromIndex := req.doltdb.Status.CurrentPrimaryPodIndex
	toIndex := req.doltdb.Replication().Primary.PodIndex
	logger := switchoverLogger.WithValues("doltdb", client.ObjectKeyFromObject(req.doltdb), "from-index", fromIndex, "to-index", toIndex)

	dbstates := GetDBStates(ctx, req.doltdb, req.clientSet)

	if !shouldReconcileSwitchover(logger, req.doltdb) {
		return nil
	}

	currentPrimary, highestEpoch, err := dolt.CurrentPrimaryAndEpoch(req.doltdb, dbstates)
	if err != nil {
		currentPrimary = ptr.Deref(req.doltdb.Status.CurrentPrimaryPodIndex, 0)
		highestEpoch = ptr.Deref(req.doltdb.Status.ReplicationEpoch, 0)
		logger.V(2).Info("failed to get current primary and epoch, falling back", "err", err)
	}

	if err := r.patchStatus(ctx, req.doltdb, func(status *doltv1alpha.DoltDBStatus) {
		conditions.SetPrimarySwitching(&req.doltdb.Status, req.doltdb)
	}); err != nil {
		return fmt.Errorf("error patching DoltDB status: %v", err)
	}

	phases := []gracefulFailoverPhase{
		{
			name:      "Shift traffic away from primary",
			reconcile: r.shiftTrafficAwayFromPrimary,
		},
		{
			name:      "Set primary to readonly (standby)",
			reconcile: r.setPrimaryReadOnly,
		},
		{
			name:      "Configure new primary",
			reconcile: r.configureNewPrimary,
		},
		{
			name:      "Shift traffic to new primary",
			reconcile: r.shiftTrafficToNewPrimary,
		},
	}

	doltDBCtx := &doltClusterContext{
		states:         dbstates,
		currentPrimary: currentPrimary,
		nextEpoch:      highestEpoch + 1,
	}

	for _, p := range phases {
		if err := p.reconcile(ctx, req.doltdb, req.clientSet, doltDBCtx, logger); err != nil {
			if apierrors.IsNotFound(err) {
				return err
			}
			return fmt.Errorf("error in '%s' switchover reconcile phase: %v", p.name, err)
		}
	}

	if err := r.patchStatus(ctx, req.doltdb, func(status *doltv1alpha.DoltDBStatus) {
		status.UpdateCurrentPrimary(req.doltdb, *toIndex)
		status.UpdateReplicationEpoch(doltDBCtx.nextEpoch)
		conditions.SetPrimarySwitched(&req.doltdb.Status)
		metrics.DoltDBCurrentPrimaryIndex.WithLabelValues(req.doltdb.Name, req.doltdb.Namespace).Set(float64(*toIndex))
		metrics.DoltDBReplicationSwitchOvers.WithLabelValues(req.doltdb.Name, req.doltdb.Namespace).Inc()
	}); err != nil {
		return fmt.Errorf("error patching DoltDB status: %v", err)
	}

	logger.Info("Primary switched")

	r.recorder.Eventf(req.doltdb, corev1.EventTypeNormal, doltv1alpha.ReasonPrimarySwitched,
		"Primary switched from index '%d' to index '%d'", *fromIndex, *toIndex)
	return nil
}

func (r *ReplicationReconciler) shiftTrafficAwayFromPrimary(
	ctx context.Context,
	doltdb *doltv1alpha.DoltDB,
	clientSet *ReplicationClientSet,
	doltCtx *doltClusterContext,
	logger logr.Logger,
) error {

	logger.Info("Shifting traffic away from primary", "pod", statefulset.PodName(doltdb.ObjectMeta, doltCtx.currentPrimary))

	r.recorder.Event(
		doltdb,
		corev1.EventTypeNormal,
		doltv1alpha.ReasonReplicationPrimaryReadonly,
		"Shifting traffic away from primary",
	)

	for i := 0; i < int(doltdb.Spec.Replicas); i++ {
		doltPod, err := r.refResolver.DoltDBPodRef(ctx, doltdb, i)
		if err != nil {
			logger.V(2).Info("error getting pod reference, skipping", "replica", i, "error", err)
			continue
		}
		if err := dolt.MarkRoleStandby(ctx, doltPod, r.Client); err != nil {
			return fmt.Errorf("error marking pod '%s' role as standby to shift traffic away from primary: %v", doltPod.Name, err)
		}
	}

	return nil
}

func (r *ReplicationReconciler) setPrimaryReadOnly(
	ctx context.Context,
	doltdb *doltv1alpha.DoltDB,
	clientSet *ReplicationClientSet,
	doltCtx *doltClusterContext,
	logger logr.Logger,
) error {
	_, ready, err := r.currentPrimaryReady(ctx, doltdb)
	if err != nil {
		return fmt.Errorf("error getting current primary readiness: %v", err)
	}
	if !ready {
		return nil
	}

	client, err := clientSet.currentPrimaryClient(ctx)
	if err != nil {
		return fmt.Errorf("error getting client for primary: %v", err)
	}
	defer clientSet.RemoveClientFromCache(*doltdb.Status.CurrentPrimaryPodIndex)

	logger.Info("Enabling readonly mode in primary")
	r.recorder.Event(
		doltdb,
		corev1.EventTypeNormal,
		doltv1alpha.ReasonReplicationPrimaryReadonly,
		"Enabling readonly mode in primary",
	)

	assumeRoleOpts := sql.AssumeRoleOpts{
		Epoch: doltCtx.nextEpoch,
		Role:  dolt.StandbyRoleValue,
	}
	if err := client.AssumeRole(ctx, assumeRoleOpts); err != nil {
		return fmt.Errorf("error setting primary as readonly: %v", err)
	}

	return nil
}

func (r *ReplicationReconciler) configureNewPrimary(
	ctx context.Context,
	doltdb *doltv1alpha.DoltDB,
	clientSet *ReplicationClientSet,
	doltCtx *doltClusterContext,
	logger logr.Logger,
) error {
	newPrimaryIndex := doltdb.Replication().Primary.PodIndex
	if newPrimaryIndex == nil {
		return errors.New("new primary 'spec.replication.primary.podIndex' must be set")
	}

	client, err := clientSet.ClientForIndex(ctx, *newPrimaryIndex)
	if err != nil {
		return fmt.Errorf("error getting new primary SQL client: %v", err)
	}
	defer clientSet.RemoveClientFromCache(*newPrimaryIndex)

	logger.Info("Configuring new primary", "pod-index", newPrimaryIndex)

	r.recorder.Eventf(
		doltdb,
		corev1.EventTypeNormal,
		doltv1alpha.ReasonReplicationPrimaryNew,
		"Configuring new primary at index '%d'",
		*newPrimaryIndex,
	)

	assumeRoleOpts := sql.AssumeRoleOpts{
		Epoch: doltCtx.nextEpoch,
		Role:  dolt.PrimaryRoleValue,
	}
	if err := client.AssumeRole(ctx, assumeRoleOpts); err != nil {
		return fmt.Errorf("error configuring new primary: %v", err)
	}

	return nil
}

func (r *ReplicationReconciler) shiftTrafficToNewPrimary(
	ctx context.Context,
	doltdb *doltv1alpha.DoltDB,
	clientSet *ReplicationClientSet,
	doltCtx *doltClusterContext,
	logger logr.Logger,
) error {
	newPrimaryPod, ready, err := health.IsDoltDBReplicaHealthy(ctx, r, doltdb, doltCtx.currentPrimary)
	if err != nil {
		return fmt.Errorf("error getting current primary readiness: %v", err)
	}
	if !ready {
		return fmt.Errorf("new primary '%s' is not ready", newPrimaryPod.Name)
	}

	logger.Info("Shifting traffic to primary")
	r.recorder.Event(
		doltdb,
		corev1.EventTypeNormal,
		doltv1alpha.ReasonReplicationPrimaryReadonly,
		"Shifting traffic to primary",
	)

	if err := dolt.MarkRolePrimary(ctx, newPrimaryPod, r); err != nil {
		return fmt.Errorf("error patching primary pod '%s' to shift traffic to it: %v", newPrimaryPod.Name, err)
	}

	return nil
}

func (r *ReplicationReconciler) currentPrimaryReady(ctx context.Context, doltdb *doltv1alpha.DoltDB) (*corev1.Pod, bool, error) {
	if doltdb.Status.CurrentPrimaryPodIndex == nil {
		return nil, false, errors.New("'status.currentPrimaryPodIndex' must be set")
	}
	return health.IsDoltDBReplicaHealthy(ctx, r, doltdb, *doltdb.Status.CurrentPrimaryPodIndex)
}
