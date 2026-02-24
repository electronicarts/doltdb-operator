// Copyright (c) 2025 Electronic Arts Inc. All rights reserved.

package controller

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/hashicorp/go-multierror"
	corev1 "k8s.io/api/core/v1"
	klabels "k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	doltv1alpha "github.com/electronicarts/doltdb-operator/api/v1alpha"
	"github.com/electronicarts/doltdb-operator/pkg/builder"
	"github.com/electronicarts/doltdb-operator/pkg/conditions"
	"github.com/electronicarts/doltdb-operator/pkg/controller/configmap"
	"github.com/electronicarts/doltdb-operator/pkg/controller/rbac"
	"github.com/electronicarts/doltdb-operator/pkg/controller/replication"
	"github.com/electronicarts/doltdb-operator/pkg/controller/service"
	stsctrl "github.com/electronicarts/doltdb-operator/pkg/controller/statefulset"
	"github.com/electronicarts/doltdb-operator/pkg/controller/status"
	"github.com/electronicarts/doltdb-operator/pkg/controller/storage"
	"github.com/electronicarts/doltdb-operator/pkg/dolt"
	doltpod "github.com/electronicarts/doltdb-operator/pkg/pod"
	"github.com/electronicarts/doltdb-operator/pkg/refresolver"
	"github.com/electronicarts/doltdb-operator/pkg/statefulset"
	appsv1 "k8s.io/api/apps/v1"
	policyv1 "k8s.io/api/policy/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
	k8sctrl "sigs.k8s.io/controller-runtime/pkg/controller"
)

// DoltDBReconciler reconciles a DoltDB object
type DoltDBReconciler struct {
	client.Client
	Scheme *runtime.Scheme

	Builder        *builder.Builder
	ConditionReady *conditions.Ready
	RefResolver    *refresolver.RefResolver

	RBACReconciler        *rbac.Reconciler
	ConfigMapReconciler   *configmap.Reconciler
	ServiceReconciler     *service.Reconciler
	StatefulSetReconciler *stsctrl.Reconciler
	StorageReconciler     *storage.Reconciler
	StatusReconciler      *status.Reconciler
	ReplicationReconciler *replication.ReplicationReconciler
}

type reconcilePhaseDoltDB struct {
	Name      string
	Reconcile func(context.Context, *doltv1alpha.DoltDB) (ctrl.Result, error)
}

