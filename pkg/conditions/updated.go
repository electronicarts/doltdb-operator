package conditions

import (
	doltv1alpha "github.com/electronicarts/doltdb-operator/api/v1alpha"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func SetPendingUpdate(c Conditioner) {
	c.SetCondition(metav1.Condition{
		Type:    doltv1alpha.ConditionTypeUpdated,
		Status:  metav1.ConditionFalse,
		Reason:  doltv1alpha.ConditionReasonPendingUpdate,
		Message: "Pending update",
	})
}

func SetUpdating(c Conditioner) {
	c.SetCondition(metav1.Condition{
		Type:    doltv1alpha.ConditionTypeUpdated,
		Status:  metav1.ConditionFalse,
		Reason:  doltv1alpha.ConditionReasonUpdating,
		Message: "Updating",
	})
}

func SetUpdated(c Conditioner) {
	c.SetCondition(metav1.Condition{
		Type:    doltv1alpha.ConditionTypeUpdated,
		Status:  metav1.ConditionTrue,
		Reason:  doltv1alpha.ConditionReasonUpdated,
		Message: "Updated",
	})
}
