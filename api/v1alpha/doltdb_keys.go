// Copyright (c) 2025 Electronic Arts Inc. All rights reserved.

package v1alpha

import (
	"fmt"

	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
)

// ServiceKey defines the key for the standalone Service
func (d *DoltDB) ServiceKey() types.NamespacedName {
	return types.NamespacedName{
		Name:      d.Name,
		Namespace: d.Namespace,
	}
}

// InternalServiceKey defines the key for the internal headless Service
func (d *DoltDB) InternalServiceKey() types.NamespacedName {
	return types.NamespacedName{
		Name:      fmt.Sprintf("%s-internal", d.Name),
		Namespace: d.Namespace,
	}
}

// PrimaryServiceKey defines the key for the internal primary instance
func (d *DoltDB) PrimaryServiceKey() types.NamespacedName {
	return types.NamespacedName{
		Name:      fmt.Sprintf("%s-primary", d.Name),
		Namespace: d.Namespace,
	}
}

// InternalServiceKey defines the key for the internal reader instances
func (d *DoltDB) ReaderServiceKey() types.NamespacedName {
	return types.NamespacedName{
		Name:      fmt.Sprintf("%s-reader", d.Name),
		Namespace: d.Namespace,
	}
}

// PodDisruptionBudgetKey defines the key for the PodDisruptionBudget
func (d *DoltDB) PodDisruptionBudgetKey() types.NamespacedName {
	return types.NamespacedName{
		Name:      fmt.Sprintf("%s-pdb", d.Name),
		Namespace: d.Namespace,
	}
}

// PVCKey defines the PVC keys.
func (d *DoltDB) PVCKey(name string, index int) types.NamespacedName {
	return types.NamespacedName{
		Name:      fmt.Sprintf("%s-%s-%d", name, d.Name, index),
		Namespace: d.Namespace,
	}
}

// ConfigMapKeySelector defines the key selector for the ConfigMap used for replication healthchecks.
func (d *DoltDB) DefaultConfigMapKeyRef() ConfigMapKeySelector {
	return ConfigMapKeySelector{
		LocalObjectReference: LocalObjectReference{
			Name: fmt.Sprintf("%s-config", d.Name),
		},
		Key: "dolt-config",
	}
}

// ConfigDataKey defines the config map keys.
func (d *DoltDB) DefaultConfigMapKey() types.NamespacedName {
	return types.NamespacedName{
		Name:      fmt.Sprintf("%s-config", d.Name),
		Namespace: d.Namespace,
	}
}

// PasswordSecretKeyRef defines the key selector for the admin password Secret.
func (d *DoltDB) RootPasswordSecretKeyRef() SecretKeySelector {
	return SecretKeySelector{
		LocalObjectReference: LocalObjectReference{
			Name: fmt.Sprintf("%s-credentials", d.Name),
		},
		Key: "admin-password",
	}
}

// UserSecretKeyRef defines the key selector for the admin user Secret.
func (d *DoltDB) RootUserSecretKeyRef() SecretKeySelector {
	return SecretKeySelector{
		LocalObjectReference: LocalObjectReference{
			Name: fmt.Sprintf("%s-credentials", d.Name),
		},
		Key: "admin-user",
	}
}

// ServiceAccountKey defines the service account key
func (d *DoltDB) ServiceAccountKey() types.NamespacedName {
	return types.NamespacedName{
		Name:      ptr.Deref(d.Spec.ServiceAccountName, d.Name),
		Namespace: d.Namespace,
	}
}
