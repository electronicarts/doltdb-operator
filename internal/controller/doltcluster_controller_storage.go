package controller

import (
	"context"
	"errors"
	"fmt"
	"time"

	doltv1alpha "github.com/electronicarts/doltdb-operator/api/v1alpha"
	"github.com/electronicarts/doltdb-operator/pkg/builder"
	"github.com/electronicarts/doltdb-operator/pkg/conditions"
	"github.com/electronicarts/doltdb-operator/pkg/pvc"
	stsobj "github.com/electronicarts/doltdb-operator/pkg/statefulset"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	klabels "k8s.io/apimachinery/pkg/labels"
	"k8s.io/utils/ptr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

func shouldReconcileStorage(doltdb *doltv1alpha.DoltCluster) bool {
	if doltdb.IsUpdating() ||
		doltdb.IsSwitchingPrimary() {
		return false
	}
	return true
}

func (r *DoltDBReconciler) reconcileStorage(ctx context.Context, doltdb *doltv1alpha.DoltCluster) (ctrl.Result, error) {
	if !shouldReconcileStorage(doltdb) {
		return ctrl.Result{}, nil
	}
	if doltdb.IsWaitingForStorageResize() {
		return r.waitForStorageResize(ctx, doltdb)
	}

	key := client.ObjectKeyFromObject(doltdb)
	var existingSts appsv1.StatefulSet
	if err := r.Get(ctx, key, &existingSts); err != nil {
		return ctrl.Result{}, err
	}

	existingSize := stsobj.GetStorageSize(&existingSts, builder.DoltDataVolume)
	desiredSize := doltdb.Spec.Storage.Size
	if existingSize == nil {
		return ctrl.Result{}, errors.New("invalid existing storage size")
	}
	if desiredSize == nil {
		return ctrl.Result{}, errors.New("invalid desired storage size")
	}
	sizeCmp := desiredSize.Cmp(*existingSize)
	if sizeCmp == 0 {
		return ctrl.Result{}, nil
	}
	if sizeCmp < 0 {
		return ctrl.Result{}, fmt.Errorf("cannot decrease storage size from '%s' to '%s'", existingSize, desiredSize)
	}

	if err := r.patchStatus(ctx, doltdb, func(status *doltv1alpha.DoltClusterStatus) error {
		conditions.SetReadyStorageResizing(status)
		return nil
	}); err != nil {
		return ctrl.Result{}, fmt.Errorf("error patching status: %v", err)
	}

	if result, err := r.resizeInUsePVCs(ctx, doltdb, *desiredSize); !result.IsZero() || err != nil {
		return result, err
	}
	if result, err := r.resizeStatefulSet(ctx, doltdb, &existingSts); !result.IsZero() || err != nil {
		return result, err
	}

	if err := r.patchStatus(ctx, doltdb, func(status *doltv1alpha.DoltClusterStatus) error {
		conditions.SetReadyWaitingStorageResize(status)
		return nil
	}); err != nil {
		return ctrl.Result{}, fmt.Errorf("error patching status: %v", err)
	}

	return r.waitForStorageResize(ctx, doltdb)
}

func (r *DoltDBReconciler) resizeInUsePVCs(ctx context.Context, doltdb *doltv1alpha.DoltCluster,
	size resource.Quantity) (ctrl.Result, error) {
	if !ptr.Deref(doltdb.Spec.Storage.ResizeInUseVolumes, true) {
		return ctrl.Result{}, nil
	}

	pvcs, err := r.getStoragePVCs(ctx, doltdb)
	if err != nil {
		return ctrl.Result{}, err
	}
	for _, pvc := range pvcs {
		patch := client.MergeFrom(pvc.DeepCopy())
		pvc.Spec.Resources.Requests[corev1.ResourceStorage] = size
		if err := r.Patch(ctx, &pvc, patch); err != nil {
			return ctrl.Result{}, fmt.Errorf("error patching PVC '%s': %v", pvc.Name, err)
		}
	}
	return ctrl.Result{}, nil
}

func (r *DoltDBReconciler) resizeStatefulSet(ctx context.Context, doltdb *doltv1alpha.DoltCluster,
	sts *appsv1.StatefulSet) (ctrl.Result, error) {
	if err := r.Delete(ctx, sts, &client.DeleteOptions{PropagationPolicy: ptr.To(metav1.DeletePropagationOrphan)}); err != nil {
		return ctrl.Result{}, fmt.Errorf("error deleting StatefulSet: %v", err)
	}
	return r.reconcileStatefulSet(ctx, doltdb)
}

func (r *DoltDBReconciler) waitForStorageResize(ctx context.Context, doltdb *doltv1alpha.DoltCluster) (ctrl.Result, error) {
	logger := log.FromContext(ctx)
	logger.V(1).Info("Waiting for storage resize")

	if ptr.Deref(doltdb.Spec.Storage.ResizeInUseVolumes, true) && ptr.Deref(doltdb.Spec.Storage.WaitForVolumeResize, true) {
		pvcs, err := r.getStoragePVCs(ctx, doltdb)
		if err != nil {
			return ctrl.Result{}, err
		}
		for _, p := range pvcs {
			if pvc.IsResizing(&p) {
				logger.V(1).Info("Waiting for PVC resize", "pvc", p.Name)
				return ctrl.Result{RequeueAfter: 1 * time.Second}, nil
			}
		}
	}

	key := client.ObjectKeyFromObject(doltdb)
	var sts appsv1.StatefulSet
	if err := r.Get(ctx, key, &sts); err != nil {
		return ctrl.Result{}, err
	}
	if sts.Status.ReadyReplicas != doltdb.Spec.Replicas {
		logger.V(1).Info(
			"Waiting for StatefulSet ready",
			"ready-replicas", sts.Status.ReadyReplicas,
			"expected-replicas", doltdb.Spec.Replicas,
		)
		return ctrl.Result{RequeueAfter: 1 * time.Second}, nil
	}

	if err := r.patchStatus(ctx, doltdb, func(status *doltv1alpha.DoltClusterStatus) error {
		conditions.SetReadyStorageResized(status)
		return nil
	}); err != nil {
		return ctrl.Result{}, fmt.Errorf("error patching status: %v", err)
	}
	return ctrl.Result{}, nil
}

func (r *DoltDBReconciler) getStoragePVCs(ctx context.Context, doltdb *doltv1alpha.DoltCluster) ([]corev1.PersistentVolumeClaim, error) {
	pvcList := corev1.PersistentVolumeClaimList{}
	listOpts := client.ListOptions{
		LabelSelector: klabels.SelectorFromSet(
			builder.NewLabelsBuilder().
				WithDoltSelectorLabels(doltdb).
				Build(),
		),
		Namespace: doltdb.GetNamespace(),
	}
	if err := r.List(ctx, &pvcList, &listOpts); err != nil {
		return nil, fmt.Errorf("error listing PVCs: %v", err)
	}
	return pvcList.Items, nil
}
