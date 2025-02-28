package dolt

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	doltv1alpha "github.com/electronicarts/doltdb-operator/api/v1alpha"
	"gopkg.in/yaml.v2"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestGenerateConfigMapData(t *testing.T) {
	tests := []struct {
		name          string
		doltdb        *doltv1alpha.DoltDB
		expectedData  map[string]interface{}
		expectedError bool
	}{
		{
			name: "default max connections",
			doltdb: &doltv1alpha.DoltDB{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-cluster",
					Namespace: "default",
				},
				Spec: doltv1alpha.DoltDBSpec{
					Replicas: 2,
					Server: doltv1alpha.Server{
						Listener: doltv1alpha.Listener{
							Host:           "0.0.0.0",
							Port:           3306,
							MaxConnections: 128,
						},
						Cluster: doltv1alpha.Cluster{
							RemotesAPI: doltv1alpha.RemotesAPI{
								Port: 50051,
							},
						},
						LogLevel: "trace",
					},
				},
			},
			expectedData:  readTestData(t, "default_max_conn.yaml"),
			expectedError: false,
		},
		{
			name: "custom max connections",
			doltdb: &doltv1alpha.DoltDB{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-cluster",
					Namespace: "default",
				},
				Spec: doltv1alpha.DoltDBSpec{
					Replicas: 1,
					Server: doltv1alpha.Server{
						Listener: doltv1alpha.Listener{
							Host:           "0.0.0.0",
							Port:           3306,
							MaxConnections: 200,
						},
						Cluster: doltv1alpha.Cluster{
							RemotesAPI: doltv1alpha.RemotesAPI{
								Port: 50051,
							},
						},
						LogLevel: "trace",
					},
				},
			},
			expectedData:  readTestData(t, "custom_max_conn.yaml"),
			expectedError: false,
		},
		{
			name: "with metrics server config",
			doltdb: &doltv1alpha.DoltDB{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-cluster",
					Namespace: "default",
				},
				Spec: doltv1alpha.DoltDBSpec{
					Replicas: 2,
					Server: doltv1alpha.Server{
						Listener: doltv1alpha.Listener{
							Host:           "0.0.0.0",
							Port:           3306,
							MaxConnections: 128,
						},
						Cluster: doltv1alpha.Cluster{
							RemotesAPI: doltv1alpha.RemotesAPI{
								Port: 50051,
							},
						},
						LogLevel: "trace",
						Metrics: &doltv1alpha.Metrics{
							Enabled: true,
							Host:    "0.0.0.0",
							Labels: map[string]string{
								"doltdb_instance": "doltdb-dev",
							},
							Port: 9092,
						},
					},
				},
			},
			expectedData:  readTestData(t, "metrics_server_config.yaml"),
			expectedError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data, err := GenerateConfigMapData(tt.doltdb)
			if err != nil {
				if !tt.expectedError {
					t.Fatalf("unexpected error, got: %v", err)
				}
				return
			}

			for key, expectedValue := range tt.expectedData {
				var expectedObj, actualObj Config

				expectedStr, err := yaml.Marshal(expectedValue)
				if err != nil {
					t.Fatalf("failed to marshal expected value for key %s: %v", key, err)
				}
				if err := yaml.Unmarshal(expectedStr, &expectedObj); err != nil {
					t.Fatalf("failed to unmarshal expected data for key %s: %v", key, err)
				}
				if err := yaml.Unmarshal([]byte(data[key]), &actualObj); err != nil {
					t.Fatalf("failed to unmarshal actual data for key %s: %v", key, err)
				}
				if !reflect.DeepEqual(expectedObj, actualObj) {
					t.Errorf("expected %v, got %v", expectedObj, actualObj)
				}
			}
		})
	}
}

func readTestData(t *testing.T, path string) map[string]interface{} {
	data, err := os.ReadFile(filepath.Join("testdata", path))
	if err != nil {
		t.Fatalf("failed to read test data: %v", err)
	}

	var result map[string]interface{}
	if err := yaml.Unmarshal(data, &result); err != nil {
		t.Fatalf("failed to unmarshal test data: %v", err)
	}

	return result
}
