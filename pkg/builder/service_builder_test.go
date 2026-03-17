// Copyright (c) 2025 Electronic Arts Inc. All rights reserved.

package builder

import (
	"testing"

	doltv1alpha "github.com/electronicarts/doltdb-operator/api/v1alpha"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
)

func TestDoltServicePorts(t *testing.T) {
	tests := []struct {
		name          string
		doltdb        *doltv1alpha.DoltDB
		expectedPorts []corev1.ServicePort
	}{
		{
			name: "default ports",
			doltdb: &doltv1alpha.DoltDB{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "doltdb-test",
					Namespace: "default",
				},
				Spec: doltv1alpha.DoltDBSpec{
					Server: doltv1alpha.Server{
						Listener: doltv1alpha.Listener{
							Port: 3306,
						},
						Cluster: doltv1alpha.Cluster{
							RemotesAPI: doltv1alpha.RemotesAPI{
								Port: 50051,
							},
						},
					},
				},
			},
			expectedPorts: []corev1.ServicePort{
				{
					Port: 3306,
					Name: DoltMySQLPortName,
				},
			},
		},
		{
			name: "custom ports",
			doltdb: &doltv1alpha.DoltDB{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "doltdb-custom",
					Namespace: "default",
				},
				Spec: doltv1alpha.DoltDBSpec{
					Server: doltv1alpha.Server{
						Listener: doltv1alpha.Listener{
							Port: 13306,
						},
						Cluster: doltv1alpha.Cluster{
							RemotesAPI: doltv1alpha.RemotesAPI{
								Port: 15051,
							},
						},
					},
				},
			},
			expectedPorts: []corev1.ServicePort{
				{
					Port: 13306,
					Name: DoltMySQLPortName,
				},
			},
		},
		{
			name: "with MCP server port",
			doltdb: &doltv1alpha.DoltDB{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "doltdb-mcp",
					Namespace: "default",
				},
				Spec: doltv1alpha.DoltDBSpec{
					Server: doltv1alpha.Server{
						Listener: doltv1alpha.Listener{
							Port: 3306,
						},
						Cluster: doltv1alpha.Cluster{
							RemotesAPI: doltv1alpha.RemotesAPI{
								Port: 50051,
							},
						},
						MCPServer: &doltv1alpha.MCPServer{
							Port: 7007,
						},
					},
				},
			},
			expectedPorts: []corev1.ServicePort{
				{
					Port: 3306,
					Name: DoltMySQLPortName,
				},
				{
					Port: 7007,
					Name: DoltMCPPortName,
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ports := doltServicePorts(tt.doltdb)

			if len(ports) != len(tt.expectedPorts) {
				t.Fatalf("expected %d ports, got %d", len(tt.expectedPorts), len(ports))
			}

			for i, expectedPort := range tt.expectedPorts {
				if ports[i].Port != expectedPort.Port {
					t.Errorf("port %d: expected port number %d, got %d", i, expectedPort.Port, ports[i].Port)
				}
				if ports[i].Name != expectedPort.Name {
					t.Errorf("port %d: expected port name %s, got %s", i, expectedPort.Name, ports[i].Name)
				}
			}

			// Verify MySQL port name
			if len(ports) > 0 {
				mysqlPort := ports[0]
				if mysqlPort.Name != DoltMySQLPortName {
					t.Errorf("expected MySQL port name to be %s, got %s", DoltMySQLPortName, mysqlPort.Name)
				}
			}
		})
	}
}

