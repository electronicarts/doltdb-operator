// Copyright (c) 2025 Electronic Arts Inc. All rights reserved.

package backup

import (
	"context"
	"testing"
	"time"

	doltv1alpha "github.com/electronicarts/doltdb-operator/api/v1alpha"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
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
			want: "aws://[my-bucket:my-bucket]",
		},
		{
			name: "S3 with region",
			storage: doltv1alpha.BackupStorage{
				S3: &doltv1alpha.S3BackupStorage{
					Bucket: "my-bucket",
					Region: "us-east-1",
				},
			},
			want: "aws://[my-bucket:my-bucket]",
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
			want: "aws://[my-bucket:my-bucket]/backups/daily",
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

func TestStableBackupName(t *testing.T) {
	url := "aws://[my-bucket:my-bucket]/backups"

	t.Run("deterministic", func(t *testing.T) {
		a := stableBackupName(url, "mydb")
		b := stableBackupName(url, "mydb")
		assert.Equal(t, a, b)
	})

	t.Run("different databases produce different names", func(t *testing.T) {
		a := stableBackupName(url, "db1")
		b := stableBackupName(url, "db2")
		assert.NotEqual(t, a, b)
	})

	t.Run("different URLs produce different names", func(t *testing.T) {
		a := stableBackupName("aws://[bucket-a:bucket-a]/backups", "mydb")
		b := stableBackupName("aws://[bucket-b:bucket-b]/backups", "mydb")
		assert.NotEqual(t, a, b)
	})

	t.Run("format includes database name", func(t *testing.T) {
		name := stableBackupName(url, "madden26")
		assert.Contains(t, name, "madden26")
		assert.True(t, len(name) > 0)
	})
}

func TestBuildS3URL(t *testing.T) {
	tests := []struct {
		name string
		s3   *doltv1alpha.S3BackupStorage
		want string
	}{
		{
			name: "bucket only",
			s3: &doltv1alpha.S3BackupStorage{
				Bucket: "backup-bucket",
			},
			want: "aws://[backup-bucket:backup-bucket]",
		},
		{
			name: "bucket with prefix",
			s3: &doltv1alpha.S3BackupStorage{
				Bucket: "backup-bucket",
				Region: "eu-west-1",
				Prefix: "dolt/prod",
			},
			want: "aws://[backup-bucket:backup-bucket]/dolt/prod",
		},
		{
			name: "custom DynamoDB table",
			s3: &doltv1alpha.S3BackupStorage{
				Bucket:        "backup-bucket",
				DynamoDBTable: "my-custom-table",
				Prefix:        "backups",
			},
			want: "aws://[my-custom-table:backup-bucket]/backups",
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

func newTestScheme() *runtime.Scheme {
	scheme := runtime.NewScheme()
	_ = doltv1alpha.AddToScheme(scheme)
	_ = corev1.AddToScheme(scheme)
	return scheme
}

func TestValidateSecretRefs(t *testing.T) {
	ctx := context.Background()
	scheme := newTestScheme()

	validSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "aws-creds",
			Namespace: "default",
		},
		Data: map[string][]byte{
			"access-key-id":     []byte("AKIA..."),
			"secret-access-key": []byte("secret"),
		},
	}

	t.Run("valid secret refs", func(t *testing.T) {
		client := fake.NewClientBuilder().WithScheme(scheme).WithObjects(validSecret).Build()
		r := &Reconciler{Client: client}

		err := r.validateSecretRefs(ctx, "default", &doltv1alpha.S3BackupStorage{
			AccessKeyIdSecretKeyRef: &doltv1alpha.SecretKeySelector{
				LocalObjectReference: doltv1alpha.LocalObjectReference{Name: "aws-creds"},
				Key:                  "access-key-id",
			},
			SecretAccessKeySecretKeyRef: &doltv1alpha.SecretKeySelector{
				LocalObjectReference: doltv1alpha.LocalObjectReference{Name: "aws-creds"},
				Key:                  "secret-access-key",
			},
		})
		assert.NoError(t, err)
	})

	t.Run("secret does not exist", func(t *testing.T) {
		client := fake.NewClientBuilder().WithScheme(scheme).Build()
		r := &Reconciler{Client: client}

		err := r.validateSecretRefs(ctx, "default", &doltv1alpha.S3BackupStorage{
			AccessKeyIdSecretKeyRef: &doltv1alpha.SecretKeySelector{
				LocalObjectReference: doltv1alpha.LocalObjectReference{Name: "nonexistent"},
				Key:                  "access-key-id",
			},
		})
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "not found")
	})

	t.Run("key does not exist in secret", func(t *testing.T) {
		client := fake.NewClientBuilder().WithScheme(scheme).WithObjects(validSecret).Build()
		r := &Reconciler{Client: client}

		err := r.validateSecretRefs(ctx, "default", &doltv1alpha.S3BackupStorage{
			AccessKeyIdSecretKeyRef: &doltv1alpha.SecretKeySelector{
				LocalObjectReference: doltv1alpha.LocalObjectReference{Name: "aws-creds"},
				Key:                  "wrong-key",
			},
		})
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "key 'wrong-key' not found")
	})

	t.Run("nil refs are skipped", func(t *testing.T) {
		client := fake.NewClientBuilder().WithScheme(scheme).Build()
		r := &Reconciler{Client: client}

		err := r.validateSecretRefs(ctx, "default", &doltv1alpha.S3BackupStorage{
			Bucket: "my-bucket",
		})
		assert.NoError(t, err)
	})

	t.Run("wrong namespace", func(t *testing.T) {
		client := fake.NewClientBuilder().WithScheme(scheme).WithObjects(validSecret).Build()
		r := &Reconciler{Client: client}

		err := r.validateSecretRefs(ctx, "other-namespace", &doltv1alpha.S3BackupStorage{
			AccessKeyIdSecretKeyRef: &doltv1alpha.SecretKeySelector{
				LocalObjectReference: doltv1alpha.LocalObjectReference{Name: "aws-creds"},
				Key:                  "access-key-id",
			},
		})
		assert.Error(t, err)
	})
}

