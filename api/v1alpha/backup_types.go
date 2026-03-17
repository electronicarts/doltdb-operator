// Copyright (c) 2025 Electronic Arts Inc. All rights reserved.

package v1alpha

import (
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	// DefaultBackoffLimit is the default number of retries for a backup.
	DefaultBackoffLimit int32 = 2
)

// BackupPhase represents the current phase of a backup.
// +kubebuilder:validation:Enum=Pending;Running;Completed;Failed
type BackupPhase string

const (
	BackupPhasePending   BackupPhase = "Pending"
	BackupPhaseRunning   BackupPhase = "Running"
	BackupPhaseCompleted BackupPhase = "Completed"
	BackupPhaseFailed    BackupPhase = "Failed"
)

// BackupStorage defines the storage backend for backups. Exactly one must be specified.
// +kubebuilder:validation:MinProperties=1
// +kubebuilder:validation:MaxProperties=1
type BackupStorage struct {
	// S3 defines an S3-compatible storage backend for backups.
	// +optional
	S3 *S3BackupStorage `json:"s3,omitempty"`
	// DoltHub defines a DoltHub remote storage backend for backups.
	// +optional
	DoltHub *DoltHubBackupStorage `json:"doltHub,omitempty"`
	// Local defines a local filesystem storage backend for backups.
	// +optional
	Local *LocalBackupStorage `json:"local,omitempty"`
}

// S3BackupStorage configures an S3-compatible storage backend.
type S3BackupStorage struct {
	// Bucket is the S3 bucket name.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MaxLength=63
	// +kubebuilder:validation:Pattern:=`^[a-z0-9][a-z0-9.-]+[a-z0-9]$`
	Bucket string `json:"bucket"`
	// Region is the AWS region for the bucket.
	// +optional
	Region string `json:"region,omitempty"`
	// Endpoint is a custom endpoint for S3-compatible services (e.g. MinIO).
	// +optional
	Endpoint string `json:"endpoint,omitempty"`
	// Prefix is the path prefix within the bucket.
	// +optional
	// +kubebuilder:validation:MaxLength=512
	Prefix string `json:"prefix,omitempty"`
	// DynamoDBTable is the DynamoDB table name used by Dolt as a manifest store.
	// Defaults to the bucket name if not specified (Dolt convention).
	// +optional
	// +kubebuilder:validation:MaxLength=255
	DynamoDBTable string `json:"dynamoDBTable,omitempty"`
	// ForcePathStyle enables path-style addressing for S3-compatible services.
	// +optional
	ForcePathStyle *bool `json:"forcePathStyle,omitempty"`
	// AccessKeyIdSecretKeyRef is a reference to a secret containing the AWS access key ID.
	// +optional
	AccessKeyIdSecretKeyRef *SecretKeySelector `json:"accessKeyIdSecretKeyRef,omitempty"`
	// SecretAccessKeySecretKeyRef is a reference to a secret containing the AWS secret access key.
	// +optional
	SecretAccessKeySecretKeyRef *SecretKeySelector `json:"secretAccessKeySecretKeyRef,omitempty"`
}

// DoltHubBackupStorage configures a DoltHub remote storage backend.
type DoltHubBackupStorage struct {
	// RemoteURL is the DoltHub remote API URL.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MaxLength=512
	RemoteURL string `json:"remoteUrl"`
	// CredentialsSecretKeyRef is a reference to a secret containing DoltHub credentials.
	// +optional
	CredentialsSecretKeyRef *SecretKeySelector `json:"credentialsSecretKeyRef,omitempty"`
}

// LocalBackupStorage configures a local filesystem storage backend.
type LocalBackupStorage struct {
	// Path is the filesystem path for the backup.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MaxLength=255
	// +kubebuilder:validation:Pattern:=`^/[a-zA-Z0-9_./-]+$`
	Path string `json:"path"`
}

