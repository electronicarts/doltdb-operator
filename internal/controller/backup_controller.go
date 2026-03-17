// Copyright (c) 2025 Electronic Arts Inc. All rights reserved.

package controller

import (
	"context"
	"fmt"
	"time"

	doltv1alpha "github.com/electronicarts/doltdb-operator/api/v1alpha"
	"github.com/electronicarts/doltdb-operator/pkg/conditions"
	"github.com/electronicarts/doltdb-operator/pkg/controller/backup"
	"github.com/electronicarts/doltdb-operator/pkg/controller/database"
	sqlClient "github.com/electronicarts/doltdb-operator/pkg/dolt/sql"
	"github.com/electronicarts/doltdb-operator/pkg/patch"
	"github.com/electronicarts/doltdb-operator/pkg/refresolver"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

// BackupReconciler reconciles a Backup object.
type BackupReconciler struct {
	client.Client
	Scheme           *runtime.Scheme
	RefResolver      *refresolver.RefResolver
	BackupReconciler *backup.Reconciler
}

// +kubebuilder:rbac:groups=k8s.dolthub.com,resources=backups,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=k8s.dolthub.com,resources=backups/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=k8s.dolthub.com,resources=backups/finalizers,verbs=update
// +kubebuilder:rbac:groups=k8s.dolthub.com,resources=doltdbs,verbs=get;list;watch;patch
// +kubebuilder:rbac:groups="",resources=secrets,verbs=get;list;watch

func (r *BackupReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx).WithValues("backup", req.NamespacedName)

	var bk doltv1alpha.Backup
	if err := r.Get(ctx, req.NamespacedName, &bk); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}
	logger.Info("Running reconciler for Backup")

	// Skip terminal states
	if bk.IsCompleted() || bk.IsFailed() {
		return ctrl.Result{}, nil
	}

	// Phase 1: Ensure pending status
	if err := r.ensurePending(ctx, &bk); err != nil {
		return ctrl.Result{Requeue: true}, err
	}

	// Phase 2: Resolve DoltDB and wait for readiness
	doltdb, result, err := r.ensureReady(ctx, &bk)
	if err != nil || !result.IsZero() {
		return result, err
	}

	// Phase 3: Check for concurrent backups targeting the same DoltDB
	if running, err := r.hasRunningBackup(ctx, &bk); err != nil {
		return ctrl.Result{Requeue: true}, err
	} else if running {
		logger.Info("Another backup is already running for this DoltDB, requeuing")
		return ctrl.Result{RequeueAfter: 30 * time.Second}, nil
	}

	// Phase 4: Execute backup
	if err := r.executeBackup(ctx, &bk, doltdb); err != nil {
		return r.handleFailure(ctx, &bk, err)
	}

	return ctrl.Result{}, nil
}

// ensurePending sets the initial Pending phase if not already set.
func (r *BackupReconciler) ensurePending(ctx context.Context, bk *doltv1alpha.Backup) error {
	if bk.Status.Phase != "" {
		return nil
	}
	return patch.PatchBackupStatus(ctx, r.Client, bk, func(s *doltv1alpha.BackupStatus) error {
		s.Phase = doltv1alpha.BackupPhasePending
		conditions.SetBackupPending(s)
		return nil
	})
}

// ensureReady resolves the DoltDB reference and waits for it to be ready.
func (r *BackupReconciler) ensureReady(
	ctx context.Context,
	bk *doltv1alpha.Backup,
) (*doltv1alpha.DoltDB, ctrl.Result, error) {
	doltdb, err := r.RefResolver.DoltDB(ctx, bk.DoltDBRef(), bk.GetNamespace())
	if err != nil {
		return nil, ctrl.Result{}, fmt.Errorf("error resolving DoltDB: %w", err)
	}

	result, err := database.WaitForDoltDB(ctx, r.Client, doltdb, false)
	if err != nil || !result.IsZero() {
		return nil, result, err
	}

	return doltdb, ctrl.Result{}, nil
}

