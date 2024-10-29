package conditions

import (
	"testing"

	doltv1alpha "github.com/electronicarts/doltdb-operator/api/v1alpha"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestSetPendingUpdate(t *testing.T) {
	mockConditioner := &MockConditioner{}
	SetPendingUpdate(mockConditioner)

	expectedCondition := metav1.Condition{
		Type:    doltv1alpha.ConditionTypeUpdated,
		Status:  metav1.ConditionFalse,
		Reason:  doltv1alpha.ConditionReasonPendingUpdate,
		Message: "Pending update",
	}

	if len(mockConditioner.conditions) != 1 {
		t.Fatalf("expected 1 condition, got %d", len(mockConditioner.conditions))
	}

	if mockConditioner.conditions[0] != expectedCondition {
		t.Errorf("expected condition %v, got %v", expectedCondition, mockConditioner.conditions[0])
	}
}

func TestSetUpdating(t *testing.T) {
	mockConditioner := &MockConditioner{}
	SetUpdating(mockConditioner)

	expectedCondition := metav1.Condition{
		Type:    doltv1alpha.ConditionTypeUpdated,
		Status:  metav1.ConditionFalse,
		Reason:  doltv1alpha.ConditionReasonUpdating,
		Message: "Updating",
	}

	if len(mockConditioner.conditions) != 1 {
		t.Fatalf("expected 1 condition, got %d", len(mockConditioner.conditions))
	}

	if mockConditioner.conditions[0] != expectedCondition {
		t.Errorf("expected condition %v, got %v", expectedCondition, mockConditioner.conditions[0])
	}
}

func TestSetUpdated(t *testing.T) {
	mockConditioner := &MockConditioner{}
	SetUpdated(mockConditioner)

	expectedCondition := metav1.Condition{
		Type:    doltv1alpha.ConditionTypeUpdated,
		Status:  metav1.ConditionTrue,
		Reason:  doltv1alpha.ConditionReasonUpdated,
		Message: "Updated",
	}

	if len(mockConditioner.conditions) != 1 {
		t.Fatalf("expected 1 condition, got %d", len(mockConditioner.conditions))
	}

	if mockConditioner.conditions[0] != expectedCondition {
		t.Errorf("expected condition %v, got %v", expectedCondition, mockConditioner.conditions[0])
	}
}