// +kubebuilder:rbac:groups=k8s.dolthub.com,resources=doltdbs,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=k8s.dolthub.com,resources=doltdbs/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=k8s.dolthub.com,resources=doltdbs/finalizers,verbs=update
// +kubebuilder:rbac:groups="",resources=secrets,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=pods,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=configmaps,verbs=get;list;watch;create;patch;delete
// +kubebuilder:rbac:groups="",resources=services,verbs=list;watch;create;patch;delete
// +kubebuilder:rbac:groups="",resources=secrets,verbs=list;watch;create;patch
// +kubebuilder:rbac:groups="",resources=pods,verbs=get;patch;delete
// +kubebuilder:rbac:groups="",resources=pods/log,verbs=get
// +kubebuilder:rbac:groups="",resources=persistentvolumeclaims,verbs=list;get;watch;patch;create
// +kubebuilder:rbac:groups="",resources=events,verbs=list;watch;create;patch
// +kubebuilder:rbac:groups="",resources=serviceaccounts,verbs=list;watch;create;patch
// +kubebuilder:rbac:groups=apps,resources=statefulsets,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=policy,resources=poddisruptionbudgets,verbs=list;watch;create;patch;delete
// +kubebuilder:rbac:groups=rbac.authorization.k8s.io,resources=roles;rolebindings;clusterrolebindings,verbs=list;watch;create;patch
// +kubebuilder:rbac:groups=authorization.k8s.io,resources=subjectaccessreviews,verbs=create
// +kubebuilder:rbac:groups=authentication.k8s.io,resources=tokenreviews,verbs=create
// +kubebuilder:rbac:groups="",resources=endpoints,verbs=list;get;watch;create;update;delete

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
func (r *DoltDBReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := log.FromContext(ctx)

	log.WithValues("namespace", req.NamespacedName, "doltdb", req.Name).
		Info("Running reconciler for DoltDB")

	// Fetch DoltDB CRD in current namespace
	var doltdb doltv1alpha.DoltDB
	if err := r.Get(ctx, req.NamespacedName, &doltdb); err != nil {
		log.Error(err, "unable to fetch DoltDB")
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	// Validate spec before proceeding with reconciliation
	// TODO: Move this validation to admission webhook and make it more comprehensive
	if err := doltdb.ValidateReplicationSpec(); err != nil {
		msg := fmt.Sprintf("Invalid spec: %v", err)
		patchErr := status.PatchStatus(ctx, r.Client, &doltdb, func(s *doltv1alpha.DoltDBStatus) error {
			s.SetCondition(metav1.Condition{
				Type:    doltv1alpha.ConditionTypeReady,
				Status:  metav1.ConditionFalse,
				Reason:  doltv1alpha.ConditionReasonInvalidSpec,
				Message: msg,
			})
			return nil
		})
		if patchErr != nil {
			return ctrl.Result{}, fmt.Errorf("error patching status: %v", patchErr)
		}
		log.Error(err, "Invalid DoltDB spec")
		return ctrl.Result{}, nil
	}

	phases := []reconcilePhaseDoltDB{
		{
			Name:      "Status",
			Reconcile: r.StatusReconciler.Reconcile,
		},
		{
			Name:      "ConfigMap",
			Reconcile: r.reconcileConfigMap,
		},
		{
			Name:      "RBAC",
			Reconcile: r.reconcileRBAC,
		},
		{
			Name:      "Storage",
			Reconcile: r.StorageReconciler.Reconcile,
		},
		{
			Name:      "PreScaleDown",
			Reconcile: r.reconcilePreScaleDown,
		},
		{
			Name:      "StatefulSet",
			Reconcile: r.StatefulSetReconciler.Reconcile,
		},
		{
			Name:      "PodDisruptionBudget",
			Reconcile: r.reconcilePodDisruptionBudget,
		},
		{
			Name:      "Service",
			Reconcile: r.reconcileService,
		},
		{
			Name:      "Replication",
			Reconcile: r.ReplicationReconciler.Reconcile,
		},
		{
			Name:      "Labels",
			Reconcile: r.reconcilePodLabels,
		},
	}
	for _, p := range phases {
		result, err := p.Reconcile(ctx, &doltdb)
		if err != nil {
			if shouldSkipPhase(err) {
				continue
			}

			var errBundle *multierror.Error
			errBundle = multierror.Append(errBundle, err)

			msg := fmt.Sprintf("Error reconciling %s: %v", p.Name, err)
			patchErr := status.PatchStatus(ctx, r.Client, &doltdb, func(s *doltv1alpha.DoltDBStatus) error {
				patcher := r.ConditionReady.PatcherFailed(msg)
				patcher(s)
				return nil
			})
			if !apierrors.IsNotFound(patchErr) {
				errBundle = multierror.Append(errBundle, patchErr)
			}

			if err := errBundle.ErrorOrNil(); err != nil {
				return ctrl.Result{}, fmt.Errorf("error reconciling %s: %v", p.Name, err)
			}
		}
		if !result.IsZero() {
			return result, err
		}
	}
	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *DoltDBReconciler) SetupWithManager(mgr ctrl.Manager, opts k8sctrl.Options) error {
	builder := ctrl.NewControllerManagedBy(mgr).
		For(&doltv1alpha.DoltDB{}).
		Owns(&corev1.ConfigMap{}).
		Owns(&corev1.Service{}).
		Owns(&corev1.Event{}).
		Owns(&corev1.ServiceAccount{}).
		Owns(&appsv1.StatefulSet{}).
		Owns(&appsv1.Deployment{}).
		Owns(&policyv1.PodDisruptionBudget{}).
		Owns(&rbacv1.Role{}).
		Owns(&rbacv1.RoleBinding{}).
		WithOptions(opts)

	// TODO: Add watchers for indexes

	return builder.Complete(r)
}

func shouldSkipPhase(err error) bool {
	if apierrors.IsNotFound(err) {
		return true
	}
	if errors.Is(err, stsctrl.ErrSkipReconciliationPhase) {
		return true
	}
	return false
}

func (r *DoltDBReconciler) reconcileConfigMap(ctx context.Context, doltdb *doltv1alpha.DoltDB) (ctrl.Result, error) {
	defaultConfigMapKeyRef := doltdb.DefaultConfigMapKey()

	config, err := dolt.GenerateConfigMapData(doltdb)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("error generating DoltDB ConfigMap data: %v", err)
	}

	req := configmap.ReconcileRequest{
		Metadata: &doltdb.ObjectMeta,
		Owner:    doltdb,
		Key:      defaultConfigMapKeyRef,
		Data:     config,
	}
	if err := r.ConfigMapReconciler.Reconcile(ctx, &req); err != nil {
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

func (r *DoltDBReconciler) reconcileService(ctx context.Context, doltdb *doltv1alpha.DoltDB) (ctrl.Result, error) {
	if result, err := r.reconcileInternalService(ctx, doltdb); !result.IsZero() || err != nil {
		return ctrl.Result{}, fmt.Errorf("error reconciling internal Service: %v", err)
	}

	if doltdb.Replication().Enabled {
		if result, err := r.reconcilePrimaryService(ctx, doltdb); !result.IsZero() || err != nil {
			return ctrl.Result{}, fmt.Errorf("error reconciling primary Service: %v", err)
		}
		if result, err := r.reconcileReaderService(ctx, doltdb); !result.IsZero() || err != nil {
			return ctrl.Result{}, fmt.Errorf("error reconciling reader Service: %v", err)
		}
		// Clean up standalone service from previous mode
		if err := r.deleteServiceIfExists(ctx, doltdb.ServiceKey()); err != nil {
			return ctrl.Result{}, fmt.Errorf("error deleting stale standalone Service: %v", err)
		}
	} else {
		if result, err := r.reconcileStandaloneService(ctx, doltdb); !result.IsZero() || err != nil {
			return ctrl.Result{}, fmt.Errorf("error reconciling standalone Service: %v", err)
		}
		// Clean up replication services from previous mode
		if err := r.deleteServiceIfExists(ctx, doltdb.PrimaryServiceKey()); err != nil {
			return ctrl.Result{}, fmt.Errorf("error deleting stale primary Service: %v", err)
		}
		if err := r.deleteServiceIfExists(ctx, doltdb.ReaderServiceKey()); err != nil {
			return ctrl.Result{}, fmt.Errorf("error deleting stale reader Service: %v", err)
		}
	}

	return ctrl.Result{}, nil
}

func (r *DoltDBReconciler) deleteServiceIfExists(ctx context.Context, key types.NamespacedName) error {
	var svc corev1.Service
	if err := r.Get(ctx, key, &svc); err != nil {
		return client.IgnoreNotFound(err)
	}
	return r.Delete(ctx, &svc)
}

func (r *DoltDBReconciler) reconcileInternalService(ctx context.Context, doltdb *doltv1alpha.DoltDB) (ctrl.Result, error) {
	internalHeadlessSvc, err := r.Builder.BuildDoltInternalService(doltdb)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("error building internal Service: %v", err)
	}

	return ctrl.Result{}, r.ServiceReconciler.Reconcile(ctx, internalHeadlessSvc)
}

func (r *DoltDBReconciler) reconcilePrimaryService(ctx context.Context, doltdb *doltv1alpha.DoltDB) (ctrl.Result, error) {
	if doltdb.Status.CurrentPrimaryPodIndex == nil {
		log.FromContext(ctx).V(1).Info("'status.currentPrimaryPodIndex' must be set")
		return ctrl.Result{RequeueAfter: 1 * time.Second}, nil
	}

	primarySvc, err := r.Builder.BuildDoltPrimaryService(doltdb)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("error building primary Service: %v", err)
	}

	return ctrl.Result{}, r.ServiceReconciler.Reconcile(ctx, primarySvc)
}

