// Copyright (c) 2025 Electronic Arts Inc. All rights reserved.

package dolt

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	doltv1alpha "github.com/electronicarts/doltdb-operator/api/v1alpha"
	"gopkg.in/yaml.v3"
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
					Replication: &doltv1alpha.Replication{
						Enabled: true,
					},
					Server: doltv1alpha.Server{
						Behavior: doltv1alpha.Behavior{
							AutoGCBehavior: doltv1alpha.AutoGCBehavior{
								Enable: true,
							},
						},
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
			name: "single instance without replication",
			doltdb: &doltv1alpha.DoltDB{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-cluster",
					Namespace: "default",
				},
				Spec: doltv1alpha.DoltDBSpec{
					Replicas: 1,
					Server: doltv1alpha.Server{
						Behavior: doltv1alpha.Behavior{
							AutoGCBehavior: doltv1alpha.AutoGCBehavior{
								Enable: true,
							},
						},
						Listener: doltv1alpha.Listener{
							Host:           "0.0.0.0",
							Port:           3306,
							MaxConnections: 128,
						},
						LogLevel: "trace",
					},
				},
			},
			expectedData:  readTestData(t, "single_instance.yaml"),
			expectedError: false,
		},
		{
			name: "replicated two nodes with cluster section",
			doltdb: &doltv1alpha.DoltDB{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-cluster",
					Namespace: "default",
				},
				Spec: doltv1alpha.DoltDBSpec{
					Replicas: 2,
					Replication: &doltv1alpha.Replication{
						Enabled: true,
					},
					Server: doltv1alpha.Server{
						Behavior: doltv1alpha.Behavior{
							AutoGCBehavior: doltv1alpha.AutoGCBehavior{
								Enable: true,
							},
						},
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
			expectedData:  readTestData(t, "replicated_two_nodes.yaml"),
			expectedError: false,
		},
		{
			name: "with maybe null server config values",
			doltdb: &doltv1alpha.DoltDB{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-cluster",
					Namespace: "default",
				},
				Spec: doltv1alpha.DoltDBSpec{
					Replicas: 2,
					Replication: &doltv1alpha.Replication{
						Enabled: true,
					},
					Server: doltv1alpha.Server{
						Listener: doltv1alpha.Listener{
							Host:           "0.0.0.0",
							Port:           3306,
							MaxConnections: 128,
						},
						Metrics: &doltv1alpha.Metrics{
							Enabled: true,
							Host:    "0.0.0.0",
							Labels: map[string]string{
								"doltdb_instance": "doltdb-dev",
							},
							Port: 9092,
						},
						LogLevel: "trace",
					},
				},
			},
			expectedData:  readTestData(t, "null_server_config.yaml"),
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
					Replication: &doltv1alpha.Replication{
						Enabled: true,
					},
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
		{
			name: "with MCP server custom user",
			doltdb: newTestDoltDBWithMCP(&doltv1alpha.MCPServer{
				Port:     7007,
				User:     "mcp-agent",
				Database: "testdb",
				PasswordSecretKeyRef: &doltv1alpha.SecretKeySelector{
					LocalObjectReference: doltv1alpha.LocalObjectReference{Name: "mcp-creds"},
					Key:                  "password",
				},
			}),
			expectedData:  readTestData(t, "mcp_server_config.yaml"),
			expectedError: false,
		},
		{
			name: "with MCP server default credentials",
			doltdb: newTestDoltDBWithMCP(&doltv1alpha.MCPServer{
				Port: 7007,
			}),
			expectedData:  readTestData(t, "mcp_server_default_creds.yaml"),
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

func TestSingleInstanceConfigNoClusterSection(t *testing.T) {
	doltdb := &doltv1alpha.DoltDB{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-cluster",
			Namespace: "default",
		},
		Spec: doltv1alpha.DoltDBSpec{
			Replicas: 1,
			Server: doltv1alpha.Server{
				Listener: doltv1alpha.Listener{
					Host: "0.0.0.0",
					Port: 3306,
				},
				LogLevel: "trace",
			},
		},
	}

	data, err := GenerateConfigMapData(doltdb)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(data) != 1 {
		t.Fatalf("expected 1 config key, got %d", len(data))
	}

	configYAML, ok := data["test-cluster-0.yaml"]
	if !ok {
		t.Fatal("expected config key 'test-cluster-0.yaml' not found")
	}

	if strings.Contains(configYAML, "cluster:") {
		t.Error("single instance config should not contain 'cluster:' section")
	}
	if !strings.Contains(configYAML, "listener:") {
		t.Error("config should contain 'listener:' section")
	}
	if !strings.Contains(configYAML, "behavior:") {
		t.Error("config should contain 'behavior:' section")
	}
}

func TestReplicatedInstanceConfigHasClusterSection(t *testing.T) {
	doltdb := &doltv1alpha.DoltDB{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-cluster",
			Namespace: "default",
		},
		Spec: doltv1alpha.DoltDBSpec{
			Replicas: 2,
			Replication: &doltv1alpha.Replication{
				Enabled: true,
			},
			Server: doltv1alpha.Server{
				Listener: doltv1alpha.Listener{
					Host: "0.0.0.0",
					Port: 3306,
				},
				Cluster: doltv1alpha.Cluster{
					RemotesAPI: doltv1alpha.RemotesAPI{
						Port: 50051,
					},
				},
				LogLevel: "trace",
			},
		},
	}

	data, err := GenerateConfigMapData(doltdb)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(data) != 2 {
		t.Fatalf("expected 2 config keys, got %d", len(data))
	}

	for key, configYAML := range data {
		if !strings.Contains(configYAML, "cluster:") {
			t.Errorf("replicated config %s should contain 'cluster:' section", key)
		}
		if !strings.Contains(configYAML, "standby_remotes:") {
			t.Errorf("replicated config %s should contain 'standby_remotes:' section", key)
		}
	}
}

func TestConfigClusterOmitEmpty(t *testing.T) {
	// When Cluster is nil, YAML output should not contain "cluster:"
	config := Config{
		Behavior: Behavior{AutoGCBehavior: AutoGCBehavior{Enable: false}},
		LogLevel: "trace",
		Listener: Listener{Host: "0.0.0.0", Port: 3306, MaxConnections: 128},
	}
	yamlData, err := yaml.Marshal(config)
	if err != nil {
		t.Fatalf("unexpected error marshaling config: %v", err)
	}
	if strings.Contains(string(yamlData), "cluster:") {
		t.Error("config with nil Cluster should not contain 'cluster:' in YAML output")
	}

	// When Cluster is set, YAML output should contain "cluster:"
	configWithCluster := Config{
		Behavior: Behavior{AutoGCBehavior: AutoGCBehavior{Enable: false}},
		LogLevel: "trace",
		Cluster: &Cluster{
			BootstrapEpoch: 1,
			BootstrapRole:  "primary",
			RemotesAPI:     RemotesAPI{Port: 50051},
		},
		Listener: Listener{Host: "0.0.0.0", Port: 3306, MaxConnections: 128},
	}
	yamlDataWithCluster, err := yaml.Marshal(configWithCluster)
	if err != nil {
		t.Fatalf("unexpected error marshaling config: %v", err)
	}
	if !strings.Contains(string(yamlDataWithCluster), "cluster:") {
		t.Error("config with Cluster set should contain 'cluster:' in YAML output")
	}
}

func TestConfigMCPServerOmitEmpty(t *testing.T) {
	config := Config{
		Behavior: Behavior{AutoGCBehavior: AutoGCBehavior{Enable: false}},
		LogLevel: "trace",
		Listener: Listener{Host: "0.0.0.0", Port: 3306, MaxConnections: 128},
	}
	yamlData, err := yaml.Marshal(config)
	if err != nil {
		t.Fatalf("unexpected error marshaling config: %v", err)
	}
	if strings.Contains(string(yamlData), "mcp_server:") {
		t.Error("config with nil MCPServer should not contain 'mcp_server:' in YAML output")
	}

	configWithMCP := Config{
		Behavior: Behavior{AutoGCBehavior: AutoGCBehavior{Enable: false}},
		LogLevel: "trace",
		Listener: Listener{Host: "0.0.0.0", Port: 3306, MaxConnections: 128},
		MCPServer: &MCPServer{
			Port:     7007,
			User:     "${DOLT_USERNAME}",
			Password: "${DOLT_PASSWORD}",
		},
	}
	yamlDataWithMCP, err := yaml.Marshal(configWithMCP)
	if err != nil {
		t.Fatalf("unexpected error marshaling config: %v", err)
	}
	if !strings.Contains(string(yamlDataWithMCP), "mcp_server:") {
		t.Error("config with MCPServer set should contain 'mcp_server:' in YAML output")
	}
}

func newTestDoltDBWithMCP(mcp *doltv1alpha.MCPServer) *doltv1alpha.DoltDB {
	return &doltv1alpha.DoltDB{
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
					MaxConnections: 128,
				},
				LogLevel:  "trace",
				MCPServer: mcp,
			},
		},
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
