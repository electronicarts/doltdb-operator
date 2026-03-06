// Copyright (c) 2025 Electronic Arts Inc. All rights reserved.

package backupschedule

import (
	"context"
	"fmt"
	"testing"
	"time"

	doltv1alpha "github.com/electronicarts/doltdb-operator/api/v1alpha"
	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func newScheme(t *testing.T) *runtime.Scheme {
	t.Helper()
	scheme := runtime.NewScheme()
	err := doltv1alpha.AddToScheme(scheme)
	assert.NoError(t, err)
	return scheme
}

func newBackupSchedule(name, schedule string, createdAt time.Time) *doltv1alpha.BackupSchedule {
	return &doltv1alpha.BackupSchedule{
		ObjectMeta: metav1.ObjectMeta{
			Name:              name,
			Namespace:         "default",
			UID:               types.UID("test-uid"),
			CreationTimestamp: metav1.NewTime(createdAt),
		},
		Spec: doltv1alpha.BackupScheduleSpec{
			Schedule: schedule,
			DoltDBRef: doltv1alpha.DoltDBRef{
				ObjectReference: doltv1alpha.ObjectReference{Name: "my-doltdb"},
			},
			Storage: doltv1alpha.BackupStorage{
				S3: &doltv1alpha.S3BackupStorage{Bucket: "my-bucket"},
			},
		},
	}
}

func newFakeClient(scheme *runtime.Scheme, objs ...client.Object) client.Client {
	return fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(objs...).
		WithStatusSubresource(&doltv1alpha.BackupSchedule{}).
		Build()
}

func TestLastScheduleTime(t *testing.T) {
	created := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	lastScheduled := time.Date(2025, 6, 15, 10, 0, 0, 0, time.UTC)

	tests := []struct {
		name string
		bs   *doltv1alpha.BackupSchedule
		want time.Time
	}{
		{
			name: "falls back to creation timestamp",
			bs: &doltv1alpha.BackupSchedule{
				ObjectMeta: metav1.ObjectMeta{
					CreationTimestamp: metav1.NewTime(created),
				},
			},
			want: created,
		},
		{
			name: "uses last schedule time when set",
			bs: &doltv1alpha.BackupSchedule{
				ObjectMeta: metav1.ObjectMeta{
					CreationTimestamp: metav1.NewTime(created),
				},
				Status: doltv1alpha.BackupScheduleStatus{
					LastScheduleTime: &metav1.Time{Time: lastScheduled},
				},
			},
			want: lastScheduled,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := LastScheduleTime(tt.bs)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestComputeRequeueAfter(t *testing.T) {
	tests := []struct {
		name     string
		schedule string
		lastTime time.Time
		now      time.Time
		want     time.Duration
	}{
		{
			name:     "next is in the future",
			schedule: "0 * * * *",
			lastTime: time.Date(2025, 1, 1, 12, 0, 0, 0, time.UTC),
			now:      time.Date(2025, 1, 1, 12, 30, 0, 0, time.UTC),
			want:     30 * time.Minute,
		},
		{
			name:     "next is past due",
			schedule: "0 * * * *",
			lastTime: time.Date(2025, 1, 1, 10, 0, 0, 0, time.UTC),
			now:      time.Date(2025, 1, 1, 12, 30, 0, 0, time.UTC),
			want:     10 * time.Second,
		},
		{
			name:     "invalid cron falls back to 1m",
			schedule: "not-valid",
			lastTime: time.Now(),
			now:      time.Now(),
			want:     1 * time.Minute,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ComputeRequeueAfter(tt.schedule, tt.lastTime, tt.now)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestBuildBackup(t *testing.T) {
	scheme := newScheme(t)
	now := time.Date(2025, 6, 15, 10, 30, 0, 0, time.UTC)

	bs := &doltv1alpha.BackupSchedule{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "my-schedule",
			Namespace: "default",
			UID:       types.UID("test-uid"),
		},
		Spec: doltv1alpha.BackupScheduleSpec{
			DoltDBRef: doltv1alpha.DoltDBRef{
				ObjectReference: doltv1alpha.ObjectReference{Name: "my-doltdb"},
			},
			Storage: doltv1alpha.BackupStorage{
				S3: &doltv1alpha.S3BackupStorage{Bucket: "my-bucket"},
			},
			Databases: []string{"db1", "db2"},
		},
	}

	bk, err := BuildBackup(bs, scheme, now)
	assert.NoError(t, err)

	// Name format: schedule-name-unixTimestamp
	expectedName := fmt.Sprintf("my-schedule-%d", now.Unix())
	assert.Equal(t, expectedName, bk.Name)
	assert.Equal(t, "default", bk.Namespace)

	// Labels
	assert.Equal(t, "my-schedule", bk.Labels["k8s.dolthub.com/backup-schedule"])

	// Spec fields copied correctly
	assert.Equal(t, bs.Spec.DoltDBRef, bk.Spec.DoltDBRef)
	assert.Equal(t, bs.Spec.Storage, bk.Spec.Storage)
	assert.Equal(t, bs.Spec.Databases, bk.Spec.Databases)

	// Owner reference
	assert.Len(t, bk.OwnerReferences, 1)
	assert.Equal(t, "my-schedule", bk.OwnerReferences[0].Name)
	assert.Equal(t, "BackupSchedule", bk.OwnerReferences[0].Kind)
}

func TestBuildBackup_MissingSchemeRegistration(t *testing.T) {
	bs := &doltv1alpha.BackupSchedule{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "sched",
			Namespace: "default",
			UID:       types.UID("uid"),
		},
	}
	// Empty scheme — SetControllerReference will fail
	_, err := BuildBackup(bs, runtime.NewScheme(), time.Now())
	assert.Error(t, err)
}

func TestReconcile_NotYetDue(t *testing.T) {
	scheme := newScheme(t)
	// Created just now with hourly cron — next run is ~1h away
	created := time.Now().Add(-5 * time.Minute)
	bs := newBackupSchedule("test-schedule", "0 * * * *", created)

	fakeClient := newFakeClient(scheme, bs)
	r := NewReconciler(fakeClient, scheme)

	result, err := r.Reconcile(context.Background(), bs)
	assert.NoError(t, err)
	assert.True(t, result.RequeueAfter > 0, "should requeue for future schedule")

	// No Backup should be created
	var backups doltv1alpha.BackupList
	assert.NoError(t, fakeClient.List(context.Background(), &backups))
	assert.Empty(t, backups.Items)
}

func TestReconcile_DueCreatesBackup(t *testing.T) {
	scheme := newScheme(t)
	// Created 2 hours ago with hourly cron — overdue
	created := time.Now().Add(-2 * time.Hour)
	bs := newBackupSchedule("test-schedule", "0 * * * *", created)

	fakeClient := newFakeClient(scheme, bs)
	r := NewReconciler(fakeClient, scheme)

	result, err := r.Reconcile(context.Background(), bs)
	assert.NoError(t, err)
	assert.True(t, result.RequeueAfter > 0 || result.Requeue)

	// A Backup should be created
	var backups doltv1alpha.BackupList
	assert.NoError(t, fakeClient.List(context.Background(), &backups))
	assert.Len(t, backups.Items, 1)
	assert.Equal(t, "test-schedule", backups.Items[0].Labels["k8s.dolthub.com/backup-schedule"])
	assert.Equal(t, bs.Spec.Storage, backups.Items[0].Spec.Storage)
}

func TestReconcile_InvalidCron(t *testing.T) {
	scheme := newScheme(t)
	bs := newBackupSchedule("bad-cron", "not-a-cron", time.Now())

	fakeClient := newFakeClient(scheme, bs)
	r := NewReconciler(fakeClient, scheme)

	_, err := r.Reconcile(context.Background(), bs)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "error parsing cron schedule")
}
