package dolt

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	doltv1alpha "github.com/electronicarts/doltdb-operator/api/v1alpha"
	"github.com/electronicarts/doltdb-operator/pkg/statefulset"
	corev1 "k8s.io/api/core/v1"

	"sigs.k8s.io/controller-runtime/pkg/client"
)

type DoltStatus struct {
	Database       string
	Role           string
	Epoch          int
	Remote         string
	ReplicationLag sql.NullInt64
	LastUpdate     sql.NullTime
	CurrentError   sql.NullString
}

type DBState struct {
	Role    string
	Epoch   int
	Status  []DoltStatus
	Version string
	Err     error
}

func PickNextPrimary(dbstates []DBState) int {
	firstStandby := -1
	nextPrimary := -1
	var updated time.Time

	for i, state := range dbstates {
		if state.Role == StandbyRoleValue.String() {
			if firstStandby == -1 {
				firstStandby = i
			}

			var oldestDB time.Time
			for _, status := range state.Status {
				if status.LastUpdate.Valid && (oldestDB == (time.Time{}) || oldestDB.After(status.LastUpdate.Time)) {
					oldestDB = status.LastUpdate.Time
				}
			}

			if oldestDB != (time.Time{}) && (updated == (time.Time{}) || updated.Before(oldestDB)) {
				nextPrimary = i
				updated = oldestDB
			}
		}
	}
	if nextPrimary != -1 {
		return nextPrimary
	}
	return firstStandby
}

func MarkRoleStandby(ctx context.Context, doltPod *corev1.Pod, k8sClient client.Client) error {
	patch := client.MergeFrom(doltPod.DeepCopy())
	doltPod.ObjectMeta.Labels[RoleLabel] = StandbyRoleValue.String()
	return k8sClient.Patch(ctx, doltPod, patch)
}

func MarkRolePrimary(ctx context.Context, doltPod *corev1.Pod, k8sClient client.Client) error {
	patch := client.MergeFrom(doltPod.DeepCopy())
	doltPod.ObjectMeta.Labels[RoleLabel] = PrimaryRoleValue.String()
	return k8sClient.Patch(ctx, doltPod, patch)
}

func CurrentPrimaryAndEpoch(doltdb *doltv1alpha.DoltDB, dbstates []DBState) (int, int, error) {
	highestEpoch := 0
	currentPrimary := -1

	for i := range dbstates {
		if dbstates[i].Role == PrimaryRoleValue.String() {
			if currentPrimary != -1 {
				return -1, -1, fmt.Errorf("more than one reachable pod was in role primary: %s and %s",
					statefulset.PodName(doltdb.ObjectMeta, currentPrimary), statefulset.PodName(doltdb.ObjectMeta, i))
			}
			currentPrimary = i
		}
		if dbstates[i].Epoch > highestEpoch {
			highestEpoch = dbstates[i].Epoch
		}
	}

	if currentPrimary == -1 {
		return -1, -1, errors.New("no reachable pod was in role primary")
	}

	return currentPrimary, highestEpoch, nil
}
