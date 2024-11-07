package conditions

import (
	"fmt"

	doltv1alpha "github.com/electronicarts/doltdb-operator/api/v1alpha"
	"github.com/electronicarts/doltdb-operator/pkg/statefulset"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
)

// SetPrimarySwitching sets the conditions indicating that the primary is in the process of switching.
func SetPrimarySwitching(c Conditioner, doltdb *doltv1alpha.DoltDB) {
	msg := switchingPrimaryMessage(doltdb)
	c.SetCondition(metav1.Condition{
		Type:    doltv1alpha.ConditionTypeReady,
		Status:  metav1.ConditionFalse,
		Reason:  doltv1alpha.ConditionReasonSwitchPrimary,
		Message: msg,
	})
	c.SetCondition(metav1.Condition{
		Type:    doltv1alpha.ConditionTypePrimarySwitched,
		Status:  metav1.ConditionFalse,
		Reason:  doltv1alpha.ConditionReasonSwitchPrimary,
		Message: msg,
	})
}

// SetPrimarySwitched sets the condition indicating that the primary has been successfully switched.
func SetPrimarySwitched(c Conditioner) {
	c.SetCondition(metav1.Condition{
		Type:    doltv1alpha.ConditionTypePrimarySwitched,
		Status:  metav1.ConditionTrue,
		Reason:  doltv1alpha.ConditionReasonSwitchPrimary,
		Message: "Switchover complete",
	})
}

// switchingPrimaryMessage generates a message indicating the target primary pod during a switch.
func switchingPrimaryMessage(doltdb *doltv1alpha.DoltDB) string {
	return fmt.Sprintf(
		"Switching primary to '%s'",
		statefulset.PodName(doltdb.ObjectMeta, ptr.Deref(doltdb.Replication().Primary.PodIndex, 0)),
	)
}
