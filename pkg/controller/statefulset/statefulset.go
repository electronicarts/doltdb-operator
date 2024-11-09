package statefulset

import (
	"context"
	"fmt"

	doltv1alpha "github.com/electronicarts/doltdb-operator/api/v1alpha"
	appsv1 "k8s.io/api/apps/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// Reconcile ensures that the desired StatefulSet is present in the cluster.
// If the StatefulSet does not exist, it will be created. If it exists, it will be updated if shouldUpdate is true.
func (r *Reconciler) Reconcile(ctx context.Context, doltdb *doltv1alpha.DoltDB) (ctrl.Result, error) {
	return r.reconcileStatefulSet(ctx, doltdb)
}

// ReconcileWithUpdates ensures that the desired StatefulSet is present in the cluster.
// If the StatefulSet does not exist, it will be created. If it exists, it will be updated based on the shouldUpdate flag.
func (r *Reconciler) ReconcileWithUpdates(ctx context.Context, desiredSts *appsv1.StatefulSet) error {
	key := client.ObjectKeyFromObject(desiredSts)
	var existingSts appsv1.StatefulSet
	if err := r.Get(ctx, key, &existingSts); err != nil {
		if !apierrors.IsNotFound(err) {
			return fmt.Errorf("error getting StatefulSet: %v", err)
		}

		if err := r.Create(ctx, desiredSts); err != nil {
			return fmt.Errorf("error creating StatefulSet: %v", err)
		}
		return nil
	}

	// NOTE: should we implement logic to only patch depending on UpdateStrategy?
	patch := client.MergeFrom(existingSts.DeepCopy())
	existingSts.Spec.Template = desiredSts.Spec.Template
	existingSts.Spec.UpdateStrategy = desiredSts.Spec.UpdateStrategy
	existingSts.Spec.Replicas = desiredSts.Spec.Replicas
	return r.Patch(ctx, &existingSts, patch)
}

func (r *Reconciler) reconcileStatefulSet(ctx context.Context, doltdb *doltv1alpha.DoltDB) (ctrl.Result, error) {
	key := client.ObjectKeyFromObject(doltdb)

	desiredSts, err := r.builder.BuildDoltStatefulSet(key, doltdb)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("error building StatefulSet: %v", err)
	}

	if err := r.ReconcileWithUpdates(ctx, desiredSts); err != nil {
		return ctrl.Result{}, fmt.Errorf("error reconciling StatefulSet: %v", err)
	}

	if result, err := r.reconcileUpdates(ctx, doltdb); !result.IsZero() || err != nil {
		return result, err
	}
	return ctrl.Result{}, nil
}
