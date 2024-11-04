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
	"github.com/electronicarts/doltdb-operator/pkg/controller"
	"github.com/electronicarts/doltdb-operator/pkg/controller/replication"
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

// DoltDBReconciler reconciles a DoltCluster object
type DoltDBReconciler struct {
	client.Client
	Scheme *runtime.Scheme

	Builder        *builder.Builder
	ConditionReady *conditions.Ready
	RefResolver    *refresolver.RefResolver

	RBACReconciler        *controller.RBACReconciler
	ConfigMapReconciler   *controller.ConfigMapReconciler
	ServiceReconciler     *controller.ServiceReconciler
	StatefulSetReconciler *controller.StatefulSetReconciler
	ReplicationReconciler *replication.ReplicationReconciler
}

type reconcilePhaseDoltCluster struct {
	Name      string
	Reconcile func(context.Context, *doltv1alpha.DoltCluster) (ctrl.Result, error)
}

// +kubebuilder:rbac:groups=k8s.dolthub.com,resources=doltclusters,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=k8s.dolthub.com,resources=doltclusters/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=k8s.dolthub.com,resources=doltclusters/finalizers,verbs=update
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

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
func (r *DoltDBReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := log.FromContext(ctx)

	// Fetch DoltCluster CRD in current namespace
	var doltCluster doltv1alpha.DoltCluster
	if err := r.Get(ctx, req.NamespacedName, &doltCluster); err != nil {
		log.Error(err, "unable to fetch DoltCluster")
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	phases := []reconcilePhaseDoltCluster{
		{
			Name:      "Status",
			Reconcile: r.reconcileStatus,
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
			Reconcile: r.reconcileStorage,
		},
		{
			Name:      "StatefulSet",
			Reconcile: r.reconcileStatefulSet,
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
		result, err := p.Reconcile(ctx, &doltCluster)
		if err != nil {
			if apierrors.IsNotFound(err) {
				continue
			}

			var errBundle *multierror.Error
			errBundle = multierror.Append(errBundle, err)

			msg := fmt.Sprintf("Error reconciling %s: %v", p.Name, err)
			patchErr := r.patchStatus(ctx, &doltCluster, func(s *doltv1alpha.DoltClusterStatus) error {
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
		For(&doltv1alpha.DoltCluster{}).
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

func (r *DoltDBReconciler) reconcileConfigMap(ctx context.Context, doltdb *doltv1alpha.DoltCluster) (ctrl.Result, error) {
	defaultConfigMapKeyRef := doltdb.DefaultConfigMapKey()

	config, err := dolt.GenerateConfigMapData(doltdb)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("error generating DoltDB ConfigMap data: %v", err)
	}

	req := controller.ConfigMapReconcileRequest{
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

func (r *DoltDBReconciler) reconcileService(ctx context.Context, doltdb *doltv1alpha.DoltCluster) (ctrl.Result, error) {
	if err := r.reconcileInternalService(ctx, doltdb); err != nil {
		return ctrl.Result{}, fmt.Errorf("error reconciling internal Service: %v", err)
	}

	// if doltdb.Replication().Enabled {
	if err := r.reconcilePrimarylService(ctx, doltdb); err != nil {
		return ctrl.Result{}, fmt.Errorf("error reconciling primary Service: %v", err)
	}
	if err := r.reconcileReaderService(ctx, doltdb); err != nil {
		return ctrl.Result{}, fmt.Errorf("error reconciling reader Service: %v", err)
	}
	// }
	return ctrl.Result{}, nil
}

func (r *DoltDBReconciler) reconcileInternalService(ctx context.Context, doltdb *doltv1alpha.DoltCluster) error {
	internalHeadlessSvc, err := r.Builder.BuildDoltInternalService(doltdb)
	if err != nil {
		return fmt.Errorf("error building internal Service: %v", err)
	}

	return r.ServiceReconciler.Reconcile(ctx, internalHeadlessSvc)
}

func (r *DoltDBReconciler) reconcilePrimarylService(ctx context.Context, doltdb *doltv1alpha.DoltCluster) error {
	primarySvc, err := r.Builder.BuildDoltPrimaryService(doltdb)
	if err != nil {
		return fmt.Errorf("error building primary Service: %v", err)
	}

	return r.ServiceReconciler.Reconcile(ctx, primarySvc)
}

func (r *DoltDBReconciler) reconcileReaderService(ctx context.Context, doltdb *doltv1alpha.DoltCluster) error {
	primarySvc, err := r.Builder.BuildDoltReaderService(doltdb)
	if err != nil {
		return fmt.Errorf("error building reader Service: %v", err)
	}

	return r.ServiceReconciler.Reconcile(ctx, primarySvc)
}

func (r *DoltDBReconciler) reconcileStatefulSet(ctx context.Context, doltdb *doltv1alpha.DoltCluster) (ctrl.Result, error) {
	key := client.ObjectKeyFromObject(doltdb)

	desiredSts, err := r.Builder.BuildDoltStatefulSet(key, doltdb)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("error building StatefulSet: %v", err)
	}

	if err := r.StatefulSetReconciler.ReconcileWithUpdates(ctx, desiredSts); err != nil {
		return ctrl.Result{}, fmt.Errorf("error reconciling StatefulSet: %v", err)
	}

	if result, err := r.reconcileUpdates(ctx, doltdb); !result.IsZero() || err != nil {
		return result, err
	}
	return ctrl.Result{}, nil
}

func (r *DoltDBReconciler) reconcilePodLabels(ctx context.Context, doltdb *doltv1alpha.DoltCluster) (ctrl.Result, error) {
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

		if pod.Status.PodIP == "" || pod.Spec.NodeName == "" {
			continue
		}
		podIndex, err := statefulset.PodIndex(pod.Name)
		if err != nil {
			return ctrl.Result{}, fmt.Errorf("error getting Pod '%s' index: %v", pod.Name, err)
		}

		podLabels.WithStatefulSetPod(doltdb, *podIndex)

		if *podIndex == *doltdb.Status.CurrentPrimaryPodIndex {
			pod.Labels = podLabels.WithPodPrimaryRole().Build()
		} else {
			pod.Labels = podLabels.WithPodStandbyRole().Build()
		}

		p := client.MergeFrom(pod.DeepCopy())

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

func (r *DoltDBReconciler) reconcileRBAC(ctx context.Context, doltdb *doltv1alpha.DoltCluster) (ctrl.Result, error) {
	return ctrl.Result{}, r.RBACReconciler.ReconcileDoltRBAC(ctx, doltdb)
}

func (r *DoltDBReconciler) reconcilePodDisruptionBudget(ctx context.Context, doltdb *doltv1alpha.DoltCluster) (ctrl.Result, error) {
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