func TestDoltInternalServicePorts(t *testing.T) {
	tests := []struct {
		name          string
		doltdb        *doltv1alpha.DoltDB
		expectedPorts []corev1.ServicePort
	}{
		{
			name: "default ports",
			doltdb: &doltv1alpha.DoltDB{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "doltdb-test",
					Namespace: "default",
				},
				Spec: doltv1alpha.DoltDBSpec{
					Server: doltv1alpha.Server{
						Listener: doltv1alpha.Listener{
							Port: 3306,
						},
						Cluster: doltv1alpha.Cluster{
							RemotesAPI: doltv1alpha.RemotesAPI{
								Port: 50051,
							},
						},
					},
				},
			},
			expectedPorts: []corev1.ServicePort{
				{
					Port: 3306,
					Name: DoltMySQLPortName,
				},
				{
					Port: 50051,
					Name: DoltRemotesAPIPortName,
				},
			},
		},
		{
			name: "custom ports",
			doltdb: &doltv1alpha.DoltDB{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "doltdb-custom",
					Namespace: "default",
				},
				Spec: doltv1alpha.DoltDBSpec{
					Server: doltv1alpha.Server{
						Listener: doltv1alpha.Listener{
							Port: 13306,
						},
						Cluster: doltv1alpha.Cluster{
							RemotesAPI: doltv1alpha.RemotesAPI{
								Port: 15051,
							},
						},
					},
				},
			},
			expectedPorts: []corev1.ServicePort{
				{
					Port: 13306,
					Name: DoltMySQLPortName,
				},
				{
					Port: 15051,
					Name: DoltRemotesAPIPortName,
				},
			},
		},
		{
			name: "with MCP server port",
			doltdb: &doltv1alpha.DoltDB{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "doltdb-mcp",
					Namespace: "default",
				},
				Spec: doltv1alpha.DoltDBSpec{
					Server: doltv1alpha.Server{
						Listener: doltv1alpha.Listener{
							Port: 3306,
						},
						Cluster: doltv1alpha.Cluster{
							RemotesAPI: doltv1alpha.RemotesAPI{
								Port: 50051,
							},
						},
						MCPServer: &doltv1alpha.MCPServer{
							Port: 7007,
						},
					},
				},
			},
			expectedPorts: []corev1.ServicePort{
				{
					Port: 3306,
					Name: DoltMySQLPortName,
				},
				{
					Port: 50051,
					Name: DoltRemotesAPIPortName,
				},
				{
					Port: 7007,
					Name: DoltMCPPortName,
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ports := doltInternalServicePorts(tt.doltdb)

			if len(ports) != len(tt.expectedPorts) {
				t.Fatalf("expected %d ports, got %d", len(tt.expectedPorts), len(ports))
			}

			for i, expectedPort := range tt.expectedPorts {
				if ports[i].Port != expectedPort.Port {
					t.Errorf("port %d: expected port number %d, got %d", i, expectedPort.Port, ports[i].Port)
				}
				if ports[i].Name != expectedPort.Name {
					t.Errorf("port %d: expected port name %s, got %s", i, expectedPort.Name, ports[i].Name)
				}
			}

			// Verify specific port names are correct
			mysqlPort := ports[0]
			if mysqlPort.Name != DoltMySQLPortName {
				t.Errorf("expected MySQL port name to be %s, got %s", DoltMySQLPortName, mysqlPort.Name)
			}

			remotesAPIPort := ports[1]
			if remotesAPIPort.Name != DoltRemotesAPIPortName {
				t.Errorf("expected RemotesAPI port name to be %s, got %s", DoltRemotesAPIPortName, remotesAPIPort.Name)
			}
		})
	}
}

