package controller

import (
	"context"
	"fmt"

	doltv1alpha "github.com/electronicarts/doltdb-operator/api/v1alpha"
	"github.com/electronicarts/doltdb-operator/pkg/builder"
	"github.com/electronicarts/doltdb-operator/pkg/conditions"
	"github.com/electronicarts/doltdb-operator/pkg/controller/replication"
	"github.com/electronicarts/doltdb-operator/pkg/dolt"
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

	replicationStatus, replErr := r.getReplicationStatus(ctx, doltdb)
	if replErr != nil {
		log.FromContext(ctx).V(1).Info("error getting replication status", "err", replErr)
	}

	return ctrl.Result{}, r.patchStatus(ctx, doltdb, func(status *doltv1alpha.DoltClusterStatus) error {
		status.Replicas = sts.Status.ReadyReplicas
		defaultPrimary(doltdb)

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

func (r *DoltDBReconciler) getReplicationStatus(ctx context.Context,
	doltdb *doltv1alpha.DoltCluster) (doltv1alpha.ReplicationStatus, error) {
	if !doltdb.Replication().Enabled {
		return nil, nil
	}

	clientSet, err := replication.NewReplicationClientSet(doltdb, r.RefResolver)
	if err != nil {
		return nil, fmt.Errorf("error creating DoltCluster clientset: %v", err)
	}
	defer clientSet.Close()

	replicationStatus := make(doltv1alpha.ReplicationStatus)
	logger := log.FromContext(ctx)
	for i := 0; i < int(doltdb.Spec.Replicas); i++ {
		pod := statefulset.PodName(doltdb.ObjectMeta, i)

		client, err := clientSet.ClientForIndex(ctx, i)
		if err != nil {
			logger.V(1).Info("error getting client for Pod", "err", err, "pod", pod)
			continue
		}

		role, _, err := client.GetRoleAndEpoch(ctx)
		if err != nil {
			logger.V(1).Info("error checking Pod replication state", "err", err, "pod", pod)
			continue
		}

		state := doltv1alpha.ReplicationStateNotConfigured
		if role == dolt.PrimaryRoleValue.String() {
			state = doltv1alpha.ReplicationStatePrimary
		} else if role == dolt.StandbyRoleValue.String() {
			state = doltv1alpha.ReplicationStateStandby
		}
		replicationStatus[pod] = state
	}
	return replicationStatus, nil
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
