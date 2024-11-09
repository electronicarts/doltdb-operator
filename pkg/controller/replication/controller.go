package replication

import (
	"context"
	"fmt"
	"time"

	"github.com/go-logr/logr"
	doltv1alpha "github.com/electronicarts/doltdb-operator/api/v1alpha"
	"github.com/electronicarts/doltdb-operator/pkg/builder"
	"github.com/electronicarts/doltdb-operator/pkg/controller/configmap"
	"github.com/electronicarts/doltdb-operator/pkg/controller/service"
	"github.com/electronicarts/doltdb-operator/pkg/dolt"
	"github.com/electronicarts/doltdb-operator/pkg/health"
	"github.com/electronicarts/doltdb-operator/pkg/refresolver"
	"github.com/electronicarts/doltdb-operator/pkg/statefulset"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

type Option func(*ReplicationReconciler)

func WithRefResolver(rr *refresolver.RefResolver) Option {
	return func(r *ReplicationReconciler) {
		r.refResolver = rr
	}
}

func WithServiceReconciler(sr *service.Reconciler) Option {
	return func(rr *ReplicationReconciler) {
		rr.serviceReconciler = sr
	}
}

type ReplicationReconciler struct {
	client.Client
	recorder            record.EventRecorder
	builder             *builder.Builder
	replConfig          *ReplicationConfig
	refResolver         *refresolver.RefResolver
	configMapreconciler *configmap.Reconciler
	serviceReconciler   *service.Reconciler
}

func NewReconciler(client client.Client, recorder record.EventRecorder, builder *builder.Builder, replConfig *ReplicationConfig,
	opts ...Option) (*ReplicationReconciler, error) {
	r := &ReplicationReconciler{
		Client:     client,
		recorder:   recorder,
		builder:    builder,
		replConfig: replConfig,
	}
	for _, setOpt := range opts {
		setOpt(r)
	}
	if r.refResolver == nil {
		r.refResolver = refresolver.New(client)
	}
	if r.configMapreconciler == nil {
		r.configMapreconciler = configmap.NewReconciler(client, builder)
	}
	if r.serviceReconciler == nil {
		r.serviceReconciler = service.NewReconciler(client)
	}
	return r, nil
}

type reconcileRequest struct {
	doltdb    *doltv1alpha.DoltDB
	key       types.NamespacedName
	clientSet *ReplicationClientSet
}

func (r *ReplicationReconciler) Reconcile(ctx context.Context, doltdb *doltv1alpha.DoltDB) (ctrl.Result, error) {
	logger := log.FromContext(ctx).WithName("replication")
	switchoverLogger := log.FromContext(ctx).WithName("switchover")

	doltdbKey := client.ObjectKeyFromObject(doltdb)

	if doltdb.IsSwitchingPrimary() {
		clientSet := NewReplicationClientSet(doltdb, r.refResolver)
		defer clientSet.close()

		req := reconcileRequest{
			doltdb:    doltdb,
			key:       doltdbKey,
			clientSet: clientSet,
		}
		return ctrl.Result{}, r.reconcileSwitchover(ctx, &req, switchoverLogger)
	}

	healthy, err := health.IsStatefulSetHealthy(
		ctx,
		r.Client,
		doltdbKey,
		doltdb.InternalServiceKey(),
		health.WithDesiredReplicas(doltdb.Spec.Replicas),
		health.WithPort(dolt.DatabasePort),
		health.WithEndpointPolicy(health.EndpointPolicyAll),
	)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("error checking DoltDB health: %v", err)
	}
	if !healthy {
		return ctrl.Result{RequeueAfter: 1 * time.Second}, nil
	}

	clientSet := NewReplicationClientSet(doltdb, r.refResolver)
	defer clientSet.close()

	req := reconcileRequest{
		doltdb:    doltdb,
		key:       client.ObjectKeyFromObject(doltdb),
		clientSet: clientSet,
	}
	if result, err := r.reconcileReplication(ctx, &req, logger); !result.IsZero() || err != nil {
		return result, err
	}
	return ctrl.Result{}, r.reconcileSwitchover(ctx, &req, switchoverLogger)
}

func (r *ReplicationReconciler) reconcileReplication(ctx context.Context, req *reconcileRequest, logger logr.Logger) (ctrl.Result, error) {
	if req.doltdb.IsSwitchingPrimary() {
		return ctrl.Result{}, nil
	}
	if req.doltdb.Status.CurrentPrimaryPodIndex == nil || req.doltdb.Status.ReplicationEpoch == nil {
		return ctrl.Result{RequeueAfter: 1 * time.Second}, nil
	}

	nextReplicationEpoch := *req.doltdb.Status.ReplicationEpoch + 1

	for i := 0; i < int(req.doltdb.Spec.Replicas); i++ {
		pod := statefulset.PodName(req.doltdb.ObjectMeta, i)

		if req.doltdb.Status.ReplicationStatus == nil {
			if err := r.reconcileReplicationInPod(ctx, req, logger, i, nextReplicationEpoch); err != nil {
				return ctrl.Result{}, fmt.Errorf("error configuring replication in Pod '%s': %v", pod, err)
			}
		}

		state, ok := req.doltdb.Status.ReplicationStatus[pod]
		if !ok || state == doltv1alpha.ReplicationStateNotConfigured {
			if err := r.reconcileReplicationInPod(ctx, req, logger, i, nextReplicationEpoch); err != nil {
				return ctrl.Result{}, fmt.Errorf("error configuring replication in Pod '%s': %v", pod, err)
			}
		}
	}
	return ctrl.Result{}, nil
}

// TODO: reconcile if two primaries, but look at label...
func (r *ReplicationReconciler) reconcileReplicationInPod(ctx context.Context, req *reconcileRequest, logger logr.Logger, index int, nextReplicationEpoch int) error {
	pod := statefulset.PodName(req.doltdb.ObjectMeta, index)
	primaryPodIndex := *req.doltdb.Status.CurrentPrimaryPodIndex

	defer req.clientSet.RemoveClientFromCache(index)

	if primaryPodIndex == index {
		logger.Info("Configuring primary", "pod", pod)
		client, err := req.clientSet.currentPrimaryClient(ctx)
		if err != nil {
			return fmt.Errorf("error getting current primary client: %v", err)
		}
		return r.replConfig.ConfigurePrimary(ctx, req.doltdb, client, index, nextReplicationEpoch)
	}

	logger.Info("Configuring replica", "pod", pod)
	client, err := req.clientSet.clientForIndex(ctx, index)
	if err != nil {
		return fmt.Errorf("error getting replica client: %v", err)
	}

	return r.replConfig.ConfigureReplica(ctx, req.doltdb, client, index, nextReplicationEpoch)
}

func (r *ReplicationReconciler) patchStatus(ctx context.Context, doltdb *doltv1alpha.DoltDB,
	patcher func(*doltv1alpha.DoltDBStatus)) error {
	patch := client.MergeFrom(doltdb.DeepCopy())
	patcher(&doltdb.Status)
	return r.Status().Patch(ctx, doltdb, patch)
}
