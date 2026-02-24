// Copyright (c) 2025 Electronic Arts Inc. All rights reserved.

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

	ConditionTypeSnapshotCreated string = "SnapshotCronJobCreated"

	ConditionReasonSnapshotCreated     string = "SnapshotCronJobCreated"
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

	ConditionReasonCreated     string = "Created"
	ConditionReasonHealthy     string = "Healthy"
	ConditionReasonFailed      string = "Failed"
	ConditionReasonInvalidSpec string = "InvalidSpec"
)

// SetCondition sets a status condition to DoltDB
func (s *DoltDBStatus) SetCondition(condition metav1.Condition) {
	if s.Conditions == nil {
		s.Conditions = make([]metav1.Condition, 0)
	}
	meta.SetStatusCondition(&s.Conditions, condition)
}

// UpdateCurrentPrimary updates the current primary status.
func (s *DoltDBStatus) UpdateCurrentPrimary(doltdb *DoltDB, index int) {
	s.CurrentPrimaryPodIndex = &index
	currentPrimary := statefulset.PodName(doltdb.ObjectMeta, index)
	s.CurrentPrimary = &currentPrimary
}

// UpdateReplicationEpoch updates the current epoch only if the incoming epoch
// is greater than or equal to the current one, preventing epoch regression.
func (s *DoltDBStatus) UpdateReplicationEpoch(epoch int) {
	if s.ReplicationEpoch != nil && epoch < *s.ReplicationEpoch {
		return
	}
	s.ReplicationEpoch = &epoch
}

// IsSwitchingPrimary indicates whether the primary is being switched.
func (d *DoltDB) IsSwitchingPrimary() bool {
	return meta.IsStatusConditionFalse(d.Status.Conditions, ConditionTypePrimarySwitched)
}

// IsReady indicates whether the DoltDB instance is ready
func (d *DoltDB) IsReady() bool {
	return meta.IsStatusConditionTrue(d.Status.Conditions, ConditionTypeReady)
}

// IsResizingStorage indicates whether the DoltDB instance is resizing storage
func (d *DoltDB) IsResizingStorage() bool {
	return meta.IsStatusConditionFalse(d.Status.Conditions, ConditionTypeStorageResized)
}

// IsResizingStorage indicates whether the DoltDB instance is waiting for storage resize
func (d *DoltDB) IsWaitingForStorageResize() bool {
	condition := meta.FindStatusCondition(d.Status.Conditions, ConditionTypeStorageResized)
	if condition == nil {
		return false
	}
	return condition.Status == metav1.ConditionFalse && condition.Reason == ConditionReasonWaitStorageResize
}

// HasPendingUpdate indicates that DoltDB has a pending update.
func (d *DoltDB) HasPendingUpdate() bool {
	condition := meta.FindStatusCondition(d.Status.Conditions, ConditionTypeUpdated)
	if condition == nil {
		return false
	}
	return condition.Status == metav1.ConditionFalse && condition.Reason == ConditionReasonPendingUpdate
}

// IsUpdating indicates that a DoltDB update is in progress.
func (d *DoltDB) IsUpdating() bool {
	condition := meta.FindStatusCondition(d.Status.Conditions, ConditionTypeUpdated)
	if condition == nil {
		return false
	}
	return condition.Status == metav1.ConditionFalse && condition.Reason == ConditionReasonUpdating
}
