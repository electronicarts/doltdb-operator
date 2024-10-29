package conditions

import (
	"testing"

	doltv1alpha "github.com/electronicarts/doltdb-operator/api/v1alpha"
	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestSetReadyWithDoltCluster(t *testing.T) {
	tests := []struct {
		name            string
		sts             *appsv1.StatefulSet
		doltdb          *doltv1alpha.DoltCluster
		expectedStatus  metav1.ConditionStatus
		expectedReason  string
		expectedMessage string
	}{
		{
			name: "updating",
			sts: &appsv1.StatefulSet{
				Status: appsv1.StatefulSetStatus{
					Replicas:      1,
					ReadyReplicas: 1,
				},
			},
			doltdb: &doltv1alpha.DoltCluster{
				Status: doltv1alpha.DoltClusterStatus{
					Conditions: []metav1.Condition{
						{
							Type:    doltv1alpha.ConditionTypeUpdated,
							Status:  metav1.ConditionFalse,
							Reason:  doltv1alpha.ConditionReasonUpdating,
							Message: "Updating",
						},
					},
				},
			},
			expectedStatus:  metav1.ConditionFalse,
			expectedReason:  doltv1alpha.ConditionReasonUpdating,
			expectedMessage: "Updating",
		},
		{
			name: "statefulset not ready",
			sts: &appsv1.StatefulSet{
				Status: appsv1.StatefulSetStatus{
					Replicas:      1,
					ReadyReplicas: 0,
				},
			},
			doltdb:          &doltv1alpha.DoltCluster{},
			expectedStatus:  metav1.ConditionFalse,
			expectedReason:  doltv1alpha.ConditionReasonStatefulSetNotReady,
			expectedMessage: "Not ready",
		},
		{
			name: "pending update",
			sts: &appsv1.StatefulSet{
				Status: appsv1.StatefulSetStatus{
					Replicas:      1,
					ReadyReplicas: 1,
				},
			},
			doltdb: &doltv1alpha.DoltCluster{
				Status: doltv1alpha.DoltClusterStatus{
					Conditions: []metav1.Condition{
						{
							Type:    doltv1alpha.ConditionTypeUpdated,
							Status:  metav1.ConditionFalse,
							Reason:  doltv1alpha.ConditionReasonPendingUpdate,
							Message: "Pending update",
						},
					},
				},
			},
			expectedStatus:  metav1.ConditionTrue,
			expectedReason:  doltv1alpha.ConditionReasonPendingUpdate,
			expectedMessage: "Pending update",
		},
		{
			name: "statefulset ready",
			sts: &appsv1.StatefulSet{
				Status: appsv1.StatefulSetStatus{
					Replicas:      1,
					ReadyReplicas: 1,
				},
			},
			doltdb:          &doltv1alpha.DoltCluster{},
			expectedStatus:  metav1.ConditionTrue,
			expectedReason:  doltv1alpha.ConditionReasonStatefulSetReady,
			expectedMessage: "Running",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := &MockConditioner{}
			SetReadyWithDoltCluster(m, tt.sts, tt.doltdb)

			if len(m.conditions) != 1 {
				t.Fatalf("expected 1 condition, got %d", len(m.conditions))
			}

			condition := m.conditions[0]
			if condition.Status != tt.expectedStatus {
				t.Errorf("expected status %v, got %v", tt.expectedStatus, condition.Status)
			}
			if condition.Reason != tt.expectedReason {
				t.Errorf("expected reason %v, got %v", tt.expectedReason, condition.Reason)
			}
			if condition.Message != tt.expectedMessage {
				t.Errorf("expected message %v, got %v", tt.expectedMessage, condition.Message)
			}
		})
	}
}
