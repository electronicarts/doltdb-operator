// Copyright (c) 2025 Electronic Arts Inc. All rights reserved.

package backup

import (
	"testing"
	"time"

	doltv1alpha "github.com/electronicarts/doltdb-operator/api/v1alpha"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/utils/ptr"
)

func TestBuildBackupURL(t *testing.T) {
	tests := []struct {
		name    string
		storage doltv1alpha.BackupStorage
		want    string
		wantErr bool
	}{
		{
			name: "S3 bucket only",
			storage: doltv1alpha.BackupStorage{
				S3: &doltv1alpha.S3BackupStorage{
					Bucket: "my-bucket",
				},
			},
			want: "aws://[my-bucket]",
		},
		{
			name: "S3 with region",
			storage: doltv1alpha.BackupStorage{
				S3: &doltv1alpha.S3BackupStorage{
					Bucket: "my-bucket",
					Region: "us-east-1",
				},
			},
			want: "aws://[my-bucket:us-east-1]",
		},
		{
			name: "S3 with region and prefix",
			storage: doltv1alpha.BackupStorage{
				S3: &doltv1alpha.S3BackupStorage{
					Bucket: "my-bucket",
					Region: "us-west-2",
					Prefix: "backups/daily",
				},
			},
			want: "aws://[my-bucket:us-west-2]/backups/daily",
		},
		{
			name: "S3 with endpoint (MinIO)",
			storage: doltv1alpha.BackupStorage{
				S3: &doltv1alpha.S3BackupStorage{
					Bucket:   "my-bucket",
					Region:   "us-east-1",
					Endpoint: "minio.local:9000",
				},
			},
			want: "aws://[my-bucket:us-east-1:minio.local:9000]",
		},
		{
			name: "S3 with endpoint but no region",
			storage: doltv1alpha.BackupStorage{
				S3: &doltv1alpha.S3BackupStorage{
					Bucket:   "my-bucket",
					Endpoint: "minio.local:9000",
				},
			},
			want: "aws://[my-bucket::minio.local:9000]",
		},
		{
			name: "DoltHub storage",
			storage: doltv1alpha.BackupStorage{
				DoltHub: &doltv1alpha.DoltHubBackupStorage{
					RemoteURL: "https://doltremoteapi.dolthub.com/myorg/myrepo",
				},
			},
			want: "https://doltremoteapi.dolthub.com/myorg/myrepo",
		},
		{
			name: "Local storage",
			storage: doltv1alpha.BackupStorage{
				Local: &doltv1alpha.LocalBackupStorage{
					Path: "/var/backups/dolt",
				},
			},
			want: "file:///var/backups/dolt",
		},
		{
			name:    "No storage backend",
			storage: doltv1alpha.BackupStorage{},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := BuildBackupURL(tt.storage)
			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			assert.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestBuildS3URL(t *testing.T) {
	tests := []struct {
		name string
		s3   *doltv1alpha.S3BackupStorage
		want string
	}{
		{
			name: "full S3 config",
			s3: &doltv1alpha.S3BackupStorage{
				Bucket:         "backup-bucket",
				Region:         "eu-west-1",
				Endpoint:       "s3.eu-west-1.amazonaws.com",
				Prefix:         "dolt/prod",
				ForcePathStyle: ptr.To(true),
			},
			want: "aws://[backup-bucket:eu-west-1:s3.eu-west-1.amazonaws.com]/dolt/prod",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := buildS3URL(tt.s3)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestS3EnvVars(t *testing.T) {
	tests := []struct {
		name string
		s3   *doltv1alpha.S3BackupStorage
		want int
	}{
		{
			name: "no secret refs (IRSA)",
			s3: &doltv1alpha.S3BackupStorage{
				Bucket: "my-bucket",
				Region: "us-east-1",
			},
			want: 1, // only AWS_REGION
		},
		{
			name: "static credentials with region",
			s3: &doltv1alpha.S3BackupStorage{
				Bucket: "my-bucket",
				Region: "us-east-1",
				AccessKeyIdSecretKeyRef: &doltv1alpha.SecretKeySelector{
					LocalObjectReference: doltv1alpha.LocalObjectReference{Name: "aws-creds"},
					Key:                  "access-key-id",
				},
				SecretAccessKeySecretKeyRef: &doltv1alpha.SecretKeySelector{
					LocalObjectReference: doltv1alpha.LocalObjectReference{Name: "aws-creds"},
					Key:                  "secret-access-key",
				},
			},
			want: 3, // AWS_ACCESS_KEY_ID + AWS_SECRET_ACCESS_KEY + AWS_REGION
		},
		{
			name: "access key only",
			s3: &doltv1alpha.S3BackupStorage{
				Bucket: "my-bucket",
				AccessKeyIdSecretKeyRef: &doltv1alpha.SecretKeySelector{
					LocalObjectReference: doltv1alpha.LocalObjectReference{Name: "aws-creds"},
					Key:                  "access-key-id",
				},
			},
			want: 1, // only AWS_ACCESS_KEY_ID
		},
		{
			name: "no refs no region",
			s3: &doltv1alpha.S3BackupStorage{
				Bucket: "my-bucket",
			},
			want: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := s3EnvVars(tt.s3)
			assert.Len(t, got, tt.want)
		})
	}
}

func TestHasEnvVars(t *testing.T) {
	current := []corev1.EnvVar{
		{Name: "AWS_ACCESS_KEY_ID", Value: "test"},
		{Name: "AWS_REGION", Value: "us-east-1"},
	}

	t.Run("all present", func(t *testing.T) {
		desired := []corev1.EnvVar{
			{Name: "AWS_ACCESS_KEY_ID"},
			{Name: "AWS_REGION"},
		}
		assert.True(t, hasEnvVars(current, desired))
	})

	t.Run("missing one", func(t *testing.T) {
		desired := []corev1.EnvVar{
			{Name: "AWS_ACCESS_KEY_ID"},
			{Name: "AWS_SECRET_ACCESS_KEY"},
		}
		assert.False(t, hasEnvVars(current, desired))
	})

	t.Run("empty desired", func(t *testing.T) {
		assert.True(t, hasEnvVars(current, nil))
	})
}

func TestContainsEnvVar(t *testing.T) {
	envVars := []corev1.EnvVar{
		{Name: "FOO", Value: "bar"},
		{Name: "BAZ", Value: "qux"},
	}

	assert.True(t, containsEnvVar(envVars, "FOO"))
	assert.True(t, containsEnvVar(envVars, "BAZ"))
	assert.False(t, containsEnvVar(envVars, "MISSING"))
	assert.False(t, containsEnvVar(nil, "FOO"))
}

func TestShouldRetry(t *testing.T) {
	tests := []struct {
		name         string
		retryCount   int32
		backoffLimit int32
		wantDelay    time.Duration
		wantExceeded bool
	}{
		{
			name:         "first retry",
			retryCount:   0,
			backoffLimit: 2,
			wantDelay:    0,
			wantExceeded: false,
		},
		{
			name:         "second retry with backoff",
			retryCount:   1,
			backoffLimit: 2,
			wantDelay:    30 * time.Second,
			wantExceeded: false,
		},
		{
			name:         "limit reached",
			retryCount:   2,
			backoffLimit: 2,
			wantDelay:    0,
			wantExceeded: true,
		},
		{
			name:         "limit exceeded",
			retryCount:   5,
			backoffLimit: 2,
			wantDelay:    0,
			wantExceeded: true,
		},
		{
			name:         "zero backoff limit",
			retryCount:   0,
			backoffLimit: 0,
			wantDelay:    0,
			wantExceeded: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			delay, exceeded := ShouldRetry(tt.retryCount, tt.backoffLimit)
			assert.Equal(t, tt.wantExceeded, exceeded)
			assert.Equal(t, tt.wantDelay, delay)
		})
	}
}
