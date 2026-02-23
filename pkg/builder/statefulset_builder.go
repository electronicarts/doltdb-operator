// Copyright (c) 2025 Electronic Arts Inc. All rights reserved.

package builder

import (
	"fmt"
	"strconv"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	doltv1alpha "github.com/electronicarts/doltdb-operator/api/v1alpha"
	"github.com/electronicarts/doltdb-operator/pkg/dolt"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

// BuildDoltStatefulSet constructs a StatefulSet for a DoltDB based on the provided NamespacedName and DoltDB object.
// It sets up the metadata, labels, volume claim templates, and pod template for the StatefulSet.
// The configMapHash parameter is included in the pod template annotations to trigger pod restarts
// when the ConfigMap content changes (e.g., when replicas are scaled up or down).
// If UpdateStrategy is set to "Never", the configMapHash is not included in the pod template.
func (b *Builder) BuildDoltStatefulSet(
	key types.NamespacedName,
	doltdb *doltv1alpha.DoltDB,
	configMapHash string,
) (*appsv1.StatefulSet, error) {
	labels := NewLabelsBuilder().
		WithDoltSelectorLabels(doltdb).
		WithVersion(doltdb.Spec.EngineVersion).
		Build()

	objMeta := NewMetadataBuilder(key).
		WithMetadata(&doltdb.ObjectMeta).
		WithAnnotations(map[string]string{
			dolt.Annotation:            key.Name,
			dolt.ReplicationAnnotation: strconv.FormatBool(doltdb.Replication().Enabled),
		}).
		WithLabels(labels).
		Build()

	matchLabels := NewLabelsBuilder().WithDoltSelectorLabels(doltdb).Build()

	// If UpdateStrategy is "Never", don't include the ConfigMap hash in pod annotations
	// This prevents automatic pod restarts when ConfigMap changes
	podConfigMapHash := configMapHash
	if doltdb.Spec.UpdateStrategy == doltv1alpha.NeverUpdateType {
		podConfigMapHash = ""
	}

	statefulSet := &appsv1.StatefulSet{
		ObjectMeta: objMeta,
		Spec: appsv1.StatefulSetSpec{
			PersistentVolumeClaimRetentionPolicy: doltPVCRetentionPolicy(doltdb),
			Selector: &metav1.LabelSelector{
				MatchLabels: matchLabels,
			},
			ServiceName:          doltdb.InternalServiceKey().Name,
			UpdateStrategy:       doltStatefulSetUpdateStrategy(doltdb),
			Replicas:             &doltdb.Spec.Replicas,
			Template:             doltPodTemplate(objMeta, doltdb, podConfigMapHash),
			VolumeClaimTemplates: doltVolumeClaimTemplates(objMeta, doltdb),
		},
	}

	if err := controllerutil.SetControllerReference(doltdb, statefulSet, b.scheme); err != nil {
		return nil, fmt.Errorf("error setting controller reference to StatefulSet: %v", err)
	}

	return statefulSet, nil
}

// doltStatefulSetUpdateStrategy returns the Kubernetes StatefulSet update strategy
// based on the DoltDB UpdateStrategy setting.
// This follows the same pattern as the MariaDB operator.
func doltStatefulSetUpdateStrategy(doltdb *doltv1alpha.DoltDB) appsv1.StatefulSetUpdateStrategy {
	switch doltdb.Spec.UpdateStrategy {
	case doltv1alpha.ReplicasFirstPrimaryLastUpdateType, "":
		// Operator manages pod deletions (replicas first, then primary)
		return appsv1.StatefulSetUpdateStrategy{
			Type: appsv1.OnDeleteStatefulSetStrategyType,
		}
	case doltv1alpha.RollingUpdateUpdateType:
		// Kubernetes handles updates natively (highest to lowest ordinal)
		return appsv1.StatefulSetUpdateStrategy{
			Type: appsv1.RollingUpdateStatefulSetStrategyType,
		}
	case doltv1alpha.OnDeleteUpdateType:
		// User must manually delete pods to trigger updates
		return appsv1.StatefulSetUpdateStrategy{
			Type: appsv1.OnDeleteStatefulSetStrategyType,
		}
	case doltv1alpha.NeverUpdateType:
		// StatefulSet will never be updated
		return appsv1.StatefulSetUpdateStrategy{}
	default:
		// Default to OnDelete for safety
		return appsv1.StatefulSetUpdateStrategy{
			Type: appsv1.OnDeleteStatefulSetStrategyType,
		}
	}
}

// doltVolumeClaimTemplates constructs a PersistentVolumeClaim for the given DoltDB.
func doltVolumeClaimTemplates(metadata metav1.ObjectMeta, doltdb *doltv1alpha.DoltDB) []corev1.PersistentVolumeClaim {
	labels := NewLabelsBuilder().
		WithDoltSelectorLabels(doltdb).
		WithPVCRole(DoltDataVolume).
		Build()

	objMeta :=
		NewMetadataBuilder(types.NamespacedName{
			Name:      DoltDataVolume,
			Namespace: doltdb.Namespace,
		}).
			WithMetadata(&metadata).
			WithLabels(labels).
			Build()

	var dataSource *corev1.TypedLocalObjectReference

	if doltdb.Spec.Storage.VolumeSnapshot != "" {
		dataSource = &corev1.TypedLocalObjectReference{
			APIGroup: ptr.To("snapshot.storage.k8s.io"),
			Kind:     "VolumeSnapshot",
			Name:     doltdb.Spec.Storage.VolumeSnapshot,
		}
	}

	pvc := corev1.PersistentVolumeClaim{
		ObjectMeta: objMeta,
		Spec: corev1.PersistentVolumeClaimSpec{
			AccessModes: []corev1.PersistentVolumeAccessMode{
				corev1.ReadWriteOnce,
			},
			Resources: corev1.VolumeResourceRequirements{
				Requests: corev1.ResourceList{
					// Apply default
					corev1.ResourceStorage: ptr.Deref(doltdb.Spec.Storage.Size, *resource.NewQuantity(1, "Gi")),
				},
			},
			DataSource: dataSource,
		},
	}

	if doltdb.Spec.Storage.StorageClassName != nil {
		pvc.Spec.StorageClassName = doltdb.Spec.Storage.StorageClassName
	}

	return []corev1.PersistentVolumeClaim{pvc}
}

func doltPVCRetentionPolicy(doltdb *doltv1alpha.DoltDB) *appsv1.StatefulSetPersistentVolumeClaimRetentionPolicy {
	if doltdb.Spec.Storage.RetentionPolicy != nil {
		return doltdb.Spec.Storage.RetentionPolicy
	}

	return &appsv1.StatefulSetPersistentVolumeClaimRetentionPolicy{
		WhenDeleted: appsv1.RetainPersistentVolumeClaimRetentionPolicyType,
		WhenScaled:  appsv1.RetainPersistentVolumeClaimRetentionPolicyType,
	}
}
