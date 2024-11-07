package conditions

import (
	"testing"

	doltv1alpha "github.com/electronicarts/doltdb-operator/api/v1alpha"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
)

func TestSetPrimarySwitching(t *testing.T) {
	tests := []struct {
		name     string
		doltdb   *doltv1alpha.DoltDB
		expected []metav1.Condition
	}{
		{
			name: "Primary switching",
			doltdb: &doltv1alpha.DoltDB{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-cluster",
					Namespace: "default",
				},
				Spec: doltv1alpha.DoltDBSpec{
					Replication: &doltv1alpha.Replication{
						ReplicationSpec: doltv1alpha.ReplicationSpec{
							Primary: &doltv1alpha.PrimaryReplication{
								PodIndex: ptr.To(1),
							},
						},
						Enabled: true,
					},
				},
			},
			expected: []metav1.Condition{
				{
					Type:    doltv1alpha.ConditionTypeReady,
					Status:  metav1.ConditionFalse,
					Reason:  doltv1alpha.ConditionReasonSwitchPrimary,
					Message: "Switching primary to 'test-cluster-1'",
				},
				{
					Type:    doltv1alpha.ConditionTypePrimarySwitched,
					Status:  metav1.ConditionFalse,
					Reason:  doltv1alpha.ConditionReasonSwitchPrimary,
					Message: "Switching primary to 'test-cluster-1'",
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := &MockConditioner{}
			SetPrimarySwitching(mock, tt.doltdb)

			if len(mock.conditions) != len(tt.expected) {
				t.Fatalf("expected %d conditions, got %d", len(tt.expected), len(mock.conditions))
			}

			for i, condition := range mock.conditions {
				if condition.Type != tt.expected[i].Type ||
					condition.Status != tt.expected[i].Status ||
					condition.Reason != tt.expected[i].Reason ||
					condition.Message != tt.expected[i].Message {
					t.Errorf("unexpected condition at index %d: %+v", i, condition)
				}
			}
		})
	}
}

func TestSetPrimarySwitched(t *testing.T) {
	tests := []struct {
		name     string
		expected metav1.Condition
	}{
		{
			name: "Primary switched",
			expected: metav1.Condition{
				Type:    doltv1alpha.ConditionTypePrimarySwitched,
				Status:  metav1.ConditionTrue,
				Reason:  doltv1alpha.ConditionReasonSwitchPrimary,
				Message: "Switchover complete",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := &MockConditioner{}
			SetPrimarySwitched(mock)

			if len(mock.conditions) != 1 {
				t.Fatalf("expected 1 condition, got %d", len(mock.conditions))
			}

			condition := mock.conditions[0]
			if condition.Type != tt.expected.Type ||
				condition.Status != tt.expected.Status ||
				condition.Reason != tt.expected.Reason ||
				condition.Message != tt.expected.Message {
				t.Errorf("unexpected condition: %+v", condition)
			}
		})
	}
}
