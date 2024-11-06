package controller

import (
	"context"
	"fmt"

	doltv1alpha "github.com/electronicarts/doltdb-operator/api/v1alpha"
	"github.com/electronicarts/doltdb-operator/pkg/builder"
	"github.com/electronicarts/doltdb-operator/pkg/conditions"
	"github.com/electronicarts/doltdb-operator/pkg/controller/replication"
	"github.com/electronicarts/doltdb-operator/pkg/dolt"
	"github.com/electronicarts/doltdb-operator/pkg/health"
	podpkg "github.com/electronicarts/doltdb-operator/pkg/pod"
	"github.com/electronicarts/doltdb-operator/pkg/statefulset"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	klabels "k8s.io/apimachinery/pkg/labels"
	"k8s.io/utils/ptr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

type patcherDoltDB func(*doltv1alpha.DoltClusterStatus) error

func (r *DoltDBReconciler) reconcileStatus(ctx context.Context, doltdb *doltv1alpha.DoltCluster) (ctrl.Result, error) {
	var sts appsv1.StatefulSet
	if err := r.Get(ctx, client.ObjectKeyFromObject(doltdb), &sts); err != nil {
		log.FromContext(ctx).V(1).Info("error getting StatefulSet", "err", err)
	}

	clientSet := replication.NewReplicationClientSet(doltdb, r.RefResolver)
	defer clientSet.Close()

	dbstates := r.ReplicationReconciler.GetDBStates(ctx, doltdb, clientSet)

	replicationStatus, highestEpoch := r.getReplicationStatusAndEpoch(ctx, doltdb, dbstates)

	return ctrl.Result{}, r.patchStatus(ctx, doltdb, func(status *doltv1alpha.DoltClusterStatus) error {
		status.Replicas = sts.Status.ReadyReplicas
		defaultPrimary(doltdb)

		if highestEpoch != -1 {
			doltdb.Status.UpdateReplicationEpoch(doltdb, highestEpoch)
		}

		if replicationStatus != nil {
			status.ReplicationStatus = replicationStatus
		}

		if doltdb.IsResizingStorage() || doltdb.IsSwitchingPrimary() {
			return nil
		}

		if err := r.setUpdatedCondition(ctx, doltdb); err != nil {
			log.FromContext(ctx).V(1).Info("error setting DoltDB updated condition", "err", err)
		}
		conditions.SetReadyWithDoltCluster(&doltdb.Status, &sts, doltdb)
		return nil
	})
}

func (r *DoltDBReconciler) getStatefulSetRevision(ctx context.Context, doltdb *doltv1alpha.DoltCluster) (string, error) {
	var sts appsv1.StatefulSet
	if err := r.Get(ctx, client.ObjectKeyFromObject(doltdb), &sts); err != nil {
		return "", err
	}
	return sts.Status.UpdateRevision, nil
}

func (r *DoltDBReconciler) patchStatus(ctx context.Context, doltdb *doltv1alpha.DoltCluster,
	patcher patcherDoltDB) error {
	patch := client.MergeFrom(doltdb.DeepCopy())
	if err := patcher(&doltdb.Status); err != nil {
		return err
	}
	return r.Status().Patch(ctx, doltdb, patch)
}

func (r *DoltDBReconciler) setUpdatedCondition(ctx context.Context, doltdb *doltv1alpha.DoltCluster) error {
	stsUpdateRevision, err := r.getStatefulSetRevision(ctx, doltdb)
	if err != nil {
		return err
	}
	if stsUpdateRevision == "" {
		return nil
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
		return fmt.Errorf("error listing Pods: %v", err)
	}

	podsUpdated := 0
	for _, pod := range list.Items {
		if podpkg.PodUpdated(&pod, stsUpdateRevision) {
			podsUpdated++
		}
	}

	logger := log.FromContext(ctx)

	if podsUpdated >= int(doltdb.Spec.Replicas) {
		logger.V(1).Info("DoltDB is up to date")
		conditions.SetUpdated(&doltdb.Status)
	} else if podsUpdated > 0 {
		logger.V(1).Info("DoltDB update in progress")
		conditions.SetUpdating(&doltdb.Status)
	} else {
		logger.V(1).Info("DoltDB has a pending update")
		conditions.SetPendingUpdate(&doltdb.Status)
	}
	return nil
}

func (r *DoltDBReconciler) getReplicationStatusAndEpoch(ctx context.Context,
	doltdb *doltv1alpha.DoltCluster, dbstates []dolt.DBState) (doltv1alpha.ReplicationStatus, int) {
	if !doltdb.Replication().Enabled {
		return nil, -1
	}

	highestEpoch := -1

	replicationStatus := make(doltv1alpha.ReplicationStatus)
	logger := log.FromContext(ctx)
	for i := 0; i < int(doltdb.Spec.Replicas); i++ {
		dbstate := dbstates[i]
		podName := statefulset.PodName(doltdb.ObjectMeta, i)

		if dbstate.Err != nil {
			logger.V(1).Info("error getting DB state to obtain replication state", "pod", podName, "err", dbstate.Err.Error())
			if dbstate.Role == "" || dbstate.Epoch == 0 {
				continue
			}
		}

		if dbstate.Epoch > highestEpoch && dbstate.Epoch > ptr.Deref(doltdb.Status.ReplicationEpoch, 0) {
			highestEpoch = dbstate.Epoch
		}

		_, healthy, err := health.IsDoltDBReplicaHealthy(ctx, r.Client, doltdb, i)
		if err != nil {
			logger.V(1).Info("error getting replica readiness", "pod", podName, "err", err.Error())
			continue
		}
		if !healthy {
			continue
		}

		state := doltv1alpha.ReplicationStateNotConfigured

		if dbstate.Role == dolt.PrimaryRoleValue.String() {
			// If there's more than one primary, we need to reconcile, marking pod replication status as broken
			if doltdb.Status.CurrentPrimary != nil && *doltdb.Status.CurrentPrimary != podName {
				logger.V(1).Info("more than 1 primary", "pod", podName, "state", dbstate, "currentPrimary", *doltdb.Status.CurrentPrimary)
				continue
			}
			state = doltv1alpha.ReplicationStatePrimary
		} else if dbstate.Role == dolt.StandbyRoleValue.String() {
			state = doltv1alpha.ReplicationStateStandby
		}

		logger.V(0).Info("setting replication status", "pod", podName, "state", state)

		replicationStatus[podName] = state
	}

	return replicationStatus, highestEpoch
}

func defaultPrimary(doltdb *doltv1alpha.DoltCluster) {
	if doltdb.Status.CurrentPrimaryPodIndex != nil || doltdb.Status.CurrentPrimary != nil {
		return
	}
	podIndex := 0
	if doltdb.Replication().Enabled {
		primaryReplication := ptr.Deref(doltdb.Replication().Primary, doltv1alpha.PrimaryReplication{})
		podIndex = ptr.Deref(primaryReplication.PodIndex, 0)
	}
	doltdb.Status.CurrentPrimaryPodIndex = &podIndex
	doltdb.Status.CurrentPrimary = ptr.To(statefulset.PodName(doltdb.ObjectMeta, podIndex))
}
