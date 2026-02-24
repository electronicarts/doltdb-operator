// Copyright (c) 2025 Electronic Arts Inc. All rights reserved.

package v1alpha

import (
	"testing"
)

func TestValidateReplicationSpec(t *testing.T) {
	tests := []struct {
		name        string
		replicas    int32
		replication *Replication
		wantErr     bool
	}{
		{
			name:        "standalone with nil replication",
			replicas:    1,
			replication: nil,
			wantErr:     false,
		},
		{
			name:     "standalone with replication disabled",
			replicas: 1,
			replication: &Replication{
				Enabled: false,
			},
			wantErr: false,
		},
		{
			name:     "standalone with replication enabled",
			replicas: 1,
			replication: &Replication{
				Enabled: true,
			},
			wantErr: true,
		},
		{
			name:     "two replicas with replication enabled",
			replicas: 2,
			replication: &Replication{
				Enabled: true,
			},
			wantErr: false,
		},
		{
			name:     "two replicas with replication disabled",
			replicas: 2,
			replication: &Replication{
				Enabled: false,
			},
			wantErr: false,
		},
		{
			name:     "three replicas with replication enabled",
			replicas: 3,
			replication: &Replication{
				Enabled: true,
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			doltdb := &DoltDB{
				Spec: DoltDBSpec{
					Replicas:    tt.replicas,
					Replication: tt.replication,
				},
			}

			err := doltdb.ValidateReplicationSpec()
			if tt.wantErr && err == nil {
				t.Error("expected error but got nil")
			}
			if !tt.wantErr && err != nil {
				t.Errorf("expected no error but got: %v", err)
			}
		})
	}
}
