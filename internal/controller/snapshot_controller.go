// Copyright (c) 2025 Electronic Arts Inc. All rights reserved.

package controller

import (
	"context"
	"fmt"

	doltv1alpha "github.com/electronicarts/doltdb-operator/api/v1alpha"
	"github.com/electronicarts/doltdb-operator/pkg/builder"
	"github.com/electronicarts/doltdb-operator/pkg/conditions"
	"github.com/electronicarts/doltdb-operator/pkg/controller/database"
	"github.com/electronicarts/doltdb-operator/pkg/controller/volumesnapshot"
	"github.com/electronicarts/doltdb-operator/pkg/patch"
	"github.com/electronicarts/doltdb-operator/pkg/refresolver"
	"github.com/hashicorp/go-multierror"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

// SnapshotReconciler reconciles a Snapshot object
type SnapshotReconciler struct {
	client.Client
	Scheme                   *runtime.Scheme
	Builder                  *builder.Builder
	RefResolver              *refresolver.RefResolver
	VolumeSnapshotReconciler *volumesnapshot.Reconciler
	ConditionReady           *conditions.Ready
}

// +kubebuilder:rbac:groups=k8s.dolthub.com,resources=snapshots,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=k8s.dolthub.com,resources=snapshots/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=snapshot.storage.k8s.io,resources=volumesnapshots,verbs=get;list;watch;create;patch;update;delete
// +kubebuilder:rbac:groups="",resources=configmaps,verbs=get;list;watch;create;patch;delete
// +kubebuilder:rbac:groups=batch,resources=cronjobs,verbs=get;list;create;update;delete;patch;watch
// +kubebuilder:rbac:groups=batch,resources=cronjobs/status,verbs=get;update
// +kubebuilder:rbac:groups=batch,resources=cronjobs/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// the Snapshot object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
func (r *SnapshotReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := log.FromContext(ctx).WithValues("snapshot", req.NamespacedName)

	// Fetch Snapshot CR in current namespace
	var snapshot doltv1alpha.Snapshot
	if err := r.Get(ctx, req.NamespacedName, &snapshot); err != nil {
		log.Error(err, "unable to fetch Snapshot")
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}
	log.WithValues("snapshot", snapshot.Name).
		Info("Running reconciler for Snapshot")

	if err := patch.PatchSnapshotStatus(ctx, r.Client, &snapshot, func(status *doltv1alpha.SnapshotStatus) error {
		conditions.SetReadyWithSnapshotJobCreated(status)
		return nil
	}); err != nil {
		return ctrl.Result{Requeue: true}, fmt.Errorf("error patching status: %v", err)
	}

	// Fetch DoltDB CR in current namespace
	doltdb, err := r.RefResolver.DoltDB(ctx, snapshot.DoltDBRef(), snapshot.GetNamespace())
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("error getting DoltDB: %v", err)
	}
	if result, err := database.WaitForDoltDB(ctx, r.Client, doltdb, false); !result.IsZero() || err != nil {
		var errBundle *multierror.Error

		if err != nil {
			errBundle = multierror.Append(errBundle, err)

			err := r.PatchStatus(ctx, &snapshot, r.ConditionReady.PatcherWithError(err))
			errBundle = multierror.Append(errBundle, err)
		}

		return result, fmt.Errorf("error waiting for DoltDB: %v", errBundle.ErrorOrNil())
	}

	// Reconcile the Snapshot
	if err = r.VolumeSnapshotReconciler.Reconcile(ctx, &volumesnapshot.ReconcileRequest{
		Metadata: &snapshot.ObjectMeta,
		Owner:    doltdb,
		SubOwner: &snapshot,
	}); err != nil {
		// Update the Snapshot status
		var errBundle *multierror.Error

		msg := fmt.Sprintf("Error creating snapshot %s: %v", snapshot.GetName(), err)
		err = r.PatchStatus(ctx, &snapshot, r.ConditionReady.PatcherFailed(msg))
		errBundle = multierror.Append(errBundle, err)

		return ctrl.Result{Requeue: true}, errBundle.ErrorOrNil()
	}

	if err := patch.PatchSnapshotStatus(ctx, r.Client, &snapshot, func(status *doltv1alpha.SnapshotStatus) error {
		conditions.SetReadyWithSnapshotJobCreated(status)
		return nil
	}); err != nil {
		return ctrl.Result{Requeue: true}, fmt.Errorf("error patching status: %v", err)
	}

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *SnapshotReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&doltv1alpha.Snapshot{}).Complete(r)
}

func (r *SnapshotReconciler) PatchStatus(ctx context.Context, snapshot *doltv1alpha.Snapshot, patcher conditions.Patcher) error {
	patch := client.MergeFrom(snapshot.DeepCopy())
	patcher(&snapshot.Status)

	if err := r.Client.Status().Patch(ctx, snapshot, patch); err != nil {
		return fmt.Errorf("error patching Snapshot status: %v", err)
	}
	return nil
}
