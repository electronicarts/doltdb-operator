// Copyright (c) 2025 Electronic Arts Inc. All rights reserved.

package dolt

import (
	"database/sql"
	"testing"
	"time"
)

func TestPickNextPrimary(t *testing.T) {
	now := time.Now()

	tests := []struct {
		name     string
		states   []DBState
		expected int
	}{
		{
			name:     "no standbys returns -1",
			states:   []DBState{{Role: "primary", Epoch: 1}},
			expected: -1,
		},
		{
			name: "single standby without LastUpdate is selected as first standby",
			states: []DBState{
				{Role: "primary", Epoch: 1},
				{Role: "standby", Epoch: 1},
			},
			expected: 1,
		},
		{
			name: "most recently updated standby is selected",
			states: []DBState{
				{Role: "primary", Epoch: 1},
				{Role: "standby", Epoch: 1, Status: []DoltStatus{
					{LastUpdate: sql.NullTime{Time: now.Add(-10 * time.Second), Valid: true}},
				}},
				{Role: "standby", Epoch: 1, Status: []DoltStatus{
					{LastUpdate: sql.NullTime{Time: now.Add(-1 * time.Second), Valid: true}},
				}},
			},
			expected: 2,
		},
		{
			name: "standby with no LastUpdate loses to one with LastUpdate",
			states: []DBState{
				{Role: "primary", Epoch: 1},
				{Role: "standby", Epoch: 1},
				{Role: "standby", Epoch: 1, Status: []DoltStatus{
					{LastUpdate: sql.NullTime{Time: now.Add(-5 * time.Second), Valid: true}},
				}},
			},
			expected: 2,
		},
		{
			name: "multiple databases use oldest LastUpdate per standby",
			states: []DBState{
				{Role: "primary", Epoch: 1},
				{Role: "standby", Epoch: 1, Status: []DoltStatus{
					{Database: "db1", LastUpdate: sql.NullTime{Time: now.Add(-2 * time.Second), Valid: true}},
					{Database: "db2", LastUpdate: sql.NullTime{Time: now.Add(-20 * time.Second), Valid: true}},
				}},
				{Role: "standby", Epoch: 1, Status: []DoltStatus{
					{Database: "db1", LastUpdate: sql.NullTime{Time: now.Add(-5 * time.Second), Valid: true}},
					{Database: "db2", LastUpdate: sql.NullTime{Time: now.Add(-5 * time.Second), Valid: true}},
				}},
			},
			// Pod 1's oldest DB is 20s ago, pod 2's oldest is 5s ago. Pod 2 wins.
			expected: 2,
		},
		{
			name: "all standbys have errors and no status falls back to first standby",
			states: []DBState{
				{Role: "primary", Epoch: 1},
				{Role: "standby", Epoch: 1},
				{Role: "standby", Epoch: 1},
			},
			expected: 1,
		},
		{
			name:     "empty states returns -1",
			states:   []DBState{},
			expected: -1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := PickNextPrimary(tt.states)
			if result != tt.expected {
				t.Errorf("expected %d, got %d", tt.expected, result)
			}
		})
	}
}
