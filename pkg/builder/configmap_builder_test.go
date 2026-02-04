// Copyright (c) 2025 Electronic Arts Inc. All rights reserved.

package builder

import (
	"testing"
)

func TestHashConfigMapData(t *testing.T) {
	tests := []struct {
		name          string
		data          map[string]string
		expectEmpty   bool
		differentFrom map[string]string // If provided, verify hash is different from this data
	}{
		{
			name:        "empty data returns empty string",
			data:        map[string]string{},
			expectEmpty: true,
		},
		{
			name:        "nil data returns empty string",
			data:        nil,
			expectEmpty: true,
		},
		{
			name: "single entry",
			data: map[string]string{
				"config.yaml": "content",
			},
			expectEmpty: false,
		},
		{
			name: "multiple entries produce same hash regardless of map order",
			data: map[string]string{
				"a.yaml": "content-a",
				"b.yaml": "content-b",
				"c.yaml": "content-c",
			},
			expectEmpty: false,
		},
		{
			name: "different content produces different hash",
			data: map[string]string{
				"config.yaml": "content-1",
			},
			differentFrom: map[string]string{
				"config.yaml": "content-2",
			},
			expectEmpty: false,
		},
		{
			name: "different number of replicas produces different hash",
			data: map[string]string{
				"doltdb-0.yaml": "config0",
				"doltdb-1.yaml": "config1",
			},
			differentFrom: map[string]string{
				"doltdb-0.yaml": "config0",
				"doltdb-1.yaml": "config1",
				"doltdb-2.yaml": "config2",
			},
			expectEmpty: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hash := HashConfigMapData(tt.data)

			if tt.expectEmpty && hash != "" {
				t.Errorf("expected empty hash, got %q", hash)
			}
			if !tt.expectEmpty && hash == "" {
				t.Errorf("expected non-empty hash, got empty string")
			}

			if tt.differentFrom != nil {
				differentHash := HashConfigMapData(tt.differentFrom)
				if hash == differentHash {
					t.Errorf("expected different hashes for different data, but both produced %q", hash)
				}
			}
		})
	}
}

func TestHashConfigMapDataDeterministic(t *testing.T) {
	// Verify that the same data always produces the same hash
	data := map[string]string{
		"z.yaml": "z-content",
		"a.yaml": "a-content",
		"m.yaml": "m-content",
	}

	hash1 := HashConfigMapData(data)
	hash2 := HashConfigMapData(data)
	hash3 := HashConfigMapData(data)

	if hash1 != hash2 || hash2 != hash3 {
		t.Errorf("hash is not deterministic: got %q, %q, %q", hash1, hash2, hash3)
	}
}
