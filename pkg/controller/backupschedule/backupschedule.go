// Copyright (c) 2025 Electronic Arts Inc. All rights reserved.

package backupschedule

import (
	"context"
	"fmt"
	"time"

	doltv1alpha "github.com/electronicarts/doltdb-operator/api/v1alpha"
	"github.com/electronicarts/doltdb-operator/pkg/conditions"
	"github.com/electronicarts/doltdb-operator/pkg/patch"
	cron "github.com/robfig/cron/v3"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

// Reconciler handles the scheduling logic for creating Backup objects.
type Reconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// NewReconciler creates a new Reconciler.
func NewReconciler(client client.Client, scheme *runtime.Scheme) *Reconciler {
	return &Reconciler{Client: client, Scheme: scheme}
}

// Reconcile evaluates the cron schedule and creates a Backup if due.
func (r *Reconciler) Reconcile(
	ctx context.Context,
	bs *doltv1alpha.BackupSchedule,
) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	now := time.Now()
	lastTime := LastScheduleTime(bs)

	schedule, err := cron.ParseStandard(bs.Spec.Schedule)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("error parsing cron schedule '%s': %w", bs.Spec.Schedule, err)
	}

	nextScheduleTime := schedule.Next(lastTime)
	nextMetaTime := metav1.NewTime(nextScheduleTime)
	if err := patch.PatchBackupScheduleStatus(ctx, r.Client, bs, func(s *doltv1alpha.BackupScheduleStatus) error {
		s.NextScheduleTime = &nextMetaTime
		conditions.SetBackupScheduleCreated(s)
		return nil
	}); err != nil {
		return ctrl.Result{}, fmt.Errorf("error patching schedule status: %w", err)
	}

	requeueAfter := ComputeRequeueAfter(bs.Spec.Schedule, lastTime, now)
	if now.Before(nextScheduleTime) {
		logger.V(1).Info("Not yet time for next backup", "nextScheduleTime", nextScheduleTime)
		return ctrl.Result{RequeueAfter: requeueAfter}, nil
	}

	bk, err := BuildBackup(bs, r.Scheme, now)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("error building Backup: %w", err)
	}

	if err := r.Create(ctx, bk); err != nil {
		if apierrors.IsAlreadyExists(err) {
			logger.Info("Backup already exists", "backup", bk.Name)
			return ctrl.Result{RequeueAfter: requeueAfter}, nil
		}
		return ctrl.Result{Requeue: true}, fmt.Errorf("error creating Backup '%s': %w", bk.Name, err)
	}
	logger.Info("Created Backup from schedule", "backup", bk.Name)

	nowMeta := metav1.NewTime(now)
	if err := patch.PatchBackupScheduleStatus(ctx, r.Client, bs, func(s *doltv1alpha.BackupScheduleStatus) error {
		s.LastScheduleTime = &nowMeta
		s.LastBackupRef = bk.Name
		return nil
	}); err != nil {
		return ctrl.Result{}, fmt.Errorf("error patching schedule status: %w", err)
	}

	return ctrl.Result{RequeueAfter: ComputeRequeueAfter(bs.Spec.Schedule, now, now)}, nil
}

// LastScheduleTime returns the last schedule time or falls back to the
// creation timestamp.
func LastScheduleTime(bs *doltv1alpha.BackupSchedule) time.Time {
	if bs.Status.LastScheduleTime != nil {
		return bs.Status.LastScheduleTime.Time
	}
	return bs.CreationTimestamp.Time
}

// ComputeRequeueAfter calculates when to next reconcile based on the cron
// schedule. Returns a short fallback interval on parse errors or past-due schedules.
func ComputeRequeueAfter(schedule string, lastTime, now time.Time) time.Duration {
	sched, err := cron.ParseStandard(schedule)
	if err != nil {
		return 1 * time.Minute
	}
	next := sched.Next(lastTime)
	if next.Before(now) {
		return 10 * time.Second
	}
	return next.Sub(now)
}

// BuildBackup creates a Backup object from a BackupSchedule, copying all
// relevant spec fields and setting an owner reference for garbage collection.
func BuildBackup(
	bs *doltv1alpha.BackupSchedule,
	scheme *runtime.Scheme,
	now time.Time,
) (*doltv1alpha.Backup, error) {
	backup := &doltv1alpha.Backup{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%s-%d", bs.Name, now.Unix()),
			Namespace: bs.Namespace,
			Labels: map[string]string{
				"k8s.dolthub.com/backup-schedule": bs.Name,
			},
		},
		Spec: doltv1alpha.BackupSpec{
			DoltDBRef:        bs.Spec.DoltDBRef,
			Storage:          bs.Spec.Storage,
			Databases:        bs.Spec.Databases,
			BackoffLimit:     bs.Spec.BackoffLimit,
			Resources:        bs.Spec.Resources,
			ImagePullSecrets: bs.Spec.ImagePullSecrets,
		},
	}

	if err := controllerutil.SetControllerReference(bs, backup, scheme); err != nil {
		return nil, fmt.Errorf("error setting owner reference: %w", err)
	}
	return backup, nil
}
