package conditions

import (
	doltv1alpha "github.com/electronicarts/doltdb-operator/api/v1alpha"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// SetCompleteFailedWithMessage sets the complete condition to failed with a custom message.
func SetCompleteFailedWithMessage(c Conditioner, message string) {
	c.SetCondition(metav1.Condition{
		Type:    doltv1alpha.ConditionTypeComplete,
		Status:  metav1.ConditionFalse,
		Reason:  doltv1alpha.ConditionReasonFailed,
		Message: message,
	})
}

// SetCompleteFailed sets the complete condition to failed with a default message.
func SetCompleteFailed(c Conditioner) {
	SetCompleteFailedWithMessage(c, "Failed")
}
