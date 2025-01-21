package builder

import (
	doltv1alpha "github.com/electronicarts/doltdb-operator/api/v1alpha"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

const (
	SnapshotClassNameConst   = "csi-hostpath-snapclass-v1"
	VolumeSnapshotAPIVersion = "snapshot.storage.k8s.io/v1"
	VolumeSnapshotKind       = "VolumeSnapshot"
)

// VolumeSnapshot represents a VolumeSnapshot CR.
type VolumeSnapshot struct {
	APIVersion         string             `json:"apiVersion" yaml:"apiVersion"`
	Kind               string             `json:"kind" yaml:"kind"`
	Metadata           metav1.ObjectMeta  `json:"metadata" yaml:"metadata"`
	VolumeSnapshotSpec VolumeSnapshotSpec `json:"spec" yaml:"spec"`
}

// Source represents the source of the VolumeSnapshot.
type Source struct {
	PersistentVolumeClaimName string `json:"persistentVolumeClaimName" yaml:"persistentVolumeClaimName"`
}

// VolumeSnapshotSpec represents the spec of the VolumeSnapshot.
type VolumeSnapshotSpec struct {
	VolumeSnapshotClassName string `json:"volumeSnapshotClassName" yaml:"volumeSnapshotClassName"`
	Source                  Source `json:"source" yaml:"source"`
}

// BuildExternalSnapshot creates a snapshot cr for taking volume backup.
func (b *Builder) BuildExternalSnapshot(pvcName string, doltdb *doltv1alpha.DoltDB) (VolumeSnapshot, error) {
	labels := NewLabelsBuilder().
		WithDoltSelectorLabels(doltdb).
		WithPVCRole(DoltDataVolume).
		Build()
	objMeta :=
		NewMetadataBuilder(types.NamespacedName{
			Name:      pvcName + "-${DATE}",
			Namespace: doltdb.Namespace,
		}).
			WithLabels(labels).
			Build()
	snapshotClassName := SnapshotClassNameConst
	if doltdb.Spec.Storage.StorageClassName != nil {
		snapshotClassName = *doltdb.Spec.Storage.StorageClassName
	}
	// Define the VolumeSnapshot object
	volumeSnapshot := VolumeSnapshot{
		APIVersion: VolumeSnapshotAPIVersion,
		Kind:       VolumeSnapshotKind,
		Metadata:   objMeta,
		VolumeSnapshotSpec: VolumeSnapshotSpec{
			VolumeSnapshotClassName: snapshotClassName,
			Source: Source{
				PersistentVolumeClaimName: pvcName, // PVC that you want to snapshot
			},
		},
	}
	return volumeSnapshot, nil
}