func (r *DoltDBReconciler) reconcileReaderService(ctx context.Context, doltdb *doltv1alpha.DoltDB) (ctrl.Result, error) {
	primarySvc, err := r.Builder.BuildDoltReaderService(doltdb)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("error building reader Service: %v", err)
	}

	return ctrl.Result{}, r.ServiceReconciler.Reconcile(ctx, primarySvc)
}

func (r *DoltDBReconciler) reconcileStandaloneService(ctx context.Context, doltdb *doltv1alpha.DoltDB) (ctrl.Result, error) {
	svc, err := r.Builder.BuildDoltService(doltdb)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("error building standalone Service: %v", err)
	}

	return ctrl.Result{}, r.ServiceReconciler.Reconcile(ctx, svc)
}

func (r *DoltDBReconciler) reconcilePodLabels(ctx context.Context, doltdb *doltv1alpha.DoltDB) (ctrl.Result, error) {
	if !doltdb.Replication().Enabled {
		// In standalone mode, remove stale role labels from pods
		return r.cleanPodRoleLabels(ctx, doltdb)
	}

	if doltdb.Status.CurrentPrimaryPodIndex == nil {
		return ctrl.Result{RequeueAfter: 1 * time.Second}, nil
	}

	podList := corev1.PodList{}
	listOpts := &client.ListOptions{
		LabelSelector: klabels.SelectorFromSet(
			builder.NewLabelsBuilder().
				WithDoltSelectorLabels(doltdb).
				Build(),
		),
		Namespace: doltdb.GetNamespace(),
	}
	if err := r.List(ctx, &podList, listOpts); err != nil {
		return ctrl.Result{}, fmt.Errorf("error listing Pods: %v", err)
	}

	for _, pod := range podList.Items {
		podLabels := builder.NewLabelsBuilder().WithLabels(pod.Labels)

		if doltdb.Status.CurrentPrimaryPodIndex == nil {
			log.FromContext(ctx).V(1).Info("'status.currentPrimaryPodIndex' must be set")
			return ctrl.Result{RequeueAfter: 1 * time.Second}, nil
		}

		if pod.Status.PodIP == "" || pod.Spec.NodeName == "" {
			continue
		}
		podIndex, err := statefulset.PodIndex(pod.Name)
		if err != nil {
			return ctrl.Result{}, fmt.Errorf("error getting Pod '%s' index: %v", pod.Name, err)
		}

		p := client.MergeFrom(pod.DeepCopy())

		podLabels.WithStatefulSetPod(doltdb, *podIndex)

		if *podIndex == *doltdb.Status.CurrentPrimaryPodIndex {
			pod.Labels = podLabels.WithPodPrimaryRole().Build()
		} else {
			pod.Labels = podLabels.WithPodStandbyRole().Build()
		}

		if doltpod.PodReady(&pod) {
			if err := r.Patch(ctx, &pod, p); err != nil {
				if apierrors.IsConflict(err) {
					return ctrl.Result{Requeue: true}, nil
				}
				if apierrors.IsNotFound(err) {
					return ctrl.Result{Requeue: true}, nil
				}
				return ctrl.Result{}, err
			}
		}
	}

	return ctrl.Result{}, nil
}

