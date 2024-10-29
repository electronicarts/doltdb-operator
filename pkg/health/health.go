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
	klabels "k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"
)

// IsStatefulSetHealthy checks if the StatefulSet specified by the key is healthy based on the provided health options.
func IsStatefulSetHealthy(ctx context.Context, client ctrlclient.Client, key types.NamespacedName,
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

	var endpoints corev1.Endpoints
	if err := client.Get(ctx, key, &endpoints); err != nil {
		return false, ctrlclient.IgnoreNotFound(err)
	}
	for _, subset := range endpoints.Subsets {
		for _, port := range subset.Ports {
			if port.Port == *healthOpts.Port {
				switch *healthOpts.EndpointPolicy {
				case EndpointPolicyAll:
					return len(subset.Addresses) == int(healthOpts.DesiredReplicas), nil
				case EndpointPolicyAtLeastOne:
					return len(subset.Addresses) > 0, nil
				default:
					return false, fmt.Errorf("unsupported EndpointPolicy '%v'", *healthOpts.EndpointPolicy)
				}
			}
		}
	}
	return false, nil
}

// HealthyDoltDBReplica returns the index of a healthy DoltDB replica that is not the current primary pod.
func HealthyDoltDBReplica(ctx context.Context, client ctrlclient.Client, doltCluster *doltv1alpha.DoltCluster) (*int, error) {
	if doltCluster.Status.CurrentPrimaryPodIndex == nil {
		return nil, errors.New("'status.currentPrimaryPodIndex' must be set")
	}
	podList := corev1.PodList{}
	listOpts := &ctrlclient.ListOptions{
		LabelSelector: klabels.SelectorFromSet(
			builder.NewLabelsBuilder().
				WithDoltSelectorLabels(doltCluster).
				Build(),
		),
		Namespace: doltCluster.GetNamespace(),
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
		if *index == *doltCluster.Status.CurrentPrimaryPodIndex {
			continue
		}
		if pod.PodReady(&p) {
			return index, nil
		}
	}
	return nil, ErrNoHealthyInstancesAvailable
}

// IsServiceHealthy checks if the service specified by the serviceKey has healthy endpoints.
func IsServiceHealthy(ctx context.Context, client ctrlclient.Client, serviceKey types.NamespacedName) (bool, error) {
	var endpoints corev1.Endpoints
	err := client.Get(ctx, serviceKey, &endpoints)
	if err != nil {
		return false, err
	}
	if len(endpoints.Subsets) == 0 {
		return false, fmt.Errorf("'%s/%s' subsets not ready", serviceKey.Name, serviceKey.Namespace)
	}
	if len(endpoints.Subsets[0].Addresses) == 0 {
		return false, fmt.Errorf("'%s/%s' addresses not ready", serviceKey.Name, serviceKey.Namespace)
	}
	return true, nil
}

// sortPodList sorts the given PodList by pod name.
func sortPodList(list corev1.PodList) {
	sort.Slice(list.Items, func(i, j int) bool {
		return list.Items[i].Name < list.Items[j].Name
	})
}
