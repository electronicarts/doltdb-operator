package v1alpha

import (
	"github.com/electronicarts/doltdb-operator/pkg/statefulset"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	ConditionTypeReady           string = "Ready"
	ConditionTypePrimarySwitched string = "PrimarySwitched"
	ConditionTypeComplete        string = "Complete"
	// ConditionTypeStorageResized indicates that the storage has been successfully resized.
	ConditionTypeStorageResized string = "StorageResized"
	// ConditionTypeUpdated indicates that an update has been successfully completed.
	ConditionTypeUpdated string = "Updated"

	ConditionReasonStatefulSetNotReady string = "StatefulSetNotReady"
	ConditionReasonStatefulSetReady    string = "StatefulSetReady"
	ConditionReasonSwitchPrimary       string = "SwitchPrimary"
	ConditionReasonResizingStorage     string = "ResizingStorage"
	ConditionReasonWaitStorageResize   string = "WaitStorageResize"
	ConditionReasonStorageResized      string = "StorageResized"
	ConditionReasonInitializing        string = "Initializing"
	ConditionReasonInitialized         string = "Initialized"
	ConditionReasonPendingUpdate       string = "PendingUpdate"
	ConditionReasonUpdating            string = "Updating"
	ConditionReasonUpdated             string = "Updated"

	ConditionReasonConnectionFailed string = "ConnectionFailed"

	ConditionReasonCreated string = "Created"
	ConditionReasonHealthy string = "Healthy"
	ConditionReasonFailed  string = "Failed"
)

// SetCondition sets a status condition to DoltCluster
func (s *DoltClusterStatus) SetCondition(condition metav1.Condition) {
	if s.Conditions == nil {
		s.Conditions = make([]metav1.Condition, 0)
	}
	meta.SetStatusCondition(&s.Conditions, condition)
}

// UpdateCurrentPrimary updates the current primary status.
func (s *DoltClusterStatus) UpdateCurrentPrimary(doltCluster *DoltCluster, index int) {
	s.CurrentPrimaryPodIndex = &index
	currentPrimary := statefulset.PodName(doltCluster.ObjectMeta, index)
	s.CurrentPrimary = &currentPrimary
}

// UpdateReplicationEpoch updates the current epoch
func (s *DoltClusterStatus) UpdateReplicationEpoch(doltCluster *DoltCluster, epoch int) {
	// NOTE: should check if incoming epoch is less than current?
	s.ReplicationEpoch = &epoch
}

// IsSwitchingPrimary indicates whether the primary is being switched.
func (d *DoltCluster) IsSwitchingPrimary() bool {
	return meta.IsStatusConditionFalse(d.Status.Conditions, ConditionTypePrimarySwitched)
}

// IsReady indicates whether the DoltCluster instance is ready
func (d *DoltCluster) IsReady() bool {
	return meta.IsStatusConditionTrue(d.Status.Conditions, ConditionTypeReady)
}

// IsResizingStorage indicates whether the DoltCluster instance is resizing storage
func (d *DoltCluster) IsResizingStorage() bool {
	return meta.IsStatusConditionFalse(d.Status.Conditions, ConditionTypeStorageResized)
}

// IsResizingStorage indicates whether the DoltCluster instance is waiting for storage resize
func (d *DoltCluster) IsWaitingForStorageResize() bool {
	condition := meta.FindStatusCondition(d.Status.Conditions, ConditionTypeStorageResized)
	if condition == nil {
		return false
	}
	return condition.Status == metav1.ConditionFalse && condition.Reason == ConditionReasonWaitStorageResize
}

// HasPendingUpdate indicates that DoltCluster has a pending update.
func (d *DoltCluster) HasPendingUpdate() bool {
	condition := meta.FindStatusCondition(d.Status.Conditions, ConditionTypeUpdated)
	if condition == nil {
		return false
	}
	return condition.Status == metav1.ConditionFalse && condition.Reason == ConditionReasonPendingUpdate
}

// IsUpdating indicates that a DoltCluster update is in progress.
func (d *DoltCluster) IsUpdating() bool {
	condition := meta.FindStatusCondition(d.Status.Conditions, ConditionTypeUpdated)
	if condition == nil {
		return false
	}
	return condition.Status == metav1.ConditionFalse && condition.Reason == ConditionReasonUpdating
}
