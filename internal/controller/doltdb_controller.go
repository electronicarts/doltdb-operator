/*
Copyright 2024.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

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
// +kubebuilder:rbac:groups="",resources=services,verbs=list;watch;create;patch
// +kubebuilder:rbac:groups="",resources=secrets,verbs=list;watch;create;patch
// +kubebuilder:rbac:groups="",resources=pods,verbs=get;patch;delete
// +kubebuilder:rbac:groups="",resources=pods/log,verbs=get
// +kubebuilder:rbac:groups="",resources=persistentvolumeclaims,verbs=list
// +kubebuilder:rbac:groups="",resources=events,verbs=list;watch;create;patch
// +kubebuilder:rbac:groups="",resources=serviceaccounts,verbs=list;watch;create;patch
// +kubebuilder:rbac:groups=apps,resources=statefulsets,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=policy,resources=poddisruptionbudgets,verbs=list;watch;create;patch
// +kubebuilder:rbac:groups=rbac.authorization.k8s.io,resources=roles;rolebindings;clusterrolebindings,verbs=list;watch;create;patch
// +kubebuilder:rbac:groups=authorization.k8s.io,resources=subjectaccessreviews,verbs=create
// +kubebuilder:rbac:groups=authentication.k8s.io,resources=tokenreviews,verbs=create
// +kubebuilder:rbac:groups="",resources=endpoints,verbs=list

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
func (r *DoltDBReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := log.FromContext(ctx)

	// Fetch DoltDB CRD in current namespace
	var doltdb doltv1alpha.DoltDB
	if err := r.Get(ctx, req.NamespacedName, &doltdb); err != nil {
		log.Error(err, "unable to fetch DoltDB")
		return ctrl.Result{}, client.IgnoreNotFound(err)
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
	}

	return ctrl.Result{}, nil
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

func (r *DoltDBReconciler) reconcilePodLabels(ctx context.Context, doltdb *doltv1alpha.DoltDB) (ctrl.Result, error) {
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

func (r *DoltDBReconciler) reconcileRBAC(ctx context.Context, doltdb *doltv1alpha.DoltDB) (ctrl.Result, error) {
	return ctrl.Result{}, r.RBACReconciler.ReconcileDoltRBAC(ctx, doltdb)
}

func (r *DoltDBReconciler) reconcilePodDisruptionBudget(ctx context.Context, doltdb *doltv1alpha.DoltDB) (ctrl.Result, error) {
	key := doltdb.PodDisruptionBudgetKey()

	var existingPDB policyv1.PodDisruptionBudget
	if err := r.Get(ctx, key, &existingPDB); err == nil {
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

	return ctrl.Result{}, r.Create(ctx, pdb)
}
