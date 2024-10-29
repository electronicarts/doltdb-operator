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
	doltpod "github.com/electronicarts/doltdb-operator/pkg/pod"
	"github.com/electronicarts/doltdb-operator/pkg/statefulset"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// Removes routing of traffic to the current primary.
// Makes the current primary assume role standby.
// Makes a different chosen standby server assume role primary.
// Starts routing traffic to the new primary.

type gracefulFailoverPhase struct {
	name      string
	reconcile func(context.Context, *doltv1alpha.DoltCluster, *ReplicationClientSet, logr.Logger) error
}

func shouldReconcileSwitchover(doltdb *doltv1alpha.DoltCluster) bool {
	if doltdb.IsUpdating() || doltdb.IsResizingStorage() {
		return false
	}
	if doltdb.Status.CurrentPrimaryPodIndex == nil {
		return false
	}
	currentPodIndex := ptr.Deref(doltdb.Status.CurrentPrimaryPodIndex, 0)
	desiredPodIndex := ptr.Deref(doltdb.Replication().Primary.PodIndex, 0)
	return currentPodIndex != desiredPodIndex
}

func (r *ReplicationReconciler) reconcileSwitchover(ctx context.Context, req *reconcileRequest, switchoverLogger logr.Logger) error {
	if !shouldReconcileSwitchover(req.doltdb) {
		return nil
	}

	fromIndex := req.doltdb.Status.CurrentPrimaryPodIndex
	toIndex := req.doltdb.Replication().Primary.PodIndex
	logger := switchoverLogger.WithValues("doltdb", client.ObjectKeyFromObject(req.doltdb), "from-index", fromIndex, "to-index", toIndex)

	if err := r.patchStatus(ctx, req.doltdb, func(status *doltv1alpha.DoltClusterStatus) {
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

	for _, p := range phases {
		if err := p.reconcile(ctx, req.doltdb, req.clientSet, logger); err != nil {
			if apierrors.IsNotFound(err) {
				return err
			}
			return fmt.Errorf("error in '%s' switchover reconcile phase: %v", p.name, err)
		}
	}

	if err := r.patchStatus(ctx, req.doltdb, func(status *doltv1alpha.DoltClusterStatus) {
		status.UpdateCurrentPrimary(req.doltdb, *toIndex)
		conditions.SetPrimarySwitched(&req.doltdb.Status)
	}); err != nil {
		return fmt.Errorf("error patching DoltDB status: %v", err)
	}

	logger.Info("Primary switched")

	r.recorder.Eventf(req.doltdb, corev1.EventTypeNormal, doltv1alpha.ReasonPrimarySwitched,
		"Primary switched from index '%d' to index '%d'", *fromIndex, toIndex)
	return nil
}

func (r *ReplicationReconciler) shiftTrafficAwayFromPrimary(ctx context.Context, doltdb *doltv1alpha.DoltCluster,
	clientSet *ReplicationClientSet, logger logr.Logger) error {
	existingPod, ready, err := r.currentPrimaryReady(ctx, doltdb)
	if err != nil {
		return fmt.Errorf("error getting current primary readiness: %v", err)
	}

	logger.WithValues(map[string]interface{}{
		"primaryReady": ready,
	}).Info("Shifting traffic away from primary")

	r.recorder.Event(doltdb, corev1.EventTypeNormal, doltv1alpha.ReasonReplicationPrimaryReadonly,
		"Shifting traffic away from primary")

	patch := client.MergeFrom(existingPod.DeepCopy())
	existingPod.ObjectMeta.Labels[dolt.RoleLabel] = dolt.StandbyRoleValue.String()

	if err := r.Patch(ctx, existingPod, patch); err != nil {
		return fmt.Errorf("error patching primary pod '%s' to remove traffic: %v", existingPod.Name, err)
	}

	return nil
}

func (r *ReplicationReconciler) setPrimaryReadOnly(ctx context.Context, doltdb *doltv1alpha.DoltCluster,
	clientSet *ReplicationClientSet, logger logr.Logger) error {
	_, ready, err := r.currentPrimaryReady(ctx, doltdb)
	if err != nil {
		return fmt.Errorf("error getting current primary readiness: %v", err)
	}
	if !ready {
		return nil
	}

	nextReplicationEpoch, err := r.nextReplicationEpoch(ctx, doltdb, clientSet)
	if err != nil {
		return fmt.Errorf("error getting next replication epoch: %v", err)
	}

	client, err := clientSet.currentPrimaryClient(ctx)
	if err != nil {
		return fmt.Errorf("error getting client for primary: %v", err)
	}

	logger.Info("Enabling readonly mode in primary")
	r.recorder.Event(doltdb, corev1.EventTypeNormal, doltv1alpha.ReasonReplicationPrimaryReadonly,
		"Enabling readonly mode in primary")

	assumeRoleOpts := sql.AssumeRoleOpts{
		Epoch: nextReplicationEpoch,
		Role:  dolt.StandbyRoleValue,
	}
	if err := client.AssumeRole(ctx, assumeRoleOpts); err != nil {
		return fmt.Errorf("error setting primary as readonly: %v", err)
	}

	return r.patchReplicationEpochStatus(ctx, doltdb, nextReplicationEpoch)
}

func (r *ReplicationReconciler) configureNewPrimary(ctx context.Context, doltdb *doltv1alpha.DoltCluster,
	clientSet *ReplicationClientSet, logger logr.Logger) error {
	newPrimaryIndex, err := health.HealthyDoltDBReplica(ctx, r, doltdb)
	if err != nil {
		return fmt.Errorf("error fetching a healthy DoltDB replica: %v", err)
	}

	client, err := clientSet.ClientForIndex(ctx, *newPrimaryIndex)
	if err != nil {
		return fmt.Errorf("error getting new primary client: %v", err)
	}

	nextReplicationEpoch, err := r.nextReplicationEpoch(ctx, doltdb, clientSet)
	if err != nil {
		return fmt.Errorf("error getting next replication epoch: %v", err)
	}

	logger.Info("Configuring new primary", "pod-index", newPrimaryIndex)
	r.recorder.Eventf(doltdb, corev1.EventTypeNormal, doltv1alpha.ReasonReplicationPrimaryNew,
		"Configuring new primary at index '%d'", newPrimaryIndex)

	assumeRoleOpts := sql.AssumeRoleOpts{
		Epoch: nextReplicationEpoch,
		Role:  dolt.PrimaryRoleValue,
	}
	if err := client.AssumeRole(ctx, assumeRoleOpts); err != nil {
		return fmt.Errorf("error configuring new primary: %v", err)
	}

	return r.patchReplicationEpochStatus(ctx, doltdb, nextReplicationEpoch)
}

func (r *ReplicationReconciler) shiftTrafficToNewPrimary(ctx context.Context, doltdb *doltv1alpha.DoltCluster,
	clientSet *ReplicationClientSet, logger logr.Logger) error {
	existingPod, ready, err := r.currentPrimaryReady(ctx, doltdb)
	if err != nil {
		return fmt.Errorf("error getting current primary readiness: %v", err)
	}
	if !ready {
		return nil
	}

	logger.Info("Shifting traffic to primary")
	r.recorder.Event(doltdb, corev1.EventTypeNormal, doltv1alpha.ReasonReplicationPrimaryReadonly,
		"Shifting traffic to primary")

	patch := client.MergeFrom(existingPod.DeepCopy())
	existingPod.ObjectMeta.Labels[dolt.RoleLabel] = dolt.PrimaryRoleValue.String()

	if err := r.Patch(ctx, existingPod, patch); err != nil {
		return fmt.Errorf("error patching primary pod '%s' to shift traffic to it: %v", existingPod.Name, err)
	}

	return nil
}

func (r *ReplicationReconciler) findHighestEpoch(ctx context.Context, doltdb *doltv1alpha.DoltCluster, clientSet *ReplicationClientSet) (int, error) {
	pods, ready, err := r.replicasReady(ctx, doltdb)
	if err != nil {
		return -1, fmt.Errorf("error waiting all replicas to be in ready state: %v", err)
	}
	if !ready {
		return -1, fmt.Errorf("waiting for replicas to be ready")
	}

	var highestEpoch int
	clients := make([]*sql.Client, len(pods))

	for i, pod := range pods {
		key := types.NamespacedName{
			Name:      pod.Name,
			Namespace: doltdb.Namespace,
		}

		clients[i], err = clientSet.clientForIndex(ctx, i)
		if err != nil {
			return -1, fmt.Errorf("error creating client for replica %s: %v", key, err)
		}

		_, epoch, err := clients[i].GetRoleAndEpoch(ctx)
		if err != nil {
			return -1, fmt.Errorf("error fetching role and epoch for %s: %v", key, err)
		}

		if epoch > highestEpoch {
			highestEpoch = epoch
		}
	}

	return highestEpoch, nil
}

func (r *ReplicationReconciler) nextReplicationEpoch(ctx context.Context, doltdb *doltv1alpha.DoltCluster, clientSet *ReplicationClientSet) (int, error) {
	if doltdb.Status.ReplicationEpoch != nil {
		return (*doltdb.Status.ReplicationEpoch) + 1, nil
	}

	highestEpoch, err := r.findHighestEpoch(ctx, doltdb, clientSet)
	if err != nil {
		return -1, fmt.Errorf("error finding highest replication epoch to set primary to readonly")
	}

	return highestEpoch + 1, nil
}

// func pickNextPrimary(dbstates []DBState) int {
// 	firststandby := -1
// 	nextprimary := -1
// 	var updated time.Time
// 	for i, state := range dbstates {
// 		if state.Role == "standby" {
// 			if firststandby == -1 {
// 				firststandby = i
// 			}

// 			var oldestDB time.Time
// 			for _, status := range state.Status {
// 				if status.LastUpdate.Valid && (oldestDB == (time.Time{}) || oldestDB.After(status.LastUpdate.Time)) {
// 					oldestDB = status.LastUpdate.Time
// 				}
// 			}

// 			if oldestDB != (time.Time{}) && (updated == (time.Time{}) || updated.Before(oldestDB)) {
// 				nextprimary = i
// 				updated = oldestDB
// 			}
// 		}
// 	}
// 	if nextprimary != -1 {
// 		return nextprimary
// 	}
// 	return firststandby
// }

func (r *ReplicationReconciler) currentPrimaryReady(ctx context.Context, doltdb *doltv1alpha.DoltCluster) (*corev1.Pod, bool, error) {
	if doltdb.Status.CurrentPrimaryPodIndex == nil {
		return nil, false, errors.New("'status.currentPrimaryPodIndex' must be set")
	}
	podName := statefulset.PodName(doltdb.ObjectMeta, *doltdb.Status.CurrentPrimaryPodIndex)
	key := types.NamespacedName{
		Name:      podName,
		Namespace: doltdb.Namespace,
	}
	var pod corev1.Pod
	if err := r.Get(ctx, key, &pod); err != nil {
		return nil, false, err
	}
	return &pod, doltpod.PodReady(&pod), nil
}

func (r *ReplicationReconciler) replicasReady(ctx context.Context, doltdb *doltv1alpha.DoltCluster) ([]*corev1.Pod, bool, error) {
	if doltdb.Status.CurrentPrimaryPodIndex == nil {
		return nil, false, errors.New("'status.currentPrimaryPodIndex' must be set")
	}

	pods := make([]*corev1.Pod, doltdb.Spec.Replicas)

	for i := 0; i < int(doltdb.Spec.Replicas); i++ {
		key := types.NamespacedName{
			Name:      statefulset.PodName(doltdb.ObjectMeta, i),
			Namespace: doltdb.Namespace,
		}
		var pod corev1.Pod
		if err := r.Get(ctx, key, &pod); err != nil {
			return nil, false, err
		}

		if !doltpod.PodReady(&pod) {
			return nil, false, nil
		}

		pods[i] = &pod
	}

	return pods, true, nil
}

// func (r *ReplicationReconciler) currentPrimaryAndEpoch(ctx context.Context, doltdb *doltv1alpha.DoltCluster, clientSet *ReplicationClientSet) (int, *corev1.Pod, error) {
// 	pods, ready, err := r.replicasReady(ctx, doltdb)
// 	if err != nil {
// 		return -1, nil, fmt.Errorf("error waiting all replicas to be in ready state: %v", err)
// 	}
// 	if !ready {
// 		return -1, nil, fmt.Errorf("waiting for replicas readiness")
// 	}

// 	var highestEpoch int
// 	var currentPrimary *corev1.Pod

// 	clients := make([]*sql.Client, len(pods))

// 	for i, pod := range pods {
// 		key := types.NamespacedName{
// 			Name:      pod.Name,
// 			Namespace: doltdb.Namespace,
// 		}

// 		clients[i], err = clientSet.clientForIndex(ctx, i)
// 		if err != nil {
// 			return -1, nil, fmt.Errorf("error creating client for replica %s: %v", key, err)
// 		}

// 		role, epoch, err := clients[i].GetRoleAndEpoch(ctx)
// 		if err != nil {
// 			return -1, nil, fmt.Errorf("error fetching role and epoch for %s: %v", key, err)
// 		}

// 		if epoch > highestEpoch {
// 			highestEpoch = epoch
// 		}

// 		if role == dolt.PrimaryRoleValue.String() {
// 			if currentPrimary != nil {
// 				return -1, nil, fmt.Errorf("found more than one primary, %s and %s", currentPrimary.Name, pod.Name)
// 			}

// 			currentPrimary = pod
// 		}
// 	}

// 	return highestEpoch, currentPrimary, nil
// }
