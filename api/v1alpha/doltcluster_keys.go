package v1alpha

import (
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
)

// InternalServiceKey defines the key for the internal headless Service
func (d *DoltCluster) InternalServiceKey() types.NamespacedName {
	return types.NamespacedName{
		Name:      fmt.Sprintf("%s-internal", d.Name),
		Namespace: d.Namespace,
	}
}

// PrimaryServiceKey defines the key for the internal primary instance
func (d *DoltCluster) PrimaryServiceKey() types.NamespacedName {
	return types.NamespacedName{
		Name:      fmt.Sprintf("%s-primary", d.Name),
		Namespace: d.Namespace,
	}
}

// InternalServiceKey defines the key for the internal reader instances
func (d *DoltCluster) ReaderServiceKey() types.NamespacedName {
	return types.NamespacedName{
		Name:      fmt.Sprintf("%s-reader", d.Name),
		Namespace: d.Namespace,
	}
}

// PVCKey defines the PVC keys.
func (m *DoltCluster) PVCKey(name string, index int) types.NamespacedName {
	return types.NamespacedName{
		Name:      fmt.Sprintf("%s-%s-%d", name, m.Name, index),
		Namespace: m.Namespace,
	}
}

// ConfigDataKey defines the config map keys.
func (m *DoltCluster) DefaultConfigMapKey() types.NamespacedName {
	return types.NamespacedName{
		Name:      fmt.Sprintf("%s-config", m.Name),
		Namespace: m.Namespace,
	}
}

// PasswordSecretKeyRef defines the key selector for the admin password Secret.
func (m *DoltCluster) PasswordSecretKeyRef() *corev1.SecretKeySelector {
	return &corev1.SecretKeySelector{
		LocalObjectReference: corev1.LocalObjectReference{
			Name: fmt.Sprintf("%s-credentials", m.Name),
		},
		Key: "admin-password",
	}
}

// UserSecretKeyRef defines the key selector for the admin user Secret.
func (m *DoltCluster) UserSecretKeyRef() *corev1.SecretKeySelector {
	return &corev1.SecretKeySelector{
		LocalObjectReference: corev1.LocalObjectReference{
			Name: fmt.Sprintf("%s-credentials", m.Name),
		},
		Key: "admin-user",
	}
}

// ServiceAccountKey defines the service account key
func (m *DoltCluster) ServiceAccountKey() types.NamespacedName {
	return types.NamespacedName{
		Name:      m.Spec.ServiceAccountName,
		Namespace: m.Namespace,
	}
}