func TestEnsureS3EnvVars(t *testing.T) {
	ctx := context.Background()
	scheme := newTestScheme()

	t.Run("rejects nonexistent secret", func(t *testing.T) {
		doltdb := &doltv1alpha.DoltDB{
			ObjectMeta: metav1.ObjectMeta{Name: "test-doltdb", Namespace: "default"},
		}
		client := fake.NewClientBuilder().WithScheme(scheme).WithObjects(doltdb).Build()
		r := &Reconciler{Client: client}

		err := r.EnsureS3EnvVars(ctx, doltdb, &doltv1alpha.S3BackupStorage{
			Bucket: "my-bucket",
			AccessKeyIdSecretKeyRef: &doltv1alpha.SecretKeySelector{
				LocalObjectReference: doltv1alpha.LocalObjectReference{Name: "nonexistent"},
				Key:                  "key",
			},
		})
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "validating S3 secret references")
	})

	t.Run("skips when no secret refs (IRSA)", func(t *testing.T) {
		doltdb := &doltv1alpha.DoltDB{
			ObjectMeta: metav1.ObjectMeta{Name: "test-doltdb", Namespace: "default"},
		}
		client := fake.NewClientBuilder().WithScheme(scheme).WithObjects(doltdb).Build()
		r := &Reconciler{Client: client}

		err := r.EnsureS3EnvVars(ctx, doltdb, &doltv1alpha.S3BackupStorage{
			Bucket: "my-bucket",
			Region: "us-east-1",
		})
		// Region-only produces desired env vars but no secret refs to validate
		// This should still work since validateSecretRefs skips nil refs
		assert.NoError(t, err)
	})
}
