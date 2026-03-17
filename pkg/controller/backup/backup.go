// Copyright (c) 2025 Electronic Arts Inc. All rights reserved.

package backup

import (
	"context"
	"crypto/sha256"
	"fmt"
	"time"

	doltv1alpha "github.com/electronicarts/doltdb-operator/api/v1alpha"
	sqlClient "github.com/electronicarts/doltdb-operator/pkg/dolt/sql"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

// Reconciler handles the backup execution logic.
type Reconciler struct {
	client.Client
}

// NewReconciler creates a new Reconciler.
func NewReconciler(client client.Client) *Reconciler {
	return &Reconciler{Client: client}
}

// Execute connects to DoltDB and runs dolt_backup stored procedures
// for each database. It is the caller's responsibility to manage
// status, retries, and phase transitions.
func (r *Reconciler) Execute(
	ctx context.Context,
	doltdbClient *sqlClient.Client,
	backup *doltv1alpha.Backup,
) error {
	logger := log.FromContext(ctx)

	databases := backup.Spec.Databases
	if len(databases) == 0 {
		var err error
		databases, err = doltdbClient.ListDatabases(ctx)
		if err != nil {
			return fmt.Errorf("error listing databases: %w", err)
		}
	}

	if len(databases) == 0 {
		logger.Info("No databases to back up")
		return nil
	}

	backupURL, err := BuildBackupURL(backup.Spec.Storage)
	if err != nil {
		return fmt.Errorf("error building backup URL: %w", err)
	}

	for _, db := range databases {
		backupName := stableBackupName(backupURL, db)
		logger.Info("Backing up database", "database", db, "backupName", backupName)

		if err := doltdbClient.BackupDatabase(ctx, db, backupName, backupURL); err != nil {
			return fmt.Errorf("error backing up database '%s': %w", db, err)
		}
		logger.Info("Database backup completed", "database", db)
	}

	return nil
}

// stableBackupName generates a deterministic backup name from the URL and
// database name. This ensures all Backup CRs targeting the same destination
// reuse the same Dolt remote registration, enabling incremental syncs.
func stableBackupName(url, database string) string {
	h := sha256.Sum256([]byte(url))
	return fmt.Sprintf("bk-%s-%x", database, h[:8])
}

// ShouldRetry evaluates whether a backup should be retried based on the
// current retry count and backoff limit. Returns the requeue delay and
// whether the backoff limit has been exceeded.
func ShouldRetry(retryCount, backoffLimit int32) (requeueAfter time.Duration, limitExceeded bool) {
	if retryCount >= backoffLimit {
		return 0, true
	}
	return time.Duration(retryCount) * 30 * time.Second, false
}

// EnsureS3EnvVars patches the DoltDB spec to include AWS credential env vars
// from the Backup's S3 secret key references. This is a no-op if the env vars
// are already present (e.g., IRSA or previously configured).
//
// Secret references are validated before injection to prevent a Backup CR from
// injecting env vars that reference non-existent secrets.
func (r *Reconciler) EnsureS3EnvVars(
	ctx context.Context,
	doltdb *doltv1alpha.DoltDB,
	s3 *doltv1alpha.S3BackupStorage,
) error {
	desired := s3EnvVars(s3)
	if len(desired) == 0 {
		return nil
	}

	if hasEnvVars(doltdb.Spec.Env, desired) {
		return nil
	}

	// Validate that referenced secrets exist before mutating DoltDB spec.
	if err := r.validateSecretRefs(ctx, doltdb.Namespace, s3); err != nil {
		return fmt.Errorf("error validating S3 secret references: %w", err)
	}

	logger := log.FromContext(ctx)
	logger.Info("Injecting S3 credential env vars into DoltDB", "doltdb", doltdb.Name)

	p := client.MergeFrom(doltdb.DeepCopy())
	for _, d := range desired {
		if !containsEnvVar(doltdb.Spec.Env, d.Name) {
			doltdb.Spec.Env = append(doltdb.Spec.Env, d)
		}
	}
	return r.Patch(ctx, doltdb, p)
}

// validateSecretRefs checks that the secrets referenced by S3 credential
// fields exist in the given namespace.
func (r *Reconciler) validateSecretRefs(ctx context.Context, namespace string, s3 *doltv1alpha.S3BackupStorage) error {
	refs := []*doltv1alpha.SecretKeySelector{
		s3.AccessKeyIdSecretKeyRef,
		s3.SecretAccessKeySecretKeyRef,
	}
	for _, ref := range refs {
		if ref == nil {
			continue
		}
		var secret corev1.Secret
		key := client.ObjectKey{Namespace: namespace, Name: ref.Name}
		if err := r.Get(ctx, key, &secret); err != nil {
			return fmt.Errorf("secret '%s' not found in namespace '%s': %w", ref.Name, namespace, err)
		}
		if _, ok := secret.Data[ref.Key]; !ok {
			return fmt.Errorf("key '%s' not found in secret '%s/%s'", ref.Key, namespace, ref.Name)
		}
	}
	return nil
}

// BuildBackupURL constructs the DoltDB backup URL from the storage configuration.
func BuildBackupURL(storage doltv1alpha.BackupStorage) (string, error) {
	if storage.S3 != nil {
		return buildS3URL(storage.S3), nil
	}
	if storage.DoltHub != nil {
		return storage.DoltHub.RemoteURL, nil
	}
	if storage.Local != nil {
		return fmt.Sprintf("file://%s", storage.Local.Path), nil
	}
	return "", fmt.Errorf("no storage backend specified")
}

// buildS3URL constructs a DoltDB-compatible S3 URL.
// Dolt's aws:// URL format is: aws://[dynamo_table:s3_bucket]/db_path
// The DynamoDB table name defaults to the bucket name (Dolt convention).
// Region must be set via AWS_REGION env var, not in the URL.
func buildS3URL(s3 *doltv1alpha.S3BackupStorage) string {
	dynamoTable := s3.Bucket
	if s3.DynamoDBTable != "" {
		dynamoTable = s3.DynamoDBTable
	}
	url := fmt.Sprintf("aws://[%s:%s]", dynamoTable, s3.Bucket)
	if s3.Prefix != "" {
		url = fmt.Sprintf("%s/%s", url, s3.Prefix)
	}
	return url
}

// s3EnvVars builds the list of AWS credential env vars from S3 secret refs.
func s3EnvVars(s3 *doltv1alpha.S3BackupStorage) []corev1.EnvVar {
	var envVars []corev1.EnvVar
	if s3.AccessKeyIdSecretKeyRef != nil {
		envVars = append(envVars, corev1.EnvVar{
			Name: "AWS_ACCESS_KEY_ID",
			ValueFrom: &corev1.EnvVarSource{
				SecretKeyRef: ptr.To(s3.AccessKeyIdSecretKeyRef.ToKubernetesType()),
			},
		})
	}
	if s3.SecretAccessKeySecretKeyRef != nil {
		envVars = append(envVars, corev1.EnvVar{
			Name: "AWS_SECRET_ACCESS_KEY",
			ValueFrom: &corev1.EnvVarSource{
				SecretKeyRef: ptr.To(s3.SecretAccessKeySecretKeyRef.ToKubernetesType()),
			},
		})
	}
	if s3.Region != "" {
		envVars = append(envVars, corev1.EnvVar{
			Name:  "AWS_REGION",
			Value: s3.Region,
		})
	}
	return envVars
}

// hasEnvVars returns true if all desired env vars are already present in the current list.
func hasEnvVars(current, desired []corev1.EnvVar) bool {
	for _, d := range desired {
		if !containsEnvVar(current, d.Name) {
			return false
		}
	}
	return true
}

func containsEnvVar(envVars []corev1.EnvVar, name string) bool {
	for _, e := range envVars {
		if e.Name == name {
			return true
		}
	}
	return false
}
