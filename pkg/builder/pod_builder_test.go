// Copyright (c) 2025 Electronic Arts Inc. All rights reserved.

package builder

import (
	"fmt"
	"reflect"
	"testing"

	doltv1alpha "github.com/electronicarts/doltdb-operator/api/v1alpha"
	"github.com/electronicarts/doltdb-operator/pkg/dolt"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
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
			podTpl := doltPodTemplate(objMeta, tt.doltdb, "test-hash")
			// Verify expected labels and annotations exist
			for key, wantValue := range tt.wantMeta.Labels {
				if gotValue, ok := podTpl.ObjectMeta.Labels[key]; !ok || gotValue != wantValue {
					t.Errorf("missing or incorrect label %q: want %q, got %q", key, wantValue, gotValue)
				}
			}
			// ConfigMap hash annotation should always be present when hash is provided
			if _, ok := podTpl.ObjectMeta.Annotations[dolt.ConfigMapHashAnnotation]; !ok {
				t.Errorf("expected ConfigMap hash annotation %q to be present", dolt.ConfigMapHashAnnotation)
			}
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
			podTpl := doltPodTemplate(objMeta, tt.doltdb, "test-hash")
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
func TestDoltPodTemplateSpec(t *testing.T) {
	objMeta := metav1.ObjectMeta{
		Name: "test-doltdb-pod-template",
	}

	tests := []struct {
		name     string
		doltdb   *doltv1alpha.DoltDB
		validate func(t *testing.T, podSpec corev1.PodSpec)
	}{
		{
			name: "basic pod spec",
			doltdb: &doltv1alpha.DoltDB{
				ObjectMeta: objMeta,
				Spec: doltv1alpha.DoltDBSpec{
					EngineVersion: "1.43.5",
				},
			},
			validate: func(t *testing.T, podSpec corev1.PodSpec) {
				if *podSpec.AutomountServiceAccountToken != false {
					t.Error("AutomountServiceAccountToken should be false")
				}
				if podSpec.ServiceAccountName != objMeta.Name {
					t.Errorf("ServiceAccountName should be %s, got %s", objMeta.Name, podSpec.ServiceAccountName)
				}
				if len(podSpec.Containers) == 0 {
					t.Error("Should have at least one container")
				}
				if len(podSpec.Volumes) == 0 {
					t.Error("Should have volumes")
				}
			},
		},
		{
			name: "custom service account",
			doltdb: &doltv1alpha.DoltDB{
				ObjectMeta: objMeta,
				Spec: doltv1alpha.DoltDBSpec{
					ServiceAccountName: ptr.To("custom-sa"),
				},
			},
			validate: func(t *testing.T, podSpec corev1.PodSpec) {
				if podSpec.ServiceAccountName != "custom-sa" {
					t.Errorf("ServiceAccountName should be custom-sa, got %s", podSpec.ServiceAccountName)
				}
			},
		},
		{
			name: "with image pull secrets",
			doltdb: &doltv1alpha.DoltDB{
				ObjectMeta: objMeta,
				Spec: doltv1alpha.DoltDBSpec{
					ImagePullSecrets: []corev1.LocalObjectReference{
						{Name: "secret1"},
						{Name: "secret2"},
					},
				},
			},
			validate: func(t *testing.T, podSpec corev1.PodSpec) {
				if len(podSpec.ImagePullSecrets) != 2 {
					t.Errorf("Expected 2 image pull secrets, got %d", len(podSpec.ImagePullSecrets))
				}
				if podSpec.ImagePullSecrets[0].Name != "secret1" {
					t.Errorf("Expected first secret to be secret1, got %s", podSpec.ImagePullSecrets[0].Name)
				}
			},
		},
		{
			name: "with node selector",
			doltdb: &doltv1alpha.DoltDB{
				ObjectMeta: objMeta,
				Spec: doltv1alpha.DoltDBSpec{
					NodeSelector: map[string]string{
						"disktype": "ssd",
					},
				},
			},
			validate: func(t *testing.T, podSpec corev1.PodSpec) {
				if podSpec.NodeSelector["disktype"] != "ssd" {
					t.Error("NodeSelector not properly set")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			podTpl := doltPodTemplate(objMeta, tt.doltdb, "test-hash")
			tt.validate(t, podTpl.Spec)
		})
	}
}

func TestDoltPodTemplateInitContainers(t *testing.T) {
	objMeta := metav1.ObjectMeta{
		Name: "test-doltdb-init-containers",
	}

	tests := []struct {
		name                 string
		doltdb               *doltv1alpha.DoltDB
		expectInitContainers bool
	}{
		{
			name: "basic doltdb",
			doltdb: &doltv1alpha.DoltDB{
				ObjectMeta: objMeta,
				Spec: doltv1alpha.DoltDBSpec{
					GlobalConfig: doltv1alpha.GlobalConfig{
						DisableClientUsageMetricsCollection: ptr.To(true),
					},
				},
			},
			expectInitContainers: true, // assuming doltInitContainers always returns some containers
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			podTpl := doltPodTemplate(objMeta, tt.doltdb, "test-hash")
			hasInitContainers := len(podTpl.Spec.InitContainers) > 0
			if hasInitContainers != tt.expectInitContainers {
				t.Errorf("Expected init containers: %v, got init containers: %v", tt.expectInitContainers, hasInitContainers)
			}

			doltConfigInitContainer := podTpl.Spec.InitContainers[0]
			if doltConfigInitContainer.Name != DoltInitContainerName {
				t.Errorf("Expected init container name %s, got %s", DoltInitContainerName, doltConfigInitContainer.Name)
			}

			if doltConfigInitContainer.Image != tt.doltdb.Spec.Image+":"+tt.doltdb.Spec.EngineVersion {
				t.Errorf(
					"Expected init container image %s:%s, got %s",
					tt.doltdb.Spec.Image,
					tt.doltdb.Spec.EngineVersion,
					doltConfigInitContainer.Image,
				)
			}
			if len(doltConfigInitContainer.Command) == 0 {
				t.Error("Expected init container to have commands")
			}

			expectedCmd := []string{"/bin/sh", "-c", `dolt config --global --add user.name "dolt kubernetes deployment"
dolt config --global --add user.email "dolt@kubernetes.deployment"
dolt config --global --add metrics.disabled true
cp /etc/doltdb/${POD_NAME}.yaml config.yaml

if [ -n "$DOLT_PASSWORD" -a ! -f .doltcfg/privileges.db ]; then
	dolt sql -q "create user '$DOLT_USERNAME' identified by '$DOLT_PASSWORD'; grant all privileges on *.* to '$DOLT_USERNAME' with grant option;"
fi
`}
			if !reflect.DeepEqual(doltConfigInitContainer.Command, expectedCmd) {
				t.Errorf("Expected init container command to be set, expected %s got  command %s", expectedCmd, doltConfigInitContainer.Command)
			}
			if doltConfigInitContainer.WorkingDir != DoltDataMountPath {
				t.Errorf("Expected init container working dir %s, got %s", DoltDataMountPath, doltConfigInitContainer.WorkingDir)
			}
			if len(doltConfigInitContainer.VolumeMounts) == 0 {
				t.Error("Expected init container to have volume mounts")
			}
		})
	}
}

func TestDoltPodTemplateVolumes(t *testing.T) {
	objMeta := metav1.ObjectMeta{
		Name: "test-doltdb-volumes",
	}

	doltdb := &doltv1alpha.DoltDB{
		ObjectMeta: objMeta,
	}

	podTpl := doltPodTemplate(objMeta, doltdb, "test-hash")

	// Check that expected volumes are present
	volumeNames := make(map[string]bool)
	for _, vol := range podTpl.Spec.Volumes {
		volumeNames[vol.Name] = true
	}

	expectedVolumes := []string{DoltConfigVolume, "serviceaccount"}
	for _, expectedVol := range expectedVolumes {
		if !volumeNames[expectedVol] {
			t.Errorf("Expected volume %s not found", expectedVol)
		}
	}
}

func TestDoltPodTemplateTerminationGracePeriod(t *testing.T) {
	objMeta := metav1.ObjectMeta{
		Name: "test-doltdb-termination",
	}

	tests := []struct {
		name     string
		doltdb   *doltv1alpha.DoltDB
		wantSecs int64
	}{
		{
			name: "nil defaults to 60",
			doltdb: &doltv1alpha.DoltDB{
				ObjectMeta: objMeta,
				Spec:       doltv1alpha.DoltDBSpec{},
			},
			wantSecs: 60,
		},
		{
			name: "custom value",
			doltdb: &doltv1alpha.DoltDB{
				ObjectMeta: objMeta,
				Spec: doltv1alpha.DoltDBSpec{
					TerminationGracePeriodSeconds: ptr.To(int64(120)),
				},
			},
			wantSecs: 120,
		},
		{
			name: "zero value",
			doltdb: &doltv1alpha.DoltDB{
				ObjectMeta: objMeta,
				Spec: doltv1alpha.DoltDBSpec{
					TerminationGracePeriodSeconds: ptr.To(int64(0)),
				},
			},
			wantSecs: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			podTpl := doltPodTemplate(objMeta, tt.doltdb, "test-hash")
			got := podTpl.Spec.TerminationGracePeriodSeconds
			if got == nil {
				t.Fatal("TerminationGracePeriodSeconds should not be nil")
			}
			if *got != tt.wantSecs {
				t.Errorf("TerminationGracePeriodSeconds = %d, want %d", *got, tt.wantSecs)
			}
		})
	}
}

