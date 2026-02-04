// Copyright (c) 2025 Electronic Arts Inc. All rights reserved.

package statefulset

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"time"

	"github.com/go-logr/logr"
	doltv1alpha "github.com/electronicarts/doltdb-operator/api/v1alpha"
	"github.com/electronicarts/doltdb-operator/pkg/builder"
	podpkg "github.com/electronicarts/doltdb-operator/pkg/pod"
	"github.com/electronicarts/doltdb-operator/pkg/wait"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	klabels "k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

func shouldReconcileUpdates(doltdb *doltv1alpha.DoltDB) bool {
	// Skip updates if DoltDB is in a transitional state
	if doltdb.IsResizingStorage() || doltdb.IsSwitchingPrimary() {
		return false
	}
	// Only automatically reconcile updates for ReplicasFirstPrimaryLast strategy
	// - RollingUpdate: Kubernetes handles updates natively (no operator intervention)
	// - OnDelete: User must manually delete pods to trigger updates
	// - Never: No automatic updates at all
	return doltdb.Spec.UpdateStrategy == "" ||
		doltdb.Spec.UpdateStrategy == doltv1alpha.ReplicasFirstPrimaryLastUpdateType
}

func (r *Reconciler) reconcileUpdates(ctx context.Context, doltdb *doltv1alpha.DoltDB) (ctrl.Result, error) {
	if !shouldReconcileUpdates(doltdb) {
		return ctrl.Result{}, nil
	}
	doltdbKey := client.ObjectKeyFromObject(doltdb)
	logger := log.FromContext(ctx).WithName("update")

	stsUpdateRevision, err := GetRevision(ctx, r.Client, doltdb)
	if err != nil {
		return ctrl.Result{}, err
	}
	if stsUpdateRevision == "" {
		logger.V(1).Info("StatefulSet status.updateRevision not set. Requeuing...")
		return ctrl.Result{RequeueAfter: 1 * time.Second}, nil
	}

	var podsByRole podRoleSet
	if result, err := r.getPodsByRole(ctx, doltdb, &podsByRole, logger); !result.IsZero() || err != nil {
		return result, err
	}

	stalePodNames := podsByRole.getStalePodNames(stsUpdateRevision)
	if len(stalePodNames) == 0 {
		return ctrl.Result{}, nil
	}
	logger.V(1).Info("Detected stale Pods that need updating", "pods", stalePodNames)

	if result, err := r.waitForReadyStatus(ctx, doltdb, logger); !result.IsZero() || err != nil {
		return result, err
	}

	for _, replicaPod := range podsByRole.replicas {
		if podpkg.PodUpdated(&replicaPod, stsUpdateRevision) {
			logger.V(1).Info("Replica Pod up to date", "pod", replicaPod.Name)
			continue
		}
		logger.Info("Updating replica Pod", "pod", replicaPod.Name)
		if err := r.updatePod(ctx, doltdbKey, &replicaPod, stsUpdateRevision, logger); err != nil {
			return ctrl.Result{}, fmt.Errorf("error updating replica Pod '%s': %v", replicaPod.Name, err)
		}

		return ctrl.Result{Requeue: true}, nil
	}

	if result, err := r.waitForConfiguredReplication(doltdb, logger); !result.IsZero() || err != nil {
		return result, err
	}

	primaryPod := podsByRole.primary
	if podpkg.PodUpdated(&primaryPod, stsUpdateRevision) {
		logger.V(1).Info("Primary Pod up to date", "pod", primaryPod.Name)
		return ctrl.Result{}, nil
	}

	logger.Info("Updating primary Pod", "pod", primaryPod.Name)
	if err := r.updatePod(ctx, doltdbKey, &primaryPod, stsUpdateRevision, logger); err != nil {
		return ctrl.Result{}, fmt.Errorf("error updating primary Pod '%s': %v", primaryPod.Name, err)
	}
	return ctrl.Result{}, nil
}

func (r *Reconciler) waitForReadyStatus(ctx context.Context, doltdb *doltv1alpha.DoltDB, logger logr.Logger) (ctrl.Result, error) {
	var sts appsv1.StatefulSet
	if err := r.Get(ctx, client.ObjectKeyFromObject(doltdb), &sts); err != nil {
		return ctrl.Result{}, err
	}
	if sts.Status.ReadyReplicas != doltdb.Spec.Replicas {
		logger.V(1).Info("Waiting for all Pods to be ready to proceed with the update. Requeuing...")
		return ctrl.Result{RequeueAfter: 1 * time.Second}, nil
	}

	return ctrl.Result{}, nil
}

