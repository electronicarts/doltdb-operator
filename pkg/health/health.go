// Copyright (c) 2025 Electronic Arts Inc. All rights reserved.

package health

import (
	"context"
	"errors"
	"fmt"
	"sort"

	doltv1alpha "github.com/electronicarts/doltdb-operator/api/v1alpha"
	"github.com/electronicarts/doltdb-operator/pkg/builder"
	"github.com/electronicarts/doltdb-operator/pkg/pod"
	"github.com/electronicarts/doltdb-operator/pkg/statefulset"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	discoveryv1 "k8s.io/api/discovery/v1"
	klabels "k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"
)

// IsStatefulSetHealthy checks if the StatefulSet specified by the key is healthy based on the provided health options.
func IsStatefulSetHealthy(
	ctx context.Context,
	client ctrlclient.Client,
	key types.NamespacedName,
	serviceKey types.NamespacedName,
	opts ...HealthOpt) (bool, error) {
	var sts appsv1.StatefulSet
	if err := client.Get(ctx, key, &sts); err != nil {
		return false, ctrlclient.IgnoreNotFound(err)
	}

	healthOpts := HealthOpts{
		DesiredReplicas: ptr.Deref(sts.Spec.Replicas, 1),
		EndpointPolicy:  ptr.To(EndpointPolicyAll),
	}
	for _, setOpt := range opts {
		setOpt(&healthOpts)
	}

	if sts.Status.ReadyReplicas != healthOpts.DesiredReplicas {
		return false, nil
	}
	if healthOpts.Port == nil || healthOpts.EndpointPolicy == nil {
		return true, nil
	}

	var sliceList discoveryv1.EndpointSliceList
	if err := client.List(ctx, &sliceList,
		ctrlclient.InNamespace(serviceKey.Namespace),
		ctrlclient.MatchingLabels{discoveryv1.LabelServiceName: serviceKey.Name},
	); err != nil {
		return false, err
	}
	if len(sliceList.Items) == 0 {
		return false, nil
	}

	readyCount := countReadyEndpoints(sliceList.Items, *healthOpts.Port)
	switch *healthOpts.EndpointPolicy {
	case EndpointPolicyAll:
		return readyCount == int(healthOpts.DesiredReplicas), nil
	case EndpointPolicyAtLeastOne:
		return readyCount > 0, nil
	default:
		return false, fmt.Errorf("unsupported EndpointPolicy '%v'", *healthOpts.EndpointPolicy)
	}
}

// HealthyDoltDBReplica returns the index of a healthy DoltDB replica that is not the current primary pod.
func HealthyDoltDBReplica(ctx context.Context, client ctrlclient.Client, doltdb *doltv1alpha.DoltDB) (*int, error) {
	if doltdb.Status.CurrentPrimaryPodIndex == nil {
		return nil, errors.New("'status.currentPrimaryPodIndex' must be set")
	}
	podList := corev1.PodList{}
	listOpts := &ctrlclient.ListOptions{
		LabelSelector: klabels.SelectorFromSet(
			builder.NewLabelsBuilder().
				WithDoltSelectorLabels(doltdb).
				Build(),
		),
		Namespace: doltdb.GetNamespace(),
	}

	if err := client.List(ctx, &podList, listOpts); err != nil {
		return nil, fmt.Errorf("error listing Pods: %v", err)
	}
	sortPodList(podList)

	for _, p := range podList.Items {
		index, err := statefulset.PodIndex(p.Name)
		if err != nil {
			return nil, fmt.Errorf("error getting index for Pod '%s': %v", p.Name, err)
		}
		if *index == *doltdb.Status.CurrentPrimaryPodIndex {
			continue
		}
		if pod.PodReady(&p) {
			return index, nil
		}
	}
	return nil, ErrNoHealthyInstancesAvailable
}

// IsDoltDBReplicaHealthy checks if the DoltDB replica specified by the podIndex is healthy.
func IsDoltDBReplicaHealthy(
	ctx context.Context,
	client ctrlclient.Client,
	doltdb *doltv1alpha.DoltDB,
	podIndex int,
) (*corev1.Pod, bool, error) {
	podName := statefulset.PodName(doltdb.ObjectMeta, podIndex)
	key := types.NamespacedName{
		Name:      podName,
		Namespace: doltdb.Namespace,
	}
	var doltPod corev1.Pod
	if err := client.Get(ctx, key, &doltPod); err != nil {
		return nil, false, err
	}
	return &doltPod, pod.PodReady(&doltPod), nil
}

