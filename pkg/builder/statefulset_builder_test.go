// Copyright (c) 2025 Electronic Arts Inc. All rights reserved.

package builder

import (
	"reflect"
	"testing"

	doltv1alpha "github.com/electronicarts/doltdb-operator/api/v1alpha"
	"github.com/electronicarts/doltdb-operator/pkg/dolt"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func TestDoltDBImagePullSecrets(t *testing.T) {
	builder := newTestBuilder()
	objMeta := metav1.ObjectMeta{
		Name:      "doltdb-image-pull-secrets",
		Namespace: "test",
	}

	tests := []struct {
		name            string
		doltdb          *doltv1alpha.DoltDB
		wantPullSecrets []corev1.LocalObjectReference
	}{
		{
			name: "No Secrets",
			doltdb: &doltv1alpha.DoltDB{
				ObjectMeta: objMeta,
				Spec: doltv1alpha.DoltDBSpec{
					UpdateStrategy: doltv1alpha.ReplicasFirstPrimaryLastUpdateType,
				},
			},
			wantPullSecrets: nil,
		},
		{
			name: "Secrets in DoltDB",
			doltdb: &doltv1alpha.DoltDB{
				ObjectMeta: objMeta,
				Spec: doltv1alpha.DoltDBSpec{
					ImagePullSecrets: []corev1.LocalObjectReference{
						{
							Name: "harbor-registry",
						},
					},
					UpdateStrategy: doltv1alpha.ReplicasFirstPrimaryLastUpdateType,
				},
			},
			wantPullSecrets: []corev1.LocalObjectReference{
				{
					Name: "harbor-registry",
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			job, err := builder.BuildDoltStatefulSet(client.ObjectKeyFromObject(tt.doltdb), tt.doltdb, "test-hash")
			if err != nil {
				t.Fatalf("unexpected error building StatefulSet: %v", err)
			}
			if !reflect.DeepEqual(tt.wantPullSecrets, job.Spec.Template.Spec.ImagePullSecrets) {
				t.Errorf("unexpected ImagePullSecrets, want: %v  got: %v", tt.wantPullSecrets, job.Spec.Template.Spec.ImagePullSecrets)
			}
		})
	}
}

func TestDoltDBStatefulSetMeta(t *testing.T) {
	builder := newTestBuilder()
	objMeta := metav1.ObjectMeta{
		Name: "doltdb",
	}
	tests := []struct {
		name           string
		doltdb         *doltv1alpha.DoltDB
		podAnnotations map[string]string
		wantMeta       *doltv1alpha.DoltDB
		wantPodMeta    *doltv1alpha.DoltDB
	}{
		{
			name: "empty",
			doltdb: &doltv1alpha.DoltDB{
				ObjectMeta: objMeta,
				Spec: doltv1alpha.DoltDBSpec{
					EngineVersion:  "1.43.5",
					UpdateStrategy: doltv1alpha.ReplicasFirstPrimaryLastUpdateType,
				},
			},
			podAnnotations: nil,
			wantMeta: &doltv1alpha.DoltDB{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"app.kubernetes.io/name":    "doltdb",
						"app.kubernetes.io/version": "1.43.5",
					},
					Annotations: map[string]string{
						"k8s.dolthub.com/doltdb":      objMeta.Name,
						"k8s.dolthub.com/replication": "false",
					},
				},
			},
			wantPodMeta: &doltv1alpha.DoltDB{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"app.kubernetes.io/name":    "doltdb",
						"app.kubernetes.io/version": "1.43.5",
					},
					Annotations: map[string]string{
						"k8s.dolthub.com/doltdb":      objMeta.Name,
						"k8s.dolthub.com/replication": "false",
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sts, err := builder.BuildDoltStatefulSet(client.ObjectKeyFromObject(tt.doltdb), tt.doltdb, "test-hash")
			if err != nil {
				t.Fatalf("unexpected error building DoltDB StatefulSet: %v", err)
			}
			assertObjectMeta(t, &sts.ObjectMeta, tt.wantMeta.Labels, tt.wantMeta.Annotations)

			// For pod template annotations, we need to verify the expected ones exist
			// The ConfigMap hash annotation is dynamically generated
			for key, wantValue := range tt.wantPodMeta.Annotations {
				if gotValue, ok := sts.Spec.Template.ObjectMeta.Annotations[key]; !ok || gotValue != wantValue {
					t.Errorf("missing or incorrect pod annotation %q: want %q, got %q", key, wantValue, gotValue)
				}
			}
			// Verify ConfigMap hash annotation is present
			if _, ok := sts.Spec.Template.ObjectMeta.Annotations[dolt.ConfigMapHashAnnotation]; !ok {
				t.Errorf("expected ConfigMap hash annotation %q to be present", dolt.ConfigMapHashAnnotation)
			}
			// Verify expected labels
			for key, wantValue := range tt.wantPodMeta.Labels {
				if gotValue, ok := sts.Spec.Template.ObjectMeta.Labels[key]; !ok || gotValue != wantValue {
					t.Errorf("missing or incorrect pod label %q: want %q, got %q", key, wantValue, gotValue)
				}
			}
		})
	}
}

