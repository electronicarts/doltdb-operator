/*
Copyright 2024.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package v1alpha

import (
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// SnapshotSpec defines the desired state of Snapshot
type SnapshotSpec struct {
	// How often a snapshot should be created against the persistent volume. Value can be provided
	// as a cron expression in string. Default is every day at midnight "0 0 * * *".
	// +kubebuilder:validation:Required
	FrequencySchedule *string `json:"frequencySchedule"`
	// DoltDBRef is a reference to a Doltdb object.
	// +kubebuilder:validation:Required
	DoltDBRef DoltDBRef `json:"doltDBRef"`
	// Image specifies the container to run as cron job.
	// +kubebuilder:default:="bitnami/kubectl"
	// +optional
	Image string `json:"image,omitempty"`
	// Image Version the container to run as cron job.
	// +kubebuilder:default:="latest"
	// +optional
	Version string `json:"version,omitempty"`
	// ImagePullSecrets specifies the secrets to use for pulling the container image.
	// +optional
	ImagePullSecrets []v1.LocalObjectReference `json:"imagePullSecrets,omitempty"`
}

// SnapshotStatus defines the observed state of Snapshot
type SnapshotStatus struct {
	// Conditions for the VolumeSnapshot object.
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

func (in *SnapshotStatus) SetCondition(condition metav1.Condition) {
	if in.Conditions == nil {
		in.Conditions = make([]metav1.Condition, 0)
	}
	meta.SetStatusCondition(&in.Conditions, condition)
}

func (d *Snapshot) IsReady() bool {
	return meta.IsStatusConditionTrue(d.Status.Conditions, ConditionTypeSnapshotCreated)
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Ready",type="string",JSONPath=".status.conditions[?(@.type==\"Ready\")].status"
// +kubebuilder:printcolumn:name="Status",type="string",JSONPath=".status.conditions[?(@.type==\"Ready\")].message"
// +kubebuilder:printcolumn:name="DoltDB",type="string",JSONPath=".spec.doltDBRef.name"
// Snapshot is the Schema for the snapshots API
type Snapshot struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   SnapshotSpec   `json:"spec,omitempty"`
	Status SnapshotStatus `json:"status,omitempty"`
}

func (d *Snapshot) DoltDBRef() *DoltDBRef {
	return &d.Spec.DoltDBRef
}

// +kubebuilder:object:root=true

// SnapshotList contains a list of Snapshot
type SnapshotList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Snapshot `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Snapshot{}, &SnapshotList{})
}