func (r *Reconciler) waitForConfiguredReplication(doltdb *doltv1alpha.DoltDB, logger logr.Logger) (ctrl.Result, error) {
	if !doltdb.Replication().Enabled {
		return ctrl.Result{}, nil
	}

	if !doltdb.IsReplicationConfigured() {
		logger.V(1).Info("Waiting for Pods to have configured replication.")
		return ctrl.Result{}, ErrSkipReconciliationPhase
	}
	logger.V(1).Info("Pods have configured replication.")

	return ctrl.Result{}, nil
}

func (r *Reconciler) updatePod(ctx context.Context, doltdbKey types.NamespacedName, pod *corev1.Pod, updateRevision string,
	logger logr.Logger) error {
	if err := r.Delete(ctx, pod); err != nil {
		return fmt.Errorf("error deleting Pod '%s': %v", pod.Name, err)
	}

	updateCtx, cancel := context.WithTimeout(ctx, 1*time.Minute)
	defer cancel()
	if err := r.pollUntilPodUpdated(updateCtx, doltdbKey, client.ObjectKeyFromObject(pod), updateRevision, logger); err != nil {
		return fmt.Errorf("error waiting for Pod '%s' to be updated: %v", pod.Name, err)
	}
	return nil
}

func (r *Reconciler) pollUntilPodUpdated(ctx context.Context, doltdbKey, podKey types.NamespacedName, updateRevision string,
	logger logr.Logger) error {
	return wait.PollWithDoltDB(ctx, doltdbKey, r.Client, logger, func(ctx context.Context) error {
		var pod corev1.Pod
		if err := r.Get(ctx, podKey, &pod); err != nil {
			return fmt.Errorf("error getting Pod '%s': %v", podKey.Name, err)
		}
		if podpkg.PodUpdated(&pod, updateRevision) {
			return nil
		}
		return errors.New("pod is stale")
	})
}

type podRoleSet struct {
	replicas []corev1.Pod
	primary  corev1.Pod
}

func (p *podRoleSet) getStalePodNames(updateRevision string) []string {
	var podNames []string
	for _, r := range p.replicas {
		if !podpkg.PodUpdated(&r, updateRevision) {
			podNames = append(podNames, r.Name)
		}
	}
	if !podpkg.PodUpdated(&p.primary, updateRevision) {
		podNames = append(podNames, p.primary.Name)
	}
	return podNames
}

func (r *Reconciler) getPodsByRole(ctx context.Context, doltdb *doltv1alpha.DoltDB, podsByRole *podRoleSet,
	logger logr.Logger) (ctrl.Result, error) {
	currentPrimary := ptr.Deref(doltdb.Status.CurrentPrimary, "")
	if currentPrimary == "" {
		logger.V(1).Info("DoltDB status.currentPrimary not set. Requeuing...")
		return ctrl.Result{RequeueAfter: 1 * time.Second}, nil
	}
	if doltdb.Spec.Replicas == 0 {
		logger.V(1).Info("DoltDB is downscaled. Requeuing...")
		return ctrl.Result{RequeueAfter: 5 * time.Second}, nil
	}

	list := corev1.PodList{}
	listOpts := &client.ListOptions{
		LabelSelector: klabels.SelectorFromSet(
			builder.NewLabelsBuilder().
				WithDoltSelectorLabels(doltdb).
				Build(),
		),
		Namespace: doltdb.GetNamespace(),
	}
	if err := r.List(ctx, &list, listOpts); err != nil {
		return ctrl.Result{}, fmt.Errorf("error listing Pods: %v", err)
	}

	numPods := len(list.Items)
	numReplicas := int(doltdb.Spec.Replicas)
	if len(list.Items) != int(doltdb.Spec.Replicas) {
		logger.V(1).Info("Number of Pods does not match DoltDB replicas. Requeuing...", "pods", numPods, "doltdb-replicas", numReplicas)
		// TODO: revisit time
		return ctrl.Result{RequeueAfter: 1 * time.Second}, nil
	}

	var replicas []corev1.Pod
	var primary *corev1.Pod
	for _, pod := range list.Items {
		if pod.Name == currentPrimary {
			primary = &pod
		} else {
			replicas = append(replicas, pod)
		}
	}
	if len(replicas) == 0 {
		return ctrl.Result{}, errors.New("no replica Pods found")
	}
	if primary == nil {
		return ctrl.Result{}, errors.New("primary Pod not found")
	}
	sort.Slice(replicas, func(i, j int) bool {
		return replicas[i].Name > replicas[j].Name
	})

	if podsByRole == nil {
		podsByRole = &podRoleSet{}
	}
	podsByRole.replicas = replicas
	podsByRole.primary = *primary

	return ctrl.Result{}, nil
}
