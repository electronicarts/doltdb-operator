// Copyright (c) 2025 Electronic Arts Inc. All rights reserved.

package v1alpha

import (
	"testing"

	"k8s.io/utils/ptr"
)

func TestUpdateReplicationEpoch(t *testing.T) {
	tests := []struct {
		name          string
		currentEpoch  *int
		incomingEpoch int
		expectedEpoch int
	}{
		{
			name:          "nil epoch accepts any value",
			currentEpoch:  nil,
			incomingEpoch: 5,
			expectedEpoch: 5,
		},
		{
			name:          "higher epoch is accepted",
			currentEpoch:  ptr.To(5),
			incomingEpoch: 10,
			expectedEpoch: 10,
		},
		{
			name:          "equal epoch is accepted",
			currentEpoch:  ptr.To(5),
			incomingEpoch: 5,
			expectedEpoch: 5,
		},
		{
			name:          "lower epoch is rejected",
			currentEpoch:  ptr.To(10),
			incomingEpoch: 5,
			expectedEpoch: 10,
		},
		{
			name:          "epoch 0 is rejected when current is higher",
			currentEpoch:  ptr.To(3),
			incomingEpoch: 0,
			expectedEpoch: 3,
		},
		{
			name:          "epoch regression by 1 is rejected",
			currentEpoch:  ptr.To(100),
			incomingEpoch: 99,
			expectedEpoch: 100,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			status := &DoltDBStatus{
				ReplicationEpoch: tt.currentEpoch,
			}
			status.UpdateReplicationEpoch(tt.incomingEpoch)

			if status.ReplicationEpoch == nil {
				t.Fatal("ReplicationEpoch should not be nil after update")
			}
			if *status.ReplicationEpoch != tt.expectedEpoch {
				t.Errorf("expected epoch %d, got %d", tt.expectedEpoch, *status.ReplicationEpoch)
			}
		})
	}
}
