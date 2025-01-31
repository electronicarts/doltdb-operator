package status

import (
	"context"
	"fmt"

	doltv1alpha "github.com/electronicarts/doltdb-operator/api/v1alpha"
	"github.com/electronicarts/doltdb-operator/pkg/builder"
	"github.com/electronicarts/doltdb-operator/pkg/conditions"
	"github.com/electronicarts/doltdb-operator/pkg/controller/replication"
	stsctrl "github.com/electronicarts/doltdb-operator/pkg/controller/statefulset"
	"github.com/electronicarts/doltdb-operator/pkg/dolt"
	"github.com/electronicarts/doltdb-operator/pkg/health"
	podpkg "github.com/electronicarts/doltdb-operator/pkg/pod"
	"github.com/electronicarts/doltdb-operator/pkg/refresolver"
	"github.com/electronicarts/doltdb-operator/pkg/statefulset"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	klabels "k8s.io/apimachinery/pkg/labels"
	"k8s.io/utils/ptr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

type Reconciler struct {
	client.Client
	refResolver *refresolver.RefResolver
}

// NewReconciler creates a new ServiceReconciler with the given client.
func NewReconciler(client client.Client, refResolver *refresolver.RefResolver) *Reconciler {
	return &Reconciler{
		Client:      client,
		refResolver: refResolver,
	}
}

func (r *Reconciler) Reconcile(ctx context.Context, doltdb *doltv1alpha.DoltDB) (ctrl.Result, error) {
	var sts appsv1.StatefulSet
	if err := r.Get(ctx, client.ObjectKeyFromObject(doltdb), &sts); err != nil {
		log.FromContext(ctx).V(1).Info("error getting StatefulSet", "err", err)
	}

	clientSet := replication.NewReplicationClientSet(doltdb, r.refResolver)
	defer clientSet.Close()

	dbstates := replication.GetDBStates(ctx, doltdb, clientSet)

	replicationStatus, highestEpoch := r.getReplicationStatusAndEpoch(ctx, doltdb, dbstates)

	return ctrl.Result{}, PatchStatus(ctx, r.Client, doltdb, func(status *doltv1alpha.DoltDBStatus) error {
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
		conditions.SetReadyWithDoltDB(&doltdb.Status, &sts, doltdb)
		return nil
	})
}

func (r *Reconciler) setUpdatedCondition(ctx context.Context, doltdb *doltv1alpha.DoltDB) error {
	stsUpdateRevision, err := stsctrl.GetRevision(ctx, r.Client, doltdb)
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

func (r *Reconciler) getReplicationStatusAndEpoch(ctx context.Context,
	doltdb *doltv1alpha.DoltDB, dbstates []dolt.DBState) (doltv1alpha.ReplicationStatus, int) {
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
		if dbstate.Role == "" {
			logger.V(0).Info("doltdb role not set, skipping", "pod", podName)
			continue
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
			logger.V(1).Info("dolt replica is unhealthy", "pod", podName)
			continue
		}

		state := doltv1alpha.ReplicationStateNotConfigured

		if dbstate.Role == dolt.PrimaryRoleValue.String() {
			// If there's more than one primary, we need to reconcile, marking pod replication status as broken
			if doltdb.Status.CurrentPrimary != nil && *doltdb.Status.CurrentPrimary != podName {
				logger.V(2).Info("more than 1 primary", "pod", podName, "state", dbstate, "currentPrimary", *doltdb.Status.CurrentPrimary)
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

func defaultPrimary(doltdb *doltv1alpha.DoltDB) {
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