// executeBackup handles the Running phase: S3 env injection, SQL connection,
// and delegation to the backup sub-reconciler.
func (r *BackupReconciler) executeBackup(
	ctx context.Context,
	bk *doltv1alpha.Backup,
	doltdb *doltv1alpha.DoltDB,
) error {
	logger := log.FromContext(ctx)

	// Set phase to Running
	now := metav1.Now()
	if err := patch.PatchBackupStatus(ctx, r.Client, bk, func(s *doltv1alpha.BackupStatus) error {
		s.Phase = doltv1alpha.BackupPhaseRunning
		if s.StartedAt == nil {
			s.StartedAt = &now
		}
		conditions.SetBackupRunning(s)
		return nil
	}); err != nil {
		return fmt.Errorf("error setting running status: %w", err)
	}

	// Ensure S3 credential env vars if needed
	if bk.Spec.Storage.S3 != nil {
		if err := r.BackupReconciler.EnsureS3EnvVars(ctx, doltdb, bk.Spec.Storage.S3); err != nil {
			return fmt.Errorf("error ensuring S3 credentials: %w", err)
		}
	}

	// Connect to DoltDB
	doltdbClient, err := sqlClient.NewClientWithDoltDB(ctx, doltdb, r.RefResolver)
	if err != nil {
		return fmt.Errorf("error connecting to DoltDB: %w", err)
	}
	defer func() {
		if err := doltdbClient.Close(); err != nil {
			logger.Error(err, "error closing DoltDB client")
		}
	}()

	// Execute backup
	if err := r.BackupReconciler.Execute(ctx, doltdbClient, bk); err != nil {
		return err
	}

	// Set phase to Completed
	completedAt := metav1.Now()
	return patch.PatchBackupStatus(ctx, r.Client, bk, func(s *doltv1alpha.BackupStatus) error {
		s.Phase = doltv1alpha.BackupPhaseCompleted
		s.CompletedAt = &completedAt
		conditions.SetBackupCompleted(s)
		return nil
	})
}

// handleFailure increments retries, checks backoff limit, and requeues
// with exponential backoff.
func (r *BackupReconciler) handleFailure(
	ctx context.Context,
	bk *doltv1alpha.Backup,
	backupErr error,
) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	// Increment retry count
	if patchErr := patch.PatchBackupStatus(ctx, r.Client, bk, func(s *doltv1alpha.BackupStatus) error {
		s.RetryCount++
		return nil
	}); patchErr != nil {
		logger.Error(patchErr, "error incrementing retry count")
	}

	// Check backoff limit
	requeueAfter, limitExceeded := backup.ShouldRetry(bk.Status.RetryCount, bk.GetBackoffLimit())
	if limitExceeded {
		now := metav1.Now()
		msg := fmt.Sprintf("backoff limit exceeded after %d retries: %v", bk.Status.RetryCount, backupErr)
		_ = patch.PatchBackupStatus(ctx, r.Client, bk, func(s *doltv1alpha.BackupStatus) error {
			s.Phase = doltv1alpha.BackupPhaseFailed
			s.CompletedAt = &now
			s.Error = msg
			conditions.SetBackupFailed(s, msg)
			return nil
		})
		return ctrl.Result{}, nil
	}

	logger.Info("Backup failed, retrying", "retryCount", bk.Status.RetryCount, "requeueAfter", requeueAfter)
	return ctrl.Result{RequeueAfter: requeueAfter}, nil
}

// hasRunningBackup returns true if another Backup targeting the same DoltDB
// is currently in Running phase. This prevents concurrent syncs to the same
// Dolt remote which could cause data corruption.
func (r *BackupReconciler) hasRunningBackup(ctx context.Context, bk *doltv1alpha.Backup) (bool, error) {
	var backupList doltv1alpha.BackupList
	if err := r.List(ctx, &backupList, client.InNamespace(bk.Namespace)); err != nil {
		return false, fmt.Errorf("error listing backups: %w", err)
	}
	for i := range backupList.Items {
		other := &backupList.Items[i]
		if other.Name == bk.Name {
			continue
		}
		if other.Spec.DoltDBRef.Name == bk.Spec.DoltDBRef.Name && other.IsRunning() {
			return true, nil
		}
	}
	return false, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *BackupReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&doltv1alpha.Backup{}).
		Complete(r)
}
