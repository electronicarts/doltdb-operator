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
		doltdb   *doltv1alpha.DoltCluster
		wantMeta *metav1.ObjectMeta
	}{
		{
			name: "empty",
			doltdb: &doltv1alpha.DoltCluster{
				ObjectMeta: objMeta,
			},
			wantMeta: &metav1.ObjectMeta{
				Labels: map[string]string{
					"app.kubernetes.io/name":     objMeta.Name,
					"app.kubernetes.io/instance": objMeta.Name,
				},
				Annotations: map[string]string{},
			},
		},
		{
			name: "HA",
			doltdb: &doltv1alpha.DoltCluster{
				ObjectMeta: objMeta,
				Spec: doltv1alpha.DoltClusterSpec{
					EngineVersion: "1.43.5",
					Replication: &doltv1alpha.Replication{
						Enabled: true,
					},
				},
			},
			wantMeta: &metav1.ObjectMeta{
				Labels: map[string]string{
					"app.kubernetes.io/name":     objMeta.Name,
					"app.kubernetes.io/instance": objMeta.Name,
					"app.kubernetes.io/version":  "1.43.5",
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
		doltdb        *doltv1alpha.DoltCluster
		wantResources corev1.ResourceRequirements
	}{
		{
			name: "no resources",
			doltdb: &doltv1alpha.DoltCluster{
				ObjectMeta: objMeta,
			},
			wantResources: doltResourceRequirements(&doltv1alpha.DoltCluster{}),
		},
		{
			name: "doltdb resources",
			doltdb: &doltv1alpha.DoltCluster{
				ObjectMeta: objMeta,
				Spec: doltv1alpha.DoltClusterSpec{
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

// func TestMariadbPodBuilderServiceAccount(t *testing.T) {
// 	builder := newDefaultTestBuilder(t)
// 	objMeta := metav1.ObjectMeta{
// 		Name: "test-mariadb-builder-serviceaccount",
// 	}
// 	tests := []struct {
// 		name               string
// 		mariadb            *doltv1alpha.MariaDB
// 		opts               []mariadbPodOpt
// 		wantServiceAccount bool
// 	}{
// 		{
// 			name: "serviceaccount",
// 			mariadb: &doltv1alpha.MariaDB{
// 				ObjectMeta: objMeta,
// 				Spec: doltv1alpha.MariaDBSpec{
// 					Galera: &doltv1alpha.Galera{
// 						Enabled: true,
// 					},
// 				},
// 			},
// 			opts:               nil,
// 			wantServiceAccount: true,
// 		},
// 		{
// 			name: "no serviceaccount",
// 			mariadb: &doltv1alpha.MariaDB{
// 				ObjectMeta: objMeta,
// 				Spec: doltv1alpha.MariaDBSpec{
// 					Galera: &doltv1alpha.Galera{
// 						Enabled: true,
// 					},
// 				},
// 			},
// 			opts: []mariadbPodOpt{
// 				withServiceAccount(false),
// 			},
// 			wantServiceAccount: false,
// 		},
// 	}

// 	for _, tt := range tests {
// 		t.Run(tt.name, func(t *testing.T) {
// 			podTpl, err := builder.mariadbPodTemplate(tt.mariadb, tt.opts...)
// 			if err != nil {
// 				t.Fatalf("unexpected error building MariaDB Pod template: %v", err)
// 			}
// 			if len(podTpl.Spec.Containers) == 0 {
// 				t.Error("expecting to have containers")
// 			}

// 			container := podTpl.Spec.Containers[0]
// 			scName := podTpl.Spec.ServiceAccountName
// 			scVol := datastructures.Find(podTpl.Spec.Volumes, func(vol corev1.Volume) bool {
// 				return vol.Name == ServiceAccountVolume
// 			})
// 			scVolMount := datastructures.Find(container.VolumeMounts, func(volMount corev1.VolumeMount) bool {
// 				return volMount.Name == ServiceAccountVolume
// 			})

// 			if tt.wantServiceAccount {
// 				if scName != objMeta.Name {
// 					t.Error("expecting to have ServiceAccount")
// 				}
// 				if scVol == nil {
// 					t.Error("expecting to have ServiceAccount Volume")
// 				}
// 				if scVolMount == nil {
// 					t.Error("expecting to have ServiceAccount VolumeMount")
// 				}
// 			} else {
// 				if scName != "" {
// 					t.Error("expecting to NOT have ServiceAccount")
// 				}
// 				if scVol != nil {
// 					t.Error("expecting to NOT have ServiceAccount Volume")
// 				}
// 				if scVolMount != nil {
// 					t.Error("expecting to NOT have ServiceAccount VolumeMount")
// 				}
// 			}
// 		})
// 	}
// }

// func TestMariadbPodBuilderAffinity(t *testing.T) {
// 	builder := newDefaultTestBuilder(t)
// 	objMeta := metav1.ObjectMeta{
// 		Name: "test-mariadb-builder-affinity",
// 	}
// 	tests := []struct {
// 		name                         string
// 		mariadb                      *doltv1alpha.MariaDB
// 		opts                         []mariadbPodOpt
// 		wantAffinity                 bool
// 		wantTopologySpreadContraints bool
// 		wantNodeAffinity             bool
// 	}{
// 		{
// 			name: "no affinity",
// 			mariadb: &doltv1alpha.MariaDB{
// 				ObjectMeta: objMeta,
// 				Spec: doltv1alpha.MariaDBSpec{
// 					Storage: doltv1alpha.Storage{
// 						Size: ptr.To(resource.MustParse("300Mi")),
// 					},
// 				},
// 			},
// 			opts:                         nil,
// 			wantAffinity:                 false,
// 			wantTopologySpreadContraints: false,
// 			wantNodeAffinity:             false,
// 		},
// 		{
// 			name: "mariadb affinity",
// 			mariadb: &doltv1alpha.MariaDB{
// 				ObjectMeta: objMeta,
// 				Spec: doltv1alpha.MariaDBSpec{
// 					PodTemplate: doltv1alpha.PodTemplate{
// 						Affinity: &doltv1alpha.AffinityConfig{
// 							AntiAffinityEnabled: ptr.To(true),
// 						},
// 					},
// 					Storage: doltv1alpha.Storage{
// 						Size: ptr.To(resource.MustParse("300Mi")),
// 					},
// 				},
// 			},
// 			opts:                         nil,
// 			wantAffinity:                 true,
// 			wantTopologySpreadContraints: false,
// 			wantNodeAffinity:             false,
// 		},
// 		{
// 			name: "mariadb topologyspreadconstraints",
// 			mariadb: &doltv1alpha.MariaDB{
// 				ObjectMeta: objMeta,
// 				Spec: doltv1alpha.MariaDBSpec{
// 					PodTemplate: doltv1alpha.PodTemplate{
// 						TopologySpreadConstraints: []doltv1alpha.TopologySpreadConstraint{
// 							{
// 								MaxSkew:     1,
// 								TopologyKey: "kubernetes.io/hostname",
// 							},
// 						},
// 					},
// 					Storage: doltv1alpha.Storage{
// 						Size: ptr.To(resource.MustParse("300Mi")),
// 					},
// 				},
// 			},
// 			opts:                         nil,
// 			wantAffinity:                 false,
// 			wantTopologySpreadContraints: true,
// 			wantNodeAffinity:             false,
// 		},
// 		{
// 			name: "opt affinity",
// 			mariadb: &doltv1alpha.MariaDB{
// 				ObjectMeta: objMeta,
// 				Spec: doltv1alpha.MariaDBSpec{
// 					Storage: doltv1alpha.Storage{
// 						Size: ptr.To(resource.MustParse("300Mi")),
// 					},
// 				},
// 			},
// 			opts: []mariadbPodOpt{
// 				withAffinity(&corev1.Affinity{}),
// 				withAffinityEnabled(true),
// 			},
// 			wantAffinity:                 true,
// 			wantTopologySpreadContraints: false,
// 			wantNodeAffinity:             false,
// 		},
// 		{
// 			name: "mariadb and opt affinity",
// 			mariadb: &doltv1alpha.MariaDB{
// 				ObjectMeta: objMeta,
// 				Spec: doltv1alpha.MariaDBSpec{
// 					PodTemplate: doltv1alpha.PodTemplate{
// 						Affinity: &doltv1alpha.AffinityConfig{
// 							AntiAffinityEnabled: ptr.To(true),
// 						},
// 						TopologySpreadConstraints: []doltv1alpha.TopologySpreadConstraint{
// 							{
// 								MaxSkew:     1,
// 								TopologyKey: "kubernetes.io/hostname",
// 							},
// 						},
// 					},
// 					Storage: doltv1alpha.Storage{
// 						Size: ptr.To(resource.MustParse("300Mi")),
// 					},
// 				},
// 			},
// 			opts: []mariadbPodOpt{
// 				withAffinity(&corev1.Affinity{}),
// 				withAffinityEnabled(true),
// 			},
// 			wantAffinity:                 true,
// 			wantTopologySpreadContraints: true,
// 			wantNodeAffinity:             false,
// 		},
// 		{
// 			name: "disable affinity",
// 			mariadb: &doltv1alpha.MariaDB{
// 				ObjectMeta: objMeta,
// 				Spec: doltv1alpha.MariaDBSpec{
// 					PodTemplate: doltv1alpha.PodTemplate{
// 						Affinity: &doltv1alpha.AffinityConfig{
// 							AntiAffinityEnabled: ptr.To(true),
// 						},
// 						TopologySpreadConstraints: []doltv1alpha.TopologySpreadConstraint{
// 							{
// 								MaxSkew:     1,
// 								TopologyKey: "kubernetes.io/hostname",
// 							},
// 						},
// 					},
// 					Storage: doltv1alpha.Storage{
// 						Size: ptr.To(resource.MustParse("300Mi")),
// 					},
// 				},
// 			},
// 			opts: []mariadbPodOpt{
// 				withAffinity(&corev1.Affinity{}),
// 				withAffinityEnabled(false),
// 			},
// 			wantAffinity:                 false,
// 			wantTopologySpreadContraints: false,
// 			wantNodeAffinity:             false,
// 		},
// 		{
// 			name: "mariadb with node affinity",
// 			mariadb: &doltv1alpha.MariaDB{
// 				ObjectMeta: objMeta,
// 				Spec: doltv1alpha.MariaDBSpec{
// 					PodTemplate: doltv1alpha.PodTemplate{
// 						Affinity: &doltv1alpha.AffinityConfig{
// 							Affinity: doltv1alpha.Affinity{
// 								NodeAffinity: &doltv1alpha.NodeAffinity{
// 									RequiredDuringSchedulingIgnoredDuringExecution: &doltv1alpha.NodeSelector{
// 										NodeSelectorTerms: []doltv1alpha.NodeSelectorTerm{
// 											{
// 												MatchExpressions: []doltv1alpha.NodeSelectorRequirement{
// 													{
// 														Key:      "kubernetes.io/hostname",
// 														Operator: corev1.NodeSelectorOpIn,
// 														Values:   []string{"node1", "node2"},
// 													},
// 												},
// 											},
// 										},
// 									},
// 								},
// 							},
// 							AntiAffinityEnabled: nil,
// 						},
// 					},
// 					Storage: doltv1alpha.Storage{
// 						Size: ptr.To(resource.MustParse("300Mi")),
// 					},
// 				},
// 			},
// 			opts:                         nil,
// 			wantAffinity:                 true,
// 			wantTopologySpreadContraints: false,
// 			wantNodeAffinity:             true,
// 		},
// 	}

// 	for _, tt := range tests {
// 		t.Run(tt.name, func(t *testing.T) {
// 			podTpl, err := builder.mariadbPodTemplate(tt.mariadb, tt.opts...)
// 			if err != nil {
// 				t.Fatalf("unexpected error building MariaDB Pod template: %v", err)
// 			}
// 			if tt.wantAffinity && podTpl.Spec.Affinity == nil {
// 				t.Error("expected affinity to have been set")
// 			}
// 			if !tt.wantAffinity && podTpl.Spec.Affinity != nil {
// 				t.Error("expected affinity to not have been set")
// 			}

// 			if tt.wantTopologySpreadContraints && podTpl.Spec.TopologySpreadConstraints == nil {
// 				t.Error("expected topologySpreadConstraints to have been set")
// 			}
// 			if !tt.wantTopologySpreadContraints && podTpl.Spec.TopologySpreadConstraints != nil {
// 				t.Error("expected topologySpreadConstraints to not have been set")
// 			}

// 			if tt.wantNodeAffinity && podTpl.Spec.Affinity.NodeAffinity == nil {
// 				t.Error("expected node affinity to have been set")
// 			}
// 			if !tt.wantNodeAffinity && podTpl.Spec.Affinity != nil && podTpl.Spec.Affinity.NodeAffinity != nil {
// 				t.Error("expected node affinity to not have been set")
// 			}
// 		})
// 	}
// }

// func TestMariadbPodBuilderInitContainers(t *testing.T) {
// 	builder := newDefaultTestBuilder(t)
// 	objMeta := metav1.ObjectMeta{
// 		Name: "test-mariadb-builder-initcontainers",
// 	}
// 	tests := []struct {
// 		name               string
// 		mariadb            *doltv1alpha.MariaDB
// 		wantInitContainers int
// 	}{
// 		{
// 			name: "no init containers",
// 			mariadb: &doltv1alpha.MariaDB{
// 				ObjectMeta: objMeta,
// 				Spec: doltv1alpha.MariaDBSpec{
// 					Image: "mariadb:11.4.3",
// 					PodTemplate: doltv1alpha.PodTemplate{
// 						InitContainers: nil,
// 					},
// 				},
// 			},
// 			wantInitContainers: 0,
// 		},
// 		{
// 			name: "init containers",
// 			mariadb: &doltv1alpha.MariaDB{
// 				ObjectMeta: objMeta,
// 				Spec: doltv1alpha.MariaDBSpec{
// 					Image: "mariadb:11.4.3",
// 					PodTemplate: doltv1alpha.PodTemplate{
// 						InitContainers: []doltv1alpha.Container{
// 							{
// 								Image: "busybox:latest",
// 							},
// 							{
// 								Image: "busybox:latest",
// 							},
// 						},
// 					},
// 				},
// 			},
// 			wantInitContainers: 2,
// 		},
// 	}

// 	for _, tt := range tests {
// 		t.Run(tt.name, func(t *testing.T) {
// 			podTpl, err := builder.mariadbPodTemplate(tt.mariadb)
// 			if err != nil {
// 				t.Fatalf("unexpected error building MariaDB Pod template: %v", err)
// 			}

// 			if len(podTpl.Spec.InitContainers) != tt.wantInitContainers {
// 				t.Errorf("unexpected number of init containers, got: %v, want: %v", len(podTpl.Spec.InitContainers), tt.wantInitContainers)
// 			}

// 			for _, container := range podTpl.Spec.InitContainers {
// 				if container.Image == "" {
// 					t.Error("expected container image to be set")
// 				}
// 				if container.Env == nil {
// 					t.Error("expected container env to be set")
// 				}
// 				if container.VolumeMounts == nil {
// 					t.Error("expected container VolumeMounts to be set")
// 				}
// 			}
// 		})
// 	}
// }

// func TestMariadbPodBuilderSidecarContainers(t *testing.T) {
// 	builder := newDefaultTestBuilder(t)
// 	objMeta := metav1.ObjectMeta{
// 		Name: "test-mariadb-builder-sidecarcontainers",
// 	}
// 	tests := []struct {
// 		name           string
// 		mariadb        *doltv1alpha.MariaDB
// 		wantContainers int
// 	}{
// 		{
// 			name: "no sidecar containers",
// 			mariadb: &doltv1alpha.MariaDB{
// 				ObjectMeta: objMeta,
// 				Spec: doltv1alpha.MariaDBSpec{
// 					Image: "mariadb:11.4.3",
// 					PodTemplate: doltv1alpha.PodTemplate{
// 						SidecarContainers: nil,
// 					},
// 				},
// 			},
// 			wantContainers: 1,
// 		},
// 		{
// 			name: "sidecar containers",
// 			mariadb: &doltv1alpha.MariaDB{
// 				ObjectMeta: objMeta,
// 				Spec: doltv1alpha.MariaDBSpec{
// 					Image: "mariadb:11.4.3",
// 					PodTemplate: doltv1alpha.PodTemplate{
// 						SidecarContainers: []doltv1alpha.Container{
// 							{
// 								Image: "busybox:latest",
// 							},
// 							{
// 								Image: "busybox:latest",
// 							},
// 						},
// 					},
// 				},
// 			},
// 			wantContainers: 3,
// 		},
// 	}

// 	for _, tt := range tests {
// 		t.Run(tt.name, func(t *testing.T) {
// 			podTpl, err := builder.mariadbPodTemplate(tt.mariadb)
// 			if err != nil {
// 				t.Fatalf("unexpected error building MariaDB Pod template: %v", err)
// 			}

// 			if len(podTpl.Spec.Containers) != tt.wantContainers {
// 				t.Errorf("unexpected number of containers, got: %v, want: %v", len(podTpl.Spec.Containers), tt.wantContainers)
// 			}

// 			for _, container := range podTpl.Spec.Containers {
// 				if container.Image == "" {
// 					t.Error("expected container image to be set")
// 				}
// 				if container.Env == nil {
// 					t.Error("expected container env to be set")
// 				}
// 				if container.VolumeMounts == nil {
// 					t.Error("expected container VolumeMounts to be set")
// 				}
// 			}
// 		})
// 	}
// }

// func TestMariadbConfigVolume(t *testing.T) {
// 	mariadb := &doltv1alpha.MariaDB{
// 		ObjectMeta: metav1.ObjectMeta{
// 			Name: "test-mariadb-builder",
// 		},
// 		Spec: doltv1alpha.MariaDBSpec{
// 			Storage: doltv1alpha.Storage{
// 				Size: ptr.To(resource.MustParse("300Mi")),
// 			},
// 		},
// 	}

// 	volume := mariadbConfigVolume(mariadb)
// 	if volume.Projected == nil {
// 		t.Fatal("expected volume to be projected")
// 	}
// 	expectedSources := 1
// 	if len(volume.Projected.Sources) != expectedSources {
// 		t.Fatalf("expecting to have %d sources, got: %d", expectedSources, len(volume.Projected.Sources))
// 	}
// 	expectedKey := "0-default.cnf"
// 	if volume.Projected.Sources[0].ConfigMap.Items[0].Key != expectedKey {
// 		t.Fatalf("expecting to have '%s' key, got: '%s'", expectedKey, volume.Projected.Sources[0].ConfigMap.Items[0].Key)
// 	}

// 	mariadb.Spec.MyCnfConfigMapKeyRef = &doltv1alpha.ConfigMapKeySelector{
// 		LocalObjectReference: doltv1alpha.LocalObjectReference{
// 			Name: "test",
// 		},
// 		Key: "my.cnf",
// 	}

// 	volume = mariadbConfigVolume(mariadb)
// 	if volume.Projected == nil {
// 		t.Fatal("expected volume to be projected")
// 	}
// 	expectedSources = 2
// 	if len(volume.Projected.Sources) != expectedSources {
// 		t.Fatalf("expecting to have %d sources, got: %d", expectedSources, len(volume.Projected.Sources))
// 	}
// 	expectedKey = "0-default.cnf"
// 	if volume.Projected.Sources[0].ConfigMap.Items[0].Key != expectedKey {
// 		t.Fatalf("expecting to have '%s' key, got: '%s'", expectedKey, volume.Projected.Sources[0].ConfigMap.Items[0].Key)
// 	}
// 	expectedKey = "my.cnf"
// 	if volume.Projected.Sources[1].ConfigMap.Items[0].Key != expectedKey {
// 		t.Fatalf("expecting to have '%s' key, got: '%s'", expectedKey, volume.Projected.Sources[0].ConfigMap.Items[0].Key)
// 	}
// }
