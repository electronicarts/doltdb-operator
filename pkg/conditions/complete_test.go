package conditions

import (
	"testing"

	doltv1alpha "github.com/electronicarts/doltdb-operator/api/v1alpha"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestSetCompleteFailedWithMessage(t *testing.T) {
	tests := []struct {
		name    string
		message string
	}{
		{
			name:    "custom message",
			message: "Custom failure message",
		},
		{
			name:    "default message",
			message: "Failed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockConditioner := &MockConditioner{}
			SetCompleteFailedWithMessage(mockConditioner, tt.message)

			if len(mockConditioner.conditions) != 1 {
				t.Fatalf("expected 1 condition, got %d", len(mockConditioner.conditions))
			}
			condition := mockConditioner.conditions[0]
			if condition.Type != doltv1alpha.ConditionTypeComplete {
				t.Errorf("expected condition type %s, got %s", doltv1alpha.ConditionTypeComplete, condition.Type)
			}
			if condition.Status != metav1.ConditionFalse {
				t.Errorf("expected condition status %s, got %s", metav1.ConditionFalse, condition.Status)
			}
			if condition.Reason != doltv1alpha.ConditionReasonFailed {
				t.Errorf("expected condition reason %s, got %s", doltv1alpha.ConditionReasonFailed, condition.Reason)
			}
			if condition.Message != tt.message {
				t.Errorf("expected condition message %s, got %s", tt.message, condition.Message)
			}
		})
	}
}

func TestSetCompleteFailed(t *testing.T) {
	mockConditioner := &MockConditioner{}
	SetCompleteFailed(mockConditioner)

	if len(mockConditioner.conditions) != 1 {
		t.Fatalf("expected 1 condition, got %d", len(mockConditioner.conditions))
	}
	condition := mockConditioner.conditions[0]
	if condition.Type != doltv1alpha.ConditionTypeComplete {
		t.Errorf("expected condition type %s, got %s", doltv1alpha.ConditionTypeComplete, condition.Type)
	}
	if condition.Status != metav1.ConditionFalse {
		t.Errorf("expected condition status %s, got %s", metav1.ConditionFalse, condition.Status)
	}
	if condition.Reason != doltv1alpha.ConditionReasonFailed {
		t.Errorf("expected condition reason %s, got %s", doltv1alpha.ConditionReasonFailed, condition.Reason)
	}
	if condition.Message != "Failed" {
		t.Errorf("expected condition message %s, got %s", "Failed", condition.Message)
	}
}
