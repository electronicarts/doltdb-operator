package builder

import (
	"reflect"
	"testing"

	"k8s.io/utils/ptr"

	doltv1alpha "github.com/electronicarts/doltdb-operator/api/v1alpha"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestBuildExternalSnapshotMeta(t *testing.T) {
	builder := newTestBuilder()
	pvcName := "dolt-pvc"
	storageClassName := "csi-hostpath-snapclass-v1"
	tests := []struct {
		name     string
		pvcName  string
		doltdb   *doltv1alpha.DoltDB
		wantMeta VolumeSnapshot
	}{
		{
			name: "dolt-pvc-${DATE}",
			wantMeta: VolumeSnapshot{
				Metadata: metav1.ObjectMeta{
					Labels: map[string]string{
						"app.kubernetes.io/part-of": "doltdb",
					},
					Annotations: map[string]string{},
				},
				VolumeSnapshotSpec: VolumeSnapshotSpec{
					Source: Source{
						PersistentVolumeClaimName: pvcName,
					},
					VolumeSnapshotClassName: storageClassName,
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			volumeSnapshot, err := builder.BuildExternalSnapshot(tt.pvcName, &doltv1alpha.DoltDB{
				ObjectMeta: metav1.ObjectMeta{
					Name: "doltdb",
				},
			})
			if err != nil {
				t.Fatalf("unexpected error building Volumesnapshot: %v", err)
			}
			assertObjectMeta(t, &volumeSnapshot.Metadata, tt.wantMeta.Metadata.Labels, tt.wantMeta.Metadata.Annotations)
		})
	}
}

func TestExternalSnapshot(t *testing.T) {
	builder := newTestBuilder()
	pvcName := "dolt-pvc"
	storageClassName := "csi-hostpath-snapclass-v1"
	objMeta := metav1.ObjectMeta{
		Name: "doltdb-snapshot",
	}
	tests := []struct {
		name         string
		pvcName      string
		doltdb       *doltv1alpha.DoltDB
		wantSnapshot VolumeSnapshot
	}{
		{
			name: "dolt-pvc-${DATE}",
			doltdb: &doltv1alpha.DoltDB{
				ObjectMeta: objMeta,
			},
			wantSnapshot: VolumeSnapshot{
				Metadata: metav1.ObjectMeta{
					Labels: map[string]string{
						"app.kubernetes.io/managed-by": "doltdb",
					},
					Annotations: map[string]string{},
				},
				VolumeSnapshotSpec: VolumeSnapshotSpec{
					Source: Source{
						PersistentVolumeClaimName: pvcName,
					},
					VolumeSnapshotClassName: storageClassName,
				},
			},
		},
		{
			name:    "With Storage Class",
			pvcName: "dolt-storage-class",
			doltdb: &doltv1alpha.DoltDB{
				ObjectMeta: objMeta,
				Spec: doltv1alpha.DoltDBSpec{
					Storage: doltv1alpha.Storage{
						StorageClassName: ptr.To("standard-resize"),
					},
				},
			},
			wantSnapshot: VolumeSnapshot{
				Metadata: metav1.ObjectMeta{
					Labels: map[string]string{
						"app.kubernetes.io/managed-by": "doltdb",
					},
					Annotations: map[string]string{},
				},
				VolumeSnapshotSpec: VolumeSnapshotSpec{
					Source: Source{
						PersistentVolumeClaimName: pvcName,
					},
					VolumeSnapshotClassName: "standard-resize",
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			volumeSnapshot, err := builder.BuildExternalSnapshot(tt.pvcName, tt.doltdb)
			if err != nil {
				t.Fatalf("unexpected error building Volumesnapshot: %v", err)
			}
			if !reflect.DeepEqual(volumeSnapshot.VolumeSnapshotSpec, tt.wantSnapshot.VolumeSnapshotSpec) {
				if !reflect.DeepEqual(
					volumeSnapshot.VolumeSnapshotSpec.VolumeSnapshotClassName,
					tt.wantSnapshot.VolumeSnapshotSpec.VolumeSnapshotClassName,
				) {
					t.Errorf("VolumeSnapshot.VolumeSnapshotSpec = %v, want %v", volumeSnapshot.VolumeSnapshotSpec, tt.wantSnapshot.VolumeSnapshotSpec)
				}
			}
		})
	}
}
