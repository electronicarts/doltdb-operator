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
func (b *Builder) BuildDoltStatefulSet(key types.NamespacedName, doltdb *doltv1alpha.DoltDB) (*appsv1.StatefulSet, error) {
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

	statefulSet := &appsv1.StatefulSet{
		ObjectMeta: objMeta,
		Spec: appsv1.StatefulSetSpec{
			// PersistentVolumeClaimRetentionPolicy: ,
			Selector: &metav1.LabelSelector{
				MatchLabels: matchLabels,
			},
			ServiceName: "dolt-internal",
			UpdateStrategy: appsv1.StatefulSetUpdateStrategy{
				Type: appsv1.OnDeleteStatefulSetStrategyType,
			},
			Replicas:             &doltdb.Spec.Replicas,
			Template:             doltPodTemplate(objMeta, doltdb),
			VolumeClaimTemplates: doltVolumeClaimTemplates(objMeta, doltdb),
		},
	}

	if err := controllerutil.SetControllerReference(doltdb, statefulSet, b.scheme); err != nil {
		return nil, fmt.Errorf("error setting controller reference to StatefulSet: %v", err)
	}

	return statefulSet, nil
}

// doltVolumeClaimTemplates constructs a PersistentVolumeClaim for the given DoltDB.
func doltVolumeClaimTemplates(metadata metav1.ObjectMeta, doltdb *doltv1alpha.DoltDB) []corev1.PersistentVolumeClaim {
	labels := NewLabelsBuilder().
		WithDoltSelectorLabels(doltdb).
		Build()

	objMeta :=
		NewMetadataBuilder(types.NamespacedName{
			Name:      DoltDataVolume,
			Namespace: doltdb.Namespace,
		}).
			WithMetadata(&metadata).
			WithLabels(labels).
			Build()

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
		},
	}

	if doltdb.Spec.Storage.StorageClassName != nil {
		pvc.Spec.StorageClassName = doltdb.Spec.Storage.StorageClassName
	}

	return []corev1.PersistentVolumeClaim{pvc}
}
