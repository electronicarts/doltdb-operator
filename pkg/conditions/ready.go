package conditions

import (
	doltv1alpha "github.com/electronicarts/doltdb-operator/api/v1alpha"
	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// SetReadyHealthy sets the Ready condition to True with a Healthy reason.
func SetReadyHealthy(c Conditioner) {
	c.SetCondition(metav1.Condition{
		Type:    doltv1alpha.ConditionTypeReady,
		Status:  metav1.ConditionTrue,
		Reason:  doltv1alpha.ConditionReasonHealthy,
		Message: "Healthy",
	})
}

// SetReadyUnhealthyWithError sets the Ready condition to False with a Healthy reason and an error message.
func SetReadyUnhealthyWithError(c Conditioner, err error) {
	c.SetCondition(metav1.Condition{
		Type:    doltv1alpha.ConditionTypeReady,
		Status:  metav1.ConditionFalse,
		Reason:  doltv1alpha.ConditionReasonHealthy,
		Message: err.Error(),
	})
}

// SetReadyCreatedWithMessage sets the Ready condition to True with a Created reason and a custom message.
func SetReadyCreatedWithMessage(c Conditioner, message string) {
	c.SetCondition(metav1.Condition{
		Type:    doltv1alpha.ConditionTypeReady,
		Status:  metav1.ConditionTrue,
		Reason:  doltv1alpha.ConditionReasonCreated,
		Message: message,
	})
}

// SetReadyCreated sets the Ready condition to True with a Created reason and a default message.
func SetReadyCreated(c Conditioner) {
	SetReadyCreatedWithMessage(c, "Created")
}

// SetReadyFailedWithMessage sets the Ready condition to False with a Failed reason and a custom message.
func SetReadyFailedWithMessage(c Conditioner, message string) {
	c.SetCondition(metav1.Condition{
		Type:    doltv1alpha.ConditionTypeReady,
		Status:  metav1.ConditionFalse,
		Reason:  doltv1alpha.ConditionReasonFailed,
		Message: message,
	})
}

// SetReadyFailed sets the Ready condition to False with a Failed reason and a default message.
func SetReadyFailed(c Conditioner) {
	SetReadyFailedWithMessage(c, "Failed")
}

// SetReadyWithStatefulSet sets the Ready condition based on the status of the provided StatefulSet.
func SetReadyWithStatefulSet(c Conditioner, sts *appsv1.StatefulSet) {
	if sts.Status.Replicas == 0 || sts.Status.ReadyReplicas != sts.Status.Replicas {
		c.SetCondition(metav1.Condition{
			Type:    doltv1alpha.ConditionTypeReady,
			Status:  metav1.ConditionFalse,
			Reason:  doltv1alpha.ConditionReasonStatefulSetNotReady,
			Message: "Not ready",
		})
		return
	}
	c.SetCondition(metav1.Condition{
		Type:    doltv1alpha.ConditionTypeReady,
		Status:  metav1.ConditionTrue,
		Reason:  doltv1alpha.ConditionReasonStatefulSetReady,
		Message: "Running",
	})
}

// SetReadyWithDoltDB sets the Ready condition based on the status of the provided StatefulSet and DoltDB.
func SetReadyWithDoltDB(c Conditioner, sts *appsv1.StatefulSet, doltdb *doltv1alpha.DoltDB) {
	if doltdb.IsUpdating() {
		c.SetCondition(metav1.Condition{
			Type:    doltv1alpha.ConditionTypeReady,
			Status:  metav1.ConditionFalse,
			Reason:  doltv1alpha.ConditionReasonUpdating,
			Message: "Updating",
		})
		return
	}
	if sts.Status.Replicas == 0 || sts.Status.ReadyReplicas != sts.Status.Replicas {
		c.SetCondition(metav1.Condition{
			Type:    doltv1alpha.ConditionTypeReady,
			Status:  metav1.ConditionFalse,
			Reason:  doltv1alpha.ConditionReasonStatefulSetNotReady,
			Message: "Not ready",
		})
		return
	}

	if doltdb.HasPendingUpdate() {
		c.SetCondition(metav1.Condition{
			Type:    doltv1alpha.ConditionTypeReady,
			Status:  metav1.ConditionTrue,
			Reason:  doltv1alpha.ConditionReasonPendingUpdate,
			Message: "Pending update",
		})
		return
	}
	c.SetCondition(metav1.Condition{
		Type:    doltv1alpha.ConditionTypeReady,
		Status:  metav1.ConditionTrue,
		Reason:  doltv1alpha.ConditionReasonStatefulSetReady,
		Message: "Running",
	})
}

// SetReadyStorageResizing sets the Ready and StorageResized conditions to False with a ResizingStorage reason.
func SetReadyStorageResizing(c Conditioner) {
	msg := "Resizing storage"
	c.SetCondition(metav1.Condition{
		Type:    doltv1alpha.ConditionTypeReady,
		Status:  metav1.ConditionFalse,
		Reason:  doltv1alpha.ConditionReasonResizingStorage,
		Message: msg,
	})
	c.SetCondition(metav1.Condition{
		Type:    doltv1alpha.ConditionTypeStorageResized,
		Status:  metav1.ConditionFalse,
		Reason:  doltv1alpha.ConditionReasonResizingStorage,
		Message: msg,
	})
}

// SetReadyWaitingStorageResize sets the Ready and StorageResized conditions to False with a WaitStorageResize reason.
func SetReadyWaitingStorageResize(c Conditioner) {
	msg := "Waiting for storage resize"
	c.SetCondition(metav1.Condition{
		Type:    doltv1alpha.ConditionTypeReady,
		Status:  metav1.ConditionFalse,
		Reason:  doltv1alpha.ConditionReasonWaitStorageResize,
		Message: msg,
	})
	c.SetCondition(metav1.Condition{
		Type:    doltv1alpha.ConditionTypeStorageResized,
		Status:  metav1.ConditionFalse,
		Reason:  doltv1alpha.ConditionReasonWaitStorageResize,
		Message: msg,
	})
}

// SetReadyStorageResized sets the StorageResized condition to True with a StorageResized reason.
func SetReadyStorageResized(c Conditioner) {
	c.SetCondition(metav1.Condition{
		Type:    doltv1alpha.ConditionTypeStorageResized,
		Status:  metav1.ConditionTrue,
		Reason:  doltv1alpha.ConditionReasonStorageResized,
		Message: "Storage resized",
	})
}

// SetReadyWithSnapshotJobCreated sets the SnapshotCreated condition to True with a SnapshotCreated reason.
func SetReadyWithSnapshotJobCreated(c Conditioner) {
	c.SetCondition(metav1.Condition{
		Type:    doltv1alpha.ConditionTypeSnapshotCreated,
		Status:  metav1.ConditionTrue,
		Reason:  doltv1alpha.ConditionReasonSnapshotCreated,
		Message: "Snapshot cron job created",
	})
}