// HealthyDoltDBStandbys returns a list of healthy DoltDB standbys that are not the current primary pod.
func HealthyDoltDBStandbys(ctx context.Context, client ctrlclient.Client, doltdb *doltv1alpha.DoltDB) ([]corev1.Pod, error) {
	podList := corev1.PodList{}
	listOpts := &ctrlclient.ListOptions{
		LabelSelector: klabels.SelectorFromSet(
			builder.NewLabelsBuilder().
				WithDoltSelectorLabels(doltdb).
				Build(),
		),
		Namespace: doltdb.GetNamespace(),
	}

	if err := client.List(ctx, &podList, listOpts); err != nil {
		return nil, fmt.Errorf("error listing Pods: %v", err)
	}
	sortPodList(podList)

	pods := make([]corev1.Pod, 0, len(podList.Items))
	for _, p := range podList.Items {
		index, err := statefulset.PodIndex(p.Name)
		if err != nil {
			return nil, fmt.Errorf("error getting index for Pod '%s': %v", p.Name, err)
		}
		if *index == *doltdb.Status.CurrentPrimaryPodIndex {
			continue
		}
		if pod.PodReady(&p) {
			pods = append(pods, p)
		}
	}

	return pods, nil
}

// IsServiceHealthy checks if the service specified by the serviceKey has healthy endpoints.
func IsServiceHealthy(ctx context.Context, client ctrlclient.Client, serviceKey types.NamespacedName) (bool, error) {
	var sliceList discoveryv1.EndpointSliceList
	if err := client.List(ctx, &sliceList,
		ctrlclient.InNamespace(serviceKey.Namespace),
		ctrlclient.MatchingLabels{discoveryv1.LabelServiceName: serviceKey.Name},
	); err != nil {
		return false, err
	}
	if len(sliceList.Items) == 0 {
		return false, fmt.Errorf("'%s/%s' endpoints not ready", serviceKey.Name, serviceKey.Namespace)
	}
	for _, slice := range sliceList.Items {
		for _, ep := range slice.Endpoints {
			if ep.Conditions.Ready != nil && *ep.Conditions.Ready {
				return true, nil
			}
		}
	}
	return false, fmt.Errorf("'%s/%s' no ready endpoints", serviceKey.Name, serviceKey.Namespace)
}

// StandbyHostFQDNs returns the FQDNs of healthy standby pods for use in graceful transitions.
func StandbyHostFQDNs(ctx context.Context, client ctrlclient.Client, doltdb *doltv1alpha.DoltDB) ([]string, error) {
	healthyStandbys, err := HealthyDoltDBStandbys(ctx, client, doltdb)
	if err != nil {
		return nil, fmt.Errorf("error getting healthy standbys: %v", err)
	}
	if len(healthyStandbys) == 0 {
		return nil, fmt.Errorf("no healthy standbys available")
	}

	hosts := make([]string, len(healthyStandbys))
	for i, standby := range healthyStandbys {
		podIndex, err := statefulset.PodIndex(standby.Name)
		if err != nil {
			return nil, fmt.Errorf("error getting index for Pod '%s': %v", standby.Name, err)
		}
		hosts[i] = statefulset.PodShortFQDNWithServiceAndNamespace(
			doltdb.ObjectMeta, *podIndex, doltdb.InternalServiceKey().Name)
	}
	return hosts, nil
}

// countReadyEndpoints counts the number of ready endpoints across all EndpointSlices
// that expose the given port.
func countReadyEndpoints(slices []discoveryv1.EndpointSlice, port int32) int {
	count := 0
	for _, slice := range slices {
		if !sliceHasPort(slice, port) {
			continue
		}
		for _, ep := range slice.Endpoints {
			if ep.Conditions.Ready != nil && *ep.Conditions.Ready {
				count++
			}
		}
	}
	return count
}

// sliceHasPort returns true if the EndpointSlice exposes the given port.
func sliceHasPort(slice discoveryv1.EndpointSlice, port int32) bool {
	for _, p := range slice.Ports {
		if p.Port != nil && *p.Port == port {
			return true
		}
	}
	return false
}

// sortPodList sorts the given PodList by pod name.
func sortPodList(list corev1.PodList) {
	sort.Slice(list.Items, func(i, j int) bool {
		return list.Items[i].Name < list.Items[j].Name
	})
}
