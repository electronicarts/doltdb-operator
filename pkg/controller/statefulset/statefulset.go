// Copyright (c) 2025 Electronic Arts Inc. All rights reserved.

package statefulset

import (
	"context"
	"fmt"

	doltv1alpha "github.com/electronicarts/doltdb-operator/api/v1alpha"
	"github.com/electronicarts/doltdb-operator/pkg/builder"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
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

// getConfigMapHash fetches the ConfigMap for the DoltDB and computes its content hash.
// This hash is used in pod template annotations to trigger pod restarts when ConfigMap changes.
func (r *Reconciler) getConfigMapHash(ctx context.Context, doltdb *doltv1alpha.DoltDB) (string, error) {
	var configMap corev1.ConfigMap
	configMapKey := doltdb.DefaultConfigMapKey()
	if err := r.Get(ctx, configMapKey, &configMap); err != nil {
		if apierrors.IsNotFound(err) {
			// ConfigMap not yet created, return empty hash
			return "", nil
		}
		return "", fmt.Errorf("error getting ConfigMap: %v", err)
	}
	return builder.HashConfigMapData(configMap.Data), nil
}

func (r *Reconciler) reconcileStatefulSet(ctx context.Context, doltdb *doltv1alpha.DoltDB) (ctrl.Result, error) {
	key := client.ObjectKeyFromObject(doltdb)

	// Get the ConfigMap hash to include in pod template annotations
	configMapHash, err := r.getConfigMapHash(ctx, doltdb)
	if err != nil {
		return ctrl.Result{}, err
	}

	desiredSts, err := r.builder.BuildDoltStatefulSet(key, doltdb, configMapHash)
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