func TestBuildDoltInternalService(t *testing.T) {
	builder := newTestBuilder()

	tests := []struct {
		name                string
		doltdb              *doltv1alpha.DoltDB
		expectedPortsCount  int
		expectedClusterIP   string
		expectedServiceType corev1.ServiceType
	}{
		{
			name: "headless service with default ports",
			doltdb: &doltv1alpha.DoltDB{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "doltdb-internal",
					Namespace: "default",
				},
				Spec: doltv1alpha.DoltDBSpec{
					Server: doltv1alpha.Server{
						Listener: doltv1alpha.Listener{
							Port: 3306,
						},
						Cluster: doltv1alpha.Cluster{
							RemotesAPI: doltv1alpha.RemotesAPI{
								Port: 50051,
							},
						},
					},
				},
			},
			expectedPortsCount: 2,
			expectedClusterIP:  "None",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc, err := builder.BuildDoltInternalService(tt.doltdb)
			if err != nil {
				t.Fatalf("unexpected error building internal service: %v", err)
			}

			if len(svc.Spec.Ports) != tt.expectedPortsCount {
				t.Errorf("expected %d ports, got %d", tt.expectedPortsCount, len(svc.Spec.Ports))
			}

			if svc.Spec.ClusterIP != tt.expectedClusterIP {
				t.Errorf("expected ClusterIP %s, got %s", tt.expectedClusterIP, svc.Spec.ClusterIP)
			}

			// Verify port names
			hasMySQL := false
			hasRemotesAPI := false
			for _, port := range svc.Spec.Ports {
				if port.Name == DoltMySQLPortName {
					hasMySQL = true
				}
				if port.Name == DoltRemotesAPIPortName {
					hasRemotesAPI = true
				}
			}

			if !hasMySQL {
				t.Errorf("service missing MySQL port with name %s", DoltMySQLPortName)
			}
			if !hasRemotesAPI {
				t.Errorf("service missing RemotesAPI port with name %s", DoltRemotesAPIPortName)
			}
		})
	}
}

func TestBuildDoltPrimaryService(t *testing.T) {
	builder := newTestBuilder()

	tests := []struct {
		name                string
		doltdb              *doltv1alpha.DoltDB
		expectedPortsCount  int
		expectedServiceType corev1.ServiceType
	}{
		{
			name: "primary service with default ports",
			doltdb: &doltv1alpha.DoltDB{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "doltdb-primary",
					Namespace: "default",
				},
				Spec: doltv1alpha.DoltDBSpec{
					Server: doltv1alpha.Server{
						Listener: doltv1alpha.Listener{
							Port: 3306,
						},
						Cluster: doltv1alpha.Cluster{
							RemotesAPI: doltv1alpha.RemotesAPI{
								Port: 50051,
							},
						},
					},
				},
				Status: doltv1alpha.DoltDBStatus{
					CurrentPrimaryPodIndex: ptr.To(0),
				},
			},
			expectedPortsCount:  1,
			expectedServiceType: corev1.ServiceTypeClusterIP,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc, err := builder.BuildDoltPrimaryService(tt.doltdb)
			if err != nil {
				t.Fatalf("unexpected error building primary service: %v", err)
			}

			if len(svc.Spec.Ports) != tt.expectedPortsCount {
				t.Errorf("expected %d ports, got %d", tt.expectedPortsCount, len(svc.Spec.Ports))
			}

			if svc.Spec.Type != tt.expectedServiceType {
				t.Errorf("expected service type %s, got %s", tt.expectedServiceType, svc.Spec.Type)
			}

			// Verify port names
			hasMySQL := false
			for _, port := range svc.Spec.Ports {
				if port.Name == DoltMySQLPortName {
					hasMySQL = true
				}
			}

			if !hasMySQL {
				t.Errorf("service missing MySQL port with name %s", DoltMySQLPortName)
			}
		})
	}
}

