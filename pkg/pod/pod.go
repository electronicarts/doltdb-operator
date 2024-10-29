package pod

import (
	corev1 "k8s.io/api/core/v1"
)

// PodReadyCondition returns the PodReady condition from the pod's status conditions.
func PodReadyCondition(pod *corev1.Pod) *corev1.PodCondition {
	for _, c := range pod.Status.Conditions {
		if c.Type == corev1.PodReady {
			return &c
		}
	}
	return nil
}

// PodReady checks if the pod is in the PodReady condition.
func PodReady(pod *corev1.Pod) bool {
	if c := PodReadyCondition(pod); c != nil {
		return c.Status == corev1.ConditionTrue
	}
	return false
}

// PodUpdated checks if the pod's update revision matches the given updateRevision.
func PodUpdated(pod *corev1.Pod, updateRevision string) bool {
	if podUpdateRevision, ok := pod.ObjectMeta.Labels["controller-revision-hash"]; ok {
		return podUpdateRevision == updateRevision
	}
	return false
}