func (r *DoltDBReconciler) cleanPodRoleLabels(
	ctx context.Context,
	doltdb *doltv1alpha.DoltDB,
) (ctrl.Result, error) {
	podList := corev1.PodList{}
	listOpts := &client.ListOptions{
		LabelSelector: klabels.SelectorFromSet(
			builder.NewLabelsBuilder().
				WithDoltSelectorLabels(doltdb).
				Build(),
		),
		Namespace: doltdb.GetNamespace(),
	}
	if err := r.List(ctx, &podList, listOpts); err != nil {
		return ctrl.Result{}, fmt.Errorf("error listing Pods: %v", err)
	}

	for _, pod := range podList.Items {
		if _, hasRole := pod.Labels[dolt.RoleLabel]; !hasRole {
			continue
		}
		p := client.MergeFrom(pod.DeepCopy())
		delete(pod.Labels, dolt.RoleLabel)
		if err := r.Patch(ctx, &pod, p); err != nil {
			if apierrors.IsConflict(err) {
				return ctrl.Result{Requeue: true}, nil
			}
			if apierrors.IsNotFound(err) {
				return ctrl.Result{Requeue: true}, nil
			}
			return ctrl.Result{}, err
		}
	}

	return ctrl.Result{}, nil
}

func (r *DoltDBReconciler) reconcileRBAC(ctx context.Context, doltdb *doltv1alpha.DoltDB) (ctrl.Result, error) {
	return ctrl.Result{}, r.RBACReconciler.ReconcileDoltRBAC(ctx, doltdb)
}