func TestBuildDoltService(t *testing.T) {
	builder := newTestBuilder()

	tests := []struct {
		name                string
		doltdb              *doltv1alpha.DoltDB
		expectedPortsCount  int
		expectedServiceType corev1.ServiceType
		expectedName        string
	}{
		{
			name: "standalone service with default ports",
			doltdb: &doltv1alpha.DoltDB{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "doltdb-standalone",
					Namespace: "default",
				},
				Spec: doltv1alpha.DoltDBSpec{
					Replicas: 1,
					Server: doltv1alpha.Server{
						Listener: doltv1alpha.Listener{
							Port: 3306,
						},
					},
				},
			},
			expectedPortsCount:  1,
			expectedServiceType: corev1.ServiceTypeClusterIP,
			expectedName:        "doltdb-standalone",
		},
		{
			name: "standalone service with custom port",
			doltdb: &doltv1alpha.DoltDB{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "doltdb-custom",
					Namespace: "default",
				},
				Spec: doltv1alpha.DoltDBSpec{
					Replicas: 1,
					Server: doltv1alpha.Server{
						Listener: doltv1alpha.Listener{
							Port: 13306,
						},
					},
				},
			},
			expectedPortsCount:  1,
			expectedServiceType: corev1.ServiceTypeClusterIP,
			expectedName:        "doltdb-custom",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc, err := builder.BuildDoltService(tt.doltdb)
			if err != nil {
				t.Fatalf("unexpected error building standalone service: %v", err)
			}

			// Service name should match DoltDB name (no suffix)
			if svc.Name != tt.expectedName {
				t.Errorf("expected service name %s, got %s", tt.expectedName, svc.Name)
			}

			if len(svc.Spec.Ports) != tt.expectedPortsCount {
				t.Errorf("expected %d ports, got %d", tt.expectedPortsCount, len(svc.Spec.Ports))
			}

			if svc.Spec.Type != tt.expectedServiceType {
				t.Errorf("expected service type %s, got %s", tt.expectedServiceType, svc.Spec.Type)
			}

			// Verify MySQL port
			hasMySQL := false
			for _, port := range svc.Spec.Ports {
				if port.Name == DoltMySQLPortName {
					hasMySQL = true
					if port.Port != tt.doltdb.Spec.Server.Listener.Port {
						t.Errorf("expected MySQL port %d, got %d", tt.doltdb.Spec.Server.Listener.Port, port.Port)
					}
				}
			}
			if !hasMySQL {
				t.Errorf("service missing MySQL port with name %s", DoltMySQLPortName)
			}

			// Verify selector uses app labels only (no role-specific labels)
			expectedLabels := NewLabelsBuilder().WithDoltSelectorLabels(tt.doltdb).Build()
			for k, v := range expectedLabels {
				if svc.Spec.Selector[k] != v {
					t.Errorf("expected selector label %s=%s, got %s", k, v, svc.Spec.Selector[k])
				}
			}

			// Ensure no role labels in selector
			if _, hasRole := svc.Spec.Selector["k8s.dolthub.com/cluster-role"]; hasRole {
				t.Error("standalone service selector should not contain role label")
			}
		})
	}
}

func TestBuildDoltReaderService(t *testing.T) {
	builder := newTestBuilder()

	tests := []struct {
		name                string
		doltdb              *doltv1alpha.DoltDB
		expectedPortsCount  int
		expectedServiceType corev1.ServiceType
	}{
		{
			name: "reader service with default ports",
			doltdb: &doltv1alpha.DoltDB{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "doltdb-reader",
					Namespace: "default",
				},
				Spec: doltv1alpha.DoltDBSpec{
					Server: doltv1alpha.Server{
						Listener: doltv1alpha.Listener{
							Port: 3306,
						},
						Cluster: doltv1alpha.Cluster{
							RemotesAPI: doltv1alpha.RemotesAPI{
								Port: 50051,
							},
						},
					},
				},
			},
			expectedPortsCount:  1,
			expectedServiceType: corev1.ServiceTypeClusterIP,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc, err := builder.BuildDoltReaderService(tt.doltdb)
			if err != nil {
				t.Fatalf("unexpected error building reader service: %v", err)
			}

			if len(svc.Spec.Ports) != tt.expectedPortsCount {
				t.Errorf("expected %d ports, got %d", tt.expectedPortsCount, len(svc.Spec.Ports))
			}

			if svc.Spec.Type != tt.expectedServiceType {
				t.Errorf("expected service type %s, got %s", tt.expectedServiceType, svc.Spec.Type)
			}

			// Verify port names
			hasMySQL := false
			for _, port := range svc.Spec.Ports {
				if port.Name == DoltMySQLPortName {
					hasMySQL = true
				}
			}

			if !hasMySQL {
				t.Errorf("service missing MySQL port with name %s", DoltMySQLPortName)
			}
		})
	}
}
