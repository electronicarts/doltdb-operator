package dolt

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	doltv1alpha "github.com/electronicarts/doltdb-operator/api/v1alpha"
	"gopkg.in/yaml.v2"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestGenerateConfigMapData(t *testing.T) {
	tests := []struct {
		name          string
		doltCluster   *doltv1alpha.DoltCluster
		expectedData  map[string]string
		expectedError bool
	}{
		{
			name: "default max connections",
			doltCluster: &doltv1alpha.DoltCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-cluster",
					Namespace: "default",
				},
				Spec: doltv1alpha.DoltClusterSpec{
					Replicas: 2,
				},
			},
			expectedData: map[string]string{
				"test-cluster-0.yaml": `log_level: trace
cluster:
  bootstrap_epoch: 1
  bootstrap_role: primary
  standby_remotes:
  - name: test-cluster-1
    remote_url_template: http://test-cluster-1.test-cluster-internal:50051/{database}
listener:
  host: 0.0.0.0
  max_connections: 128
  port: 3306
remotesapi:
  port: 50051
`,
				"test-cluster-1.yaml": `log_level: trace
cluster:
  bootstrap_epoch: 1
  bootstrap_role: standby
  standby_remotes:
  - name: test-cluster-0
    remote_url_template: http://test-cluster-0.test-cluster-internal:50051/{database}
listener:
  host: 0.0.0.0
  max_connections: 128
  port: 3306
remotesapi:
  port: 50051
`,
			},
			expectedError: false,
		},
		{
			name: "custom max connections",
			doltCluster: &doltv1alpha.DoltCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-cluster",
					Namespace: "default",
				},
				Spec: doltv1alpha.DoltClusterSpec{
					Replicas:       1,
					MaxConnections: int32Ptr(200),
				},
			},
			expectedData: map[string]string{
				"test-cluster-0.yaml": `cluster:
  bootstrap_epoch: 1
  bootstrap_role: primary
  standby_remotes: []
listener:
  host: 0.0.0.0
  max_connections: 200
  port: 3306
log_level: trace
remotesapi:
  port: 50051
`,
			},
			expectedError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data, err := GenerateConfigMapData(tt.doltCluster)
			if err != nil {
				if !tt.expectedError {
					t.Fatalf("unexpected error, got: %v", err)
				}
				return
			}

			for key, expectedValue := range tt.expectedData {
				var expectedObj, actualObj Config
				if err := yaml.Unmarshal([]byte(expectedValue), &expectedObj); err != nil {
					t.Fatalf("failed to unmarshal expected data for key %s: %v", key, err)
				}
				if err := yaml.Unmarshal([]byte(data[key]), &actualObj); err != nil {
					t.Fatalf("failed to unmarshal actual data for key %s: %v", key, err)
				}
				if !cmp.Equal(expectedObj, actualObj) {
					t.Errorf("expected %v, got %v", expectedObj, actualObj)
				}
			}
		})
	}
}

func int32Ptr(i int32) *int32 {
	return &i
}