// reconcilePreScaleDown ensures the primary is transitioned to pod-0 before a scale-down
// that would delete the current primary pod. StatefulSets delete highest-ordinal pods first,
// so if the primary has an index >= the desired replica count, it would be terminated without
// a graceful switchover. This phase triggers the existing switchover mechanism by patching
// spec.replication.primary.podIndex to 0 and requeueing.
func (r *DoltDBReconciler) reconcilePreScaleDown(
	ctx context.Context,
	doltdb *doltv1alpha.DoltDB,
) (ctrl.Result, error) {
	if !doltdb.Replication().Enabled || doltdb.Status.CurrentPrimaryPodIndex == nil {
		return ctrl.Result{}, nil
	}

	var sts appsv1.StatefulSet
	if err := r.Get(ctx, client.ObjectKeyFromObject(doltdb), &sts); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	currentReplicas := ptr.Deref(sts.Spec.Replicas, 0)
	desiredReplicas := doltdb.Spec.Replicas
	if desiredReplicas >= currentReplicas {
		return ctrl.Result{}, nil
	}

	primaryIndex := *doltdb.Status.CurrentPrimaryPodIndex
	if primaryIndex < int(desiredReplicas) {
		return ctrl.Result{}, nil
	}

	logger := log.FromContext(ctx).WithName("pre-scale-down")
	logger.Info(
		"Primary pod would be deleted by scale-down, triggering switchover to pod-0",
		"currentPrimaryIndex", primaryIndex,
		"desiredReplicas", desiredReplicas,
	)

	// Patch spec to move primary to pod-0, triggering the existing switchover mechanism.
	patch := client.MergeFrom(doltdb.DeepCopy())
	doltdb.Spec.Replication.Primary.PodIndex = ptr.To(0)
	if err := r.Patch(ctx, doltdb, patch); err != nil {
		return ctrl.Result{}, fmt.Errorf("error patching DoltDB to transition primary before scale-down: %v", err)
	}

	return ctrl.Result{Requeue: true}, nil
}

func (r *DoltDBReconciler) reconcilePodDisruptionBudget(ctx context.Context, doltdb *doltv1alpha.DoltDB) (ctrl.Result, error) {
	key := doltdb.PodDisruptionBudgetKey()

	var existingPDB policyv1.PodDisruptionBudget
	exists := r.Get(ctx, key, &existingPDB) == nil

	// Single instance: PDB blocks eviction of the only pod, so delete it
	if doltdb.Spec.Replicas == 1 {
		if exists {
			if err := r.Delete(ctx, &existingPDB); err != nil {
				return ctrl.Result{}, fmt.Errorf("error deleting PodDisruptionBudget: %v", err)
			}
		}
		return ctrl.Result{}, nil
	}

	selectorLabels :=
		builder.NewLabelsBuilder().
			WithDoltSelectorLabels(doltdb).
			Build()
	minAvailable := intstr.FromString("50%")
	opts := builder.PodDisruptionBudgetOpts{
		Metadata:       &doltdb.ObjectMeta,
		Key:            key,
		MinAvailable:   &minAvailable,
		SelectorLabels: selectorLabels,
	}
	pdb, err := r.Builder.BuildPodDisruptionBudget(opts, doltdb)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("error building PodDisruptionBudget: %v", err)
	}

	if exists {
		patch := client.MergeFrom(existingPDB.DeepCopy())
		existingPDB.Spec = pdb.Spec
		existingPDB.Labels = pdb.Labels
		return ctrl.Result{}, r.Patch(ctx, &existingPDB, patch)
	}
	return ctrl.Result{}, r.Create(ctx, pdb)
}