// BackupSpec defines the desired state of Backup.
//
// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="backup spec is immutable"
// +kubebuilder:validation:XValidation:rule="!has(self.databases) || self.databases.all(d, d.matches('^[a-zA-Z0-9_]+$'))",message="bad name"
// +kubebuilder:validation:XValidation:rule="!has(self.databases) || self.databases.all(d, size(d) <= 80)",message="name too long"
type BackupSpec struct {
	// DoltDBRef is a reference to the DoltDB cluster to back up.
	// +kubebuilder:validation:Required
	DoltDBRef DoltDBRef `json:"doltDBRef"`
	// Storage defines where the backup will be stored.
	// +kubebuilder:validation:Required
	Storage BackupStorage `json:"storage"`
	// Databases is the list of databases to back up. If empty, all databases are backed up.
	// +optional
	// +kubebuilder:validation:MaxItems=100
	// +kubebuilder:validation:items:MaxLength=80
	Databases []string `json:"databases,omitempty"`
	// BackoffLimit specifies the number of retries before marking the backup as failed.
	// +kubebuilder:default:=2
	// +optional
	BackoffLimit *int32 `json:"backoffLimit,omitempty"`
	// Resources defines the compute resources for the backup job.
	// +optional
	Resources *corev1.ResourceRequirements `json:"resources,omitempty"`
	// ImagePullSecrets specifies the secrets to use for pulling container images.
	// +optional
	ImagePullSecrets []corev1.LocalObjectReference `json:"imagePullSecrets,omitempty"`
}

// BackupStatus defines the observed state of Backup.
type BackupStatus struct {
	// Conditions for the Backup object.
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`
	// Phase is the current phase of the backup.
	// +optional
	Phase BackupPhase `json:"phase,omitempty"`
	// StartedAt is the time when the backup started.
	// +optional
	StartedAt *metav1.Time `json:"startedAt,omitempty"`
	// CompletedAt is the time when the backup completed.
	// +optional
	CompletedAt *metav1.Time `json:"completedAt,omitempty"`
	// Error contains the error message if the backup failed.
	// +optional
	Error string `json:"error,omitempty"`
	// RetryCount tracks the number of failed reconciliation attempts.
	// +optional
	RetryCount int32 `json:"retryCount,omitempty"`
}

// SetCondition sets or updates a status condition on the Backup.
func (in *BackupStatus) SetCondition(condition metav1.Condition) {
	if in.Conditions == nil {
		in.Conditions = make([]metav1.Condition, 0)
	}
	meta.SetStatusCondition(&in.Conditions, condition)
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Phase",type="string",JSONPath=".status.phase"
// +kubebuilder:printcolumn:name="DoltDB",type="string",JSONPath=".spec.doltDBRef.name"
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"

// Backup is the Schema for the backups API.
type Backup struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   BackupSpec   `json:"spec,omitempty"`
	Status BackupStatus `json:"status,omitempty"`
}

// DoltDBRef returns a pointer to the DoltDB reference.
func (b *Backup) DoltDBRef() *DoltDBRef {
	return &b.Spec.DoltDBRef
}

// GetBackoffLimit returns the backoff limit or the default.
func (b *Backup) GetBackoffLimit() int32 {
	if b.Spec.BackoffLimit != nil {
		return *b.Spec.BackoffLimit
	}
	return DefaultBackoffLimit
}

// IsReady indicates whether the Backup is completed successfully.
func (b *Backup) IsReady() bool {
	return meta.IsStatusConditionTrue(b.Status.Conditions, ConditionTypeReady)
}

// IsCompleted returns true if the backup phase is Completed.
func (b *Backup) IsCompleted() bool {
	return b.Status.Phase == BackupPhaseCompleted
}

// IsFailed returns true if the backup phase is Failed.
func (b *Backup) IsFailed() bool {
	return b.Status.Phase == BackupPhaseFailed
}

// IsRunning returns true if the backup phase is Running.
func (b *Backup) IsRunning() bool {
	return b.Status.Phase == BackupPhaseRunning
}

// +kubebuilder:object:root=true

// BackupList contains a list of Backup.
type BackupList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Backup `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Backup{}, &BackupList{})
}
