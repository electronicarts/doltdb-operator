package builder

import (
	"reflect"
	"testing"

	doltv1alpha "github.com/electronicarts/doltdb-operator/api/v1alpha"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestDoltDBPodMeta(t *testing.T) {
	objMeta := metav1.ObjectMeta{
		Name: "doltdb-obj",
	}
	tests := []struct {
		name     string
		doltdb   *doltv1alpha.DoltDB
		wantMeta *metav1.ObjectMeta
	}{
		{
			name: "empty",
			doltdb: &doltv1alpha.DoltDB{
				ObjectMeta: objMeta,
			},
			wantMeta: &metav1.ObjectMeta{
				Labels: map[string]string{
					"app.kubernetes.io/name": objMeta.Name,
				},
				Annotations: map[string]string{},
			},
		},
		{
			name: "HA",
			doltdb: &doltv1alpha.DoltDB{
				ObjectMeta: objMeta,
				Spec: doltv1alpha.DoltDBSpec{
					EngineVersion: "1.43.5",
					Replication: &doltv1alpha.Replication{
						Enabled: true,
					},
				},
			},
			wantMeta: &metav1.ObjectMeta{
				Labels: map[string]string{
					"app.kubernetes.io/name":    objMeta.Name,
					"app.kubernetes.io/version": "1.43.5",
				},
				Annotations: map[string]string{},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			podTpl := doltPodTemplate(objMeta, tt.doltdb)
			assertObjectMeta(t, &podTpl.ObjectMeta, tt.wantMeta.Labels, tt.wantMeta.Annotations)
		})
	}
}

func TestDoltDBPodBuilderResources(t *testing.T) {
	objMeta := metav1.ObjectMeta{
		Name: "test-doltdb-builder-resources",
	}
	tests := []struct {
		name          string
		doltdb        *doltv1alpha.DoltDB
		wantResources corev1.ResourceRequirements
	}{
		{
			name: "no resources",
			doltdb: &doltv1alpha.DoltDB{
				ObjectMeta: objMeta,
			},
			wantResources: doltResourceRequirements(&doltv1alpha.DoltDB{}),
		},
		{
			name: "doltdb resources",
			doltdb: &doltv1alpha.DoltDB{
				ObjectMeta: objMeta,
				Spec: doltv1alpha.DoltDBSpec{
					Resources: &v1.ResourceRequirements{
						Requests: corev1.ResourceList{
							"cpu": resource.MustParse("300m"),
						},
					},
				},
			},
			wantResources: corev1.ResourceRequirements{
				Requests: corev1.ResourceList{
					"cpu": resource.MustParse("300m"),
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			podTpl := doltPodTemplate(objMeta, tt.doltdb)
			if len(podTpl.Spec.Containers) != 1 {
				t.Error("expecting to have one container")
			}
			resources := podTpl.Spec.Containers[0].Resources
			if !reflect.DeepEqual(resources, tt.wantResources) {
				t.Errorf("unexpected resources, got: %v, expected: %v", resources, tt.wantResources)
			}
		})
	}
}

// TODO: we should test all other things like volumes, init containers, etc
