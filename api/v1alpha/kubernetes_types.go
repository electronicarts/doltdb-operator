package v1alpha

import (
	corev1 "k8s.io/api/core/v1"
)

// Refer to the Kubernetes docs: https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.31/#localobjectreference-v1-core.
type LocalObjectReference struct {
	// +optional
	Name string `json:"name,omitempty"`
}

func (r LocalObjectReference) ToKubernetesType() corev1.LocalObjectReference {
	return corev1.LocalObjectReference{
		Name: r.Name,
	}
}

// Refer to the Kubernetes docs: https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.31/#objectreference-v1-core.
type ObjectReference struct {
	// +optional
	Name string `json:"name,omitempty"`
	// +optional
	Namespace string `json:"namespace,omitempty"`
}

func (r ObjectReference) ToKubernetesType() corev1.ObjectReference {
	return corev1.ObjectReference{
		Name:      r.Name,
		Namespace: r.Namespace,
	}
}

// Refer to the Kubernetes docs: https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.31/#secretkeyselector-v1-core.
// +structType=atomic
type SecretKeySelector struct {
	LocalObjectReference `json:",inline"`
	Key                  string `json:"key"`
}

func (s SecretKeySelector) ToKubernetesType() corev1.SecretKeySelector {
	return corev1.SecretKeySelector{
		LocalObjectReference: s.LocalObjectReference.ToKubernetesType(),
		Key:                  s.Key,
	}
}

// Refer to the Kubernetes docs: https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.31/#configmapkeyselector-v1-core.
// +structType=atomic
type ConfigMapKeySelector struct {
	LocalObjectReference `json:",inline"`
	Key                  string `json:"key"`
}

func (s ConfigMapKeySelector) ToKubernetesType() corev1.ConfigMapKeySelector {
	return corev1.ConfigMapKeySelector{
		LocalObjectReference: s.LocalObjectReference.ToKubernetesType(),
		Key:                  s.Key,
	}
}