func TestDoltDBPersistentVolumeClaims(t *testing.T) {
	objMeta := metav1.ObjectMeta{
		Name: "doltdb-obj",
	}
	tests := []struct {
		name        string
		doltdb      *doltv1alpha.DoltDB
		wantVolumes []string
	}{
		{
			name: "standalone",
			doltdb: &doltv1alpha.DoltDB{
				ObjectMeta: objMeta,
				Spec: doltv1alpha.DoltDBSpec{
					Storage: doltv1alpha.Storage{
						Size: ptr.To(resource.MustParse("1Gi")),
					},
				},
			},
			wantVolumes: []string{DoltDataVolume},
		},
		{
			name: "replication",
			doltdb: &doltv1alpha.DoltDB{
				ObjectMeta: objMeta,
				Spec: doltv1alpha.DoltDBSpec{
					Storage: doltv1alpha.Storage{
						Size: ptr.To(resource.MustParse("1Gi")),
					},
					Replication: &doltv1alpha.Replication{
						Enabled: true,
					},
				},
			},
			wantVolumes: []string{DoltDataVolume},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pvcs := doltVolumeClaimTemplates(objMeta, tt.doltdb)
			if len(pvcs) != len(tt.wantVolumes) {
				t.Errorf("unexpected number of PVCs, got: %v, want: %v", len(pvcs), len(tt.wantVolumes))
			}
			for _, wantVolume := range tt.wantVolumes {
				if !hasVolume(pvcs, wantVolume) {
					t.Errorf("expecting Volume \"%s\", but it was not found", wantVolume)
				}
			}
		})
	}
}

func hasVolume(pvcs []corev1.PersistentVolumeClaim, volumeName string) bool {
	for _, p := range pvcs {
		if p.Name == volumeName {
			return true
		}
	}
	return false
}

func TestDoltVolumeClaimTemplatesWithVolumeSnapshot(t *testing.T) {
	objMeta := metav1.ObjectMeta{
		Name:      "doltdb-with-snapshot",
		Namespace: "test-namespace",
	}

	tests := []struct {
		name           string
		doltdb         *doltv1alpha.DoltDB
		wantDataSource *corev1.TypedLocalObjectReference
	}{
		{
			name: "With VolumeSnapshot",
			doltdb: &doltv1alpha.DoltDB{
				ObjectMeta: objMeta,
				Spec: doltv1alpha.DoltDBSpec{
					Storage: doltv1alpha.Storage{
						VolumeSnapshot: "snapshot-name",
					},
				},
			},
			wantDataSource: &corev1.TypedLocalObjectReference{
				APIGroup: ptr.To("snapshot.storage.k8s.io"),
				Kind:     "VolumeSnapshot",
				Name:     "snapshot-name",
			},
		},
		{
			name: "Without VolumeSnapshot",
			doltdb: &doltv1alpha.DoltDB{
				ObjectMeta: objMeta,
				Spec: doltv1alpha.DoltDBSpec{
					Storage: doltv1alpha.Storage{},
				},
			},
			wantDataSource: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pvcs := doltVolumeClaimTemplates(objMeta, tt.doltdb)
			if len(pvcs) != 1 {
				t.Fatalf("expected 1 PVC, got %d", len(pvcs))
			}
			gotDataSource := pvcs[0].Spec.DataSource
			if !reflect.DeepEqual(gotDataSource, tt.wantDataSource) {
				t.Errorf("unexpected DataSource, want: %v, got: %v", tt.wantDataSource, gotDataSource)
			}
		})
	}
}
