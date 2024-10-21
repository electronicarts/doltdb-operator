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

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	doltv1alpha "github.com/electronicarts/doltdb-operator/api/v1alpha"
	"github.com/electronicarts/doltdb-operator/pkg/builder"
	"github.com/electronicarts/doltdb-operator/pkg/controller"
	"github.com/electronicarts/doltdb-operator/pkg/dolt/config"
)

// DoltClusterReconciler reconciles a DoltCluster object
type DoltClusterReconciler struct {
	client.Client
	Scheme *runtime.Scheme

	builder *builder.Builder

	configMapReconciler *controller.ConfigMapReconciler
	serviceReconciler   *controller.ServiceReconciler
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
// +kubebuilder:rbac:groups=rbac.authorization.k8s.io,resources=roles;rolebindings;clusterrolebindings,verbs=list;watch;create;patch
// +kubebuilder:rbac:groups=authorization.k8s.io,resources=subjectaccessreviews,verbs=create
// +kubebuilder:rbac:groups=authentication.k8s.io,resources=tokenreviews,verbs=create

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
func (r *DoltClusterReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := log.FromContext(ctx)

	// Fetch DoltCluster CRD in current namespace
	var doltCluster doltv1alpha.DoltCluster
	if err := r.Get(ctx, req.NamespacedName, &doltCluster); err != nil {
		log.Error(err, "unable to fetch DoltCluster")
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	phases := []reconcilePhaseDoltCluster{
		{
			Name:      "ConfigMap",
			Reconcile: r.reconcileConfigMap,
		},
		{
			Name:      "Service",
			Reconcile: r.reconcileService,
		},
		// {
		// 	Name:      "StatefulSet",
		// 	Reconcile: r.reconcileStatefulSet,
		// },
	}

	// Fetch DoltCluster StatefulSets
	var statefulSet appsv1.StatefulSet
	if err := r.Get(ctx, req.NamespacedName, &statefulSet); err != nil {
		log.Error(err, "unable to find StatefulSets")
		return ctrl.Result{}, err
	}

	// Fetch all pods in part of statefulset
	var pods corev1.PodList
	if err := r.List(ctx, &pods, client.InNamespace(req.Namespace), client.MatchingFields{"managedBy": req.Name}); err != nil {
		log.Error(err, "unable to list pods")
		return ctrl.Result{}, err
	}

	// Find primary and replicas
	// Check status
	// Primary failed? Promote replica
	// Replica failed? Perform a rollout restart replica

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *DoltClusterReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&doltv1alpha.DoltCluster{}).
		Complete(r)
}

func (r *DoltClusterReconciler) reconcileConfigMap(ctx context.Context, doltcluster *doltv1alpha.DoltCluster) (ctrl.Result, error) {
	defaultConfigMapKeyRef := doltcluster.DefaultConfigMapKey()

	config := config.GenerateConfigMapData(doltcluster)

	req := controller.ConfigMapReconcileRequest{
		Metadata: doltcluster,
		Owner:    doltcluster,
		Key:      defaultConfigMapKeyRef,
		Data:     config,
	}
	if err := r.configMapReconciler.Reconcile(ctx, &req); err != nil {
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

func (r *DoltClusterReconciler) reconcileService(ctx context.Context, doltcluster *doltv1alpha.DoltCluster) (ctrl.Result, error) {
	if err := r.reconcileInternalService(ctx, doltcluster); err != nil {
		return ctrl.Result{}, fmt.Errorf("error reconciling internal Service: %v", err)
	}
	if err := r.reconcilePrimarylService(ctx, doltcluster); err != nil {
		return ctrl.Result{}, fmt.Errorf("error reconciling primary Service: %v", err)
	}
	if err := r.reconcileReaderService(ctx, doltcluster); err != nil {
		return ctrl.Result{}, fmt.Errorf("error reconciling reader Service: %v", err)
	}
	return ctrl.Result{}, nil
}

func (r *DoltClusterReconciler) reconcileInternalService(ctx context.Context, doltcluster *doltv1alpha.DoltCluster) error {
	internalHeadlessSvc, err := r.builder.BuildDoltInternalService(doltcluster)
	if err != nil {
		return fmt.Errorf("error building internal Service: %v", err)
	}

	return r.serviceReconciler.Reconcile(ctx, internalHeadlessSvc)
}

func (r *DoltClusterReconciler) reconcilePrimarylService(ctx context.Context, doltcluster *doltv1alpha.DoltCluster) error {
	primarySvc, err := r.builder.BuildDoltPrimaryService(doltcluster)
	if err != nil {
		return fmt.Errorf("error building primary Service: %v", err)
	}

	return r.serviceReconciler.Reconcile(ctx, primarySvc)
}

func (r *DoltClusterReconciler) reconcileReaderService(ctx context.Context, doltcluster *doltv1alpha.DoltCluster) error {
	primarySvc, err := r.builder.BuildDoltReaderService(doltcluster)
	if err != nil {
		return fmt.Errorf("error building reader Service: %v", err)
	}

	return r.serviceReconciler.Reconcile(ctx, primarySvc)
}

func (r *DoltClusterReconciler) reconcileStatefulSet(ctx context.Context, doltcluster *doltv1alpha.DoltCluster) (ctrl.Result, error) {
	key := client.ObjectKeyFromObject(doltcluster)

	desiredSts, err := r.builder.BuildDoltStatefulSet(key, doltcluster)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("error building StatefulSet: %v", err)
	}

	if err := r.StatefulSetReconciler.ReconcileWithUpdates(ctx, desiredSts, shouldUpdate); err != nil {
		return ctrl.Result{}, fmt.Errorf("error reconciling StatefulSet: %v", err)
	}

	if result, err := r.reconcileUpdates(ctx, doltcluster); !result.IsZero() || err != nil {
		return result, err
	}
	return ctrl.Result{}, nil
}