func TestDoltLifecycle(t *testing.T) {
	tests := []struct {
		name     string
		doltdb   *doltv1alpha.DoltDB
		wantNil  bool
		wantPort int
	}{
		{
			name: "replication enabled includes lifecycle",
			doltdb: &doltv1alpha.DoltDB{
				Spec: doltv1alpha.DoltDBSpec{
					Replication: &doltv1alpha.Replication{
						Enabled: true,
					},
					Server: doltv1alpha.Server{
						Listener: doltv1alpha.Listener{Port: 3306},
					},
				},
			},
			wantNil:  false,
			wantPort: 3306,
		},
		{
			name: "replication disabled excludes lifecycle",
			doltdb: &doltv1alpha.DoltDB{
				Spec: doltv1alpha.DoltDBSpec{
					Replication: &doltv1alpha.Replication{
						Enabled: false,
					},
				},
			},
			wantNil: true,
		},
		{
			name: "nil replication excludes lifecycle",
			doltdb: &doltv1alpha.DoltDB{
				Spec: doltv1alpha.DoltDBSpec{},
			},
			wantNil: true,
		},
		{
			name: "custom port in lifecycle",
			doltdb: &doltv1alpha.DoltDB{
				Spec: doltv1alpha.DoltDBSpec{
					Replication: &doltv1alpha.Replication{
						Enabled: true,
					},
					Server: doltv1alpha.Server{
						Listener: doltv1alpha.Listener{Port: 3307},
					},
				},
			},
			wantNil:  false,
			wantPort: 3307,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			containers := doltContainers(tt.doltdb)
			if len(containers) == 0 {
				t.Fatal("expected at least one container")
			}
			lifecycle := containers[0].Lifecycle
			if tt.wantNil {
				if lifecycle != nil {
					t.Error("expected lifecycle to be nil")
				}
				return
			}
			if lifecycle == nil {
				t.Fatal("expected lifecycle to not be nil")
			}
			if lifecycle.PreStop == nil {
				t.Fatal("expected preStop to not be nil")
			}
			if lifecycle.PreStop.Exec == nil {
				t.Fatal("expected preStop.exec to not be nil")
			}
			cmd := lifecycle.PreStop.Exec.Command
			if len(cmd) != 3 || cmd[0] != "/bin/sh" || cmd[1] != "-c" {
				t.Errorf("expected shell command, got %v", cmd)
			}
			// Verify the port appears in the command
			portStr := fmt.Sprintf("--port %d", tt.wantPort)
			if !contains(cmd[2], portStr) {
				t.Errorf("expected command to contain %q, got %q", portStr, cmd[2])
			}
		})
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && searchString(s, substr)
}

func searchString(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// TODO: we should test all other things like volumes, init containers, etc
