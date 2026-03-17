// Copyright (c) 2025 Electronic Arts Inc. All rights reserved.

package builder

import (
	"testing"

	doltv1alpha "github.com/electronicarts/doltdb-operator/api/v1alpha"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestDoltEnvMCP(t *testing.T) {
	baseDoltDB := func(mcp *doltv1alpha.MCPServer) *doltv1alpha.DoltDB {
		return &doltv1alpha.DoltDB{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-doltdb",
				Namespace: "default",
			},
			Spec: doltv1alpha.DoltDBSpec{
				Server: doltv1alpha.Server{
					MCPServer: mcp,
				},
			},
		}
	}

	tests := []struct {
		name              string
		doltdb            *doltv1alpha.DoltDB
		expectMCPUser     bool
		mcpUserValue      string
		expectMCPPassword bool
		mcpPasswordSecret string
	}{
		{
			name:              "MCP disabled - no MCP env vars",
			doltdb:            baseDoltDB(nil),
			expectMCPUser:     false,
			expectMCPPassword: false,
		},
		{
			name: "MCP with custom user and password secret",
			doltdb: baseDoltDB(&doltv1alpha.MCPServer{
				Port: 7007,
				User: "mcp-agent",
				PasswordSecretKeyRef: &doltv1alpha.SecretKeySelector{
					LocalObjectReference: doltv1alpha.LocalObjectReference{Name: "mcp-creds"},
					Key:                  "password",
				},
			}),
			expectMCPUser:     true,
			mcpUserValue:      "mcp-agent",
			expectMCPPassword: true,
			mcpPasswordSecret: "mcp-creds",
		},
		{
			name: "MCP with default credentials - no custom user, root password fallback",
			doltdb: baseDoltDB(&doltv1alpha.MCPServer{
				Port: 7007,
			}),
			expectMCPUser:     false,
			expectMCPPassword: true,
			mcpPasswordSecret: "test-doltdb-credentials",
		},
		{
			name: "MCP with custom user only - no password secret, root password fallback",
			doltdb: baseDoltDB(&doltv1alpha.MCPServer{
				Port: 7007,
				User: "readonly-agent",
			}),
			expectMCPUser:     true,
			mcpUserValue:      "readonly-agent",
			expectMCPPassword: true,
			mcpPasswordSecret: "test-doltdb-credentials",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			env := doltEnv(tt.doltdb)

			var mcpUser, mcpPassword *string
			var mcpPasswordSecretName string
			for _, e := range env {
				if e.Name == "DOLT_MCP_USER" {
					v := e.Value
					mcpUser = &v
				}
				if e.Name == "DOLT_MCP_PASSWORD" {
					empty := ""
					mcpPassword = &empty
					if e.ValueFrom != nil && e.ValueFrom.SecretKeyRef != nil {
						mcpPasswordSecretName = e.ValueFrom.SecretKeyRef.Name
					}
				}
			}

			if tt.expectMCPUser {
				if mcpUser == nil {
					t.Fatal("expected DOLT_MCP_USER env var, not found")
				}
				if *mcpUser != tt.mcpUserValue {
					t.Errorf("DOLT_MCP_USER: expected %q, got %q", tt.mcpUserValue, *mcpUser)
				}
			} else if mcpUser != nil {
				t.Errorf("unexpected DOLT_MCP_USER env var with value %q", *mcpUser)
			}

			if tt.expectMCPPassword {
				if mcpPassword == nil {
					t.Fatal("expected DOLT_MCP_PASSWORD env var, not found")
				}
				if mcpPasswordSecretName != tt.mcpPasswordSecret {
					t.Errorf(
						"DOLT_MCP_PASSWORD secret: expected %q, got %q",
						tt.mcpPasswordSecret, mcpPasswordSecretName,
					)
				}
			} else if mcpPassword != nil {
				t.Error("unexpected DOLT_MCP_PASSWORD env var")
			}
		})
	}
}

func TestDoltContainerPortsMCP(t *testing.T) {
	tests := []struct {
		name       string
		doltdb     *doltv1alpha.DoltDB
		expectMCP  bool
		mcpPortNum int32
	}{
		{
			name: "MCP disabled - no MCP port",
			doltdb: &doltv1alpha.DoltDB{
				Spec: doltv1alpha.DoltDBSpec{
					Server: doltv1alpha.Server{
						Listener: doltv1alpha.Listener{Port: 3306},
					},
				},
			},
			expectMCP: false,
		},
		{
			name: "MCP enabled - MCP port present",
			doltdb: &doltv1alpha.DoltDB{
				Spec: doltv1alpha.DoltDBSpec{
					Server: doltv1alpha.Server{
						Listener:  doltv1alpha.Listener{Port: 3306},
						MCPServer: &doltv1alpha.MCPServer{Port: 7007},
					},
				},
			},
			expectMCP:  true,
			mcpPortNum: 7007,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ports := doltContainerPorts(tt.doltdb)

			var mcpPort *int32
			for _, p := range ports {
				if p.Name == DoltMCPPortName {
					mcpPort = &p.ContainerPort
				}
			}

			if tt.expectMCP {
				if mcpPort == nil {
					t.Fatal("expected MCP container port, not found")
				}
				if *mcpPort != tt.mcpPortNum {
					t.Errorf("MCP port: expected %d, got %d", tt.mcpPortNum, *mcpPort)
				}
			} else if mcpPort != nil {
				t.Errorf("unexpected MCP container port %d", *mcpPort)
			}
		})
	}
}
