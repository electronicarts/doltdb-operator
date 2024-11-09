package controller

import (
	"context"
	"errors"

	corev1 "k8s.io/api/core/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/electronicarts/doltdb-operator/pkg/controller/replication"
	doltpod "github.com/electronicarts/doltdb-operator/pkg/pod"
	"github.com/electronicarts/doltdb-operator/pkg/predicate"
	"github.com/electronicarts/doltdb-operator/pkg/refresolver"
)

// PodController reconciles a Pod object
type PodController struct {
	client.Client
	name               string
	refResolver        *refresolver.RefResolver
	failoverController *replication.PodReadinessController
	podAnnotations     []string
}

// NewPodController creates a new PodController
func NewPodController(name string, client client.Client, refResolver *refresolver.RefResolver,
	failoverController *replication.PodReadinessController, podAnnotations []string) *PodController {
	return &PodController{
		Client:             client,
		name:               name,
		refResolver:        refResolver,
		failoverController: failoverController,
		podAnnotations:     podAnnotations,
	}
}

// +kubebuilder:rbac:groups="",resources=pods,verbs=get;list;watch

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// Reconcile reconciles a Pod resource to ensure it is in the desired state.
// It fetches the Pod object, resolves the associated DoltDB from annotations,
// and handles the Pod's readiness state.
func (r *PodController) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	var pod corev1.Pod
	if err := r.Get(ctx, req.NamespacedName, &pod); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	log.FromContext(ctx).Info("Reconciling Pod", "pod", pod.Name)

	doltdb, err := r.refResolver.DoltDBFromAnnotation(ctx, pod.ObjectMeta)
	if err != nil {
		if errors.Is(err, refresolver.ErrDoltClusterAnnotationNotFound) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	if !doltpod.PodReady(&pod) {
		if err := r.failoverController.ReconcilePodNotReady(ctx, pod, doltdb); err != nil {
			log.FromContext(ctx).V(1).Info("Error reconciling Pod in non Ready state", "pod", pod.Name)
			return ctrl.Result{Requeue: true}, nil
		}
	}
	return ctrl.Result{}, nil
}

func (r *PodController) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		Named(r.name).
		For(&corev1.Pod{}).
		WithEventFilter(
			predicate.PredicateChangedWithAnnotations(
				r.podAnnotations,
				podHasChanged,
			),
		).
		Complete(r)
}

func podHasChanged(old, new client.Object) bool {
	oldPod, ok := old.(*corev1.Pod)
	if !ok {
		return false
	}
	newPod, ok := new.(*corev1.Pod)
	if !ok {
		return false
	}
	return doltpod.PodReady(oldPod) != doltpod.PodReady(newPod)
}
