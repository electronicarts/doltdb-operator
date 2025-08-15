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
	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// DoltDBSpec defines the desired state of DoltDB
type DoltDBSpec struct {
	// EngineVersion defines the version of the Dolt DB server to use.
	EngineVersion string `json:"engineVersion"`
	// ServiceAccountName defines the service account for the operator.
	// +optional
	ServiceAccountName *string `json:"serviceAccountName,omitempty"`
	// Image specifies the container image name for the Dolt DB server.
	// +kubebuilder:default:="dolthub/dolt"
	// +optional
	Image string `json:"image,omitempty"`
	// ImagePullSecrets specifies the secrets to use for pulling the container image.
	// +optional
	ImagePullSecrets []v1.LocalObjectReference `json:"imagePullSecrets,omitempty"`
	// PodSecurityContext defines the security context for the pod.
	// +optional
	PodSecurityContext v1.PodSecurityContext `json:"securityContext,omitempty"`
	// PodAnnotations defines the annotations for the pod.
	// +optional
	PodAnnotations map[string]string `json:"podAnnotations,omitempty"`
	// Affinity defines the affinity rules for the pod.
	// +optional
	Affinity *v1.Affinity `json:"affinity,omitempty"`
	// Tolerations defines the tolerations for the pod.
	// +optional
	Tolerations []v1.Toleration `json:"tolerations,omitempty"`
	// NodeSelector defines the node selector for the pod.
	// +optional
	NodeSelector map[string]string `json:"nodeSelector,omitempty"`
	// PodDisruptionBudget defines the budget for replica availability.
	// +optional
	PodDisruptionBudget *PodDisruptionBudget `json:"podDisruptionBudget,omitempty"`
	// Resources defines the resource requirements for the pod.
	// +optional
	Resources *v1.ResourceRequirements `json:"resources,omitempty"`
	// Storage defines the volume configuration for the Dolt cluster.
	Storage Storage `json:"storage"`
	// Replicas specifies the number of replicas for the Dolt cluster.
	// +kubebuilder:validation:Minimum=2
	// +optional
	Replicas int32 `json:"replicas"`
	// DEPRECATED: MaxConnections specifies the maximum number of connections for the Dolt cluster.
	// Default is 128 connections.
	// +kubebuilder:validation:Minimum=10
	// +kubebuilder:default:=128
	// +optional
	MaxConnections *int32 `json:"maxConnections,omitempty"`
	// ReplicationStrategy specifies the type of replication for the Dolt cluster.
	// Valid values are:
	//  - DirectStandby (default): Direct-to-standby replication
	//  - Remote: Remote-based replication
	// +kubebuilder:validation:Enum=DirectStandby;Remote
	// +kubebuilder:default:="DirectStandby"
	// +optional
	ReplicationStrategy ClusterType `json:"replicationStrategy,omitempty"`
	// AutoMinorVersionUpgrade enables or disables automatic minor version upgrades.
	// +kubebuilder:default:=false
	// +optional
	AutoMinorVersionUpgrade *bool `json:"autoMinorVersionUpgrade,omitempty"`
	// UpdateStrategy defines the update strategy for the StatefulSet object.
	// +optional
	UpdateStrategy *appsv1.StatefulSetUpdateStrategy `json:"updateStrategy,omitempty"`
	// Replication configures high availability via replication.
	// This feature is still in alpha; use Galera for a more production-ready HA solution.
	// +optional
	Replication *Replication `json:"replication,omitempty"`
	// TopologySpreadConstraints defines the topology spread constraints for the pod.
	// +optional
	TopologySpreadConstrains []v1.TopologySpreadConstraint `json:"topologySpreadConstraints,omitempty"`
	// Probes defines the liveness and readiness probes for the DoltDB pods.
	// +optional
	Probes Probes `json:"probes"`
	// Server defines the server configuration for the DoltDB server.
	// +optional
	Server Server `json:"server,omitempty"`
	// GlobalConfig defines the global configuration for the DoltDB server.
	// +optional
	GlobalConfig GlobalConfig `json:"globalConfig,omitempty"`
}

// PodDisruptionBudget is the Pod availability bundget for a DoltDB
type PodDisruptionBudget struct {
	// MinAvailable defines the number of minimum available Pods.
	// +optional
	MinAvailable *intstr.IntOrString `json:"minAvailable,omitempty"`
	// MaxUnavailable defines the number of maximum unavailable Pods.
	// +optional
	MaxUnavailable *intstr.IntOrString `json:"maxUnavailable,omitempty"`
}

// Volume defines a single volume in the manifest.
// Storage defines the volume configuration for the Dolt cluster.
type Storage struct {
	// Size specifies the size of the volume for the Dolt Cluster.
	// +kubebuilder:default:="100Gi"
	// +optional
	Size *resource.Quantity `json:"size,omitempty"`
	// StorageClassName defines the storage class for the volume.
	// If not specified, the default storage class of the Kubernetes cluster will be used.
	// +optional
	StorageClassName *string `json:"storageClassName,omitempty"`
	// ResizeInUseVolumes indicates whether the PersistentVolumeClaims (PVCs) can be resized while in use.
	// The StorageClass used should have 'allowVolumeExpansion' set to 'true' to allow resizing.
	// +kubebuilder:default:=true
	// +optional
	ResizeInUseVolumes *bool `json:"resizeInUseVolumes,omitempty"`
	// WaitForVolumeResize indicates whether to wait for the PVCs to be resized before marking the DoltDB object as ready.
	// This will block other operations such as cluster recovery while the resize is in progress.
	// Defaults to true.
	// +kubebuilder:default:=true
	// +optional
	WaitForVolumeResize *bool `json:"waitForVolumeResize,omitempty"`
	// RetentionPolicy describes the policy used for PVCs created from the StatefulSet VolumeClaimTemplates.
	// +optional
	RetentionPolicy *appsv1.StatefulSetPersistentVolumeClaimRetentionPolicy `json:"retentionPolicy,omitempty"`
	// VolumeSnapshot defines the volume snapshot name to restore DoltDB from snapshot.
	// +optional
	VolumeSnapshot string `json:"volumeSnapshot,omitempty"`
}

// Probes defines the liveness and readiness probes for the DoltDB pods.
type Probes struct {
	// LivenessProbe defines the liveness probe for the pod.
	// +kubebuilder:default:={initialDelaySeconds: 15, periodSeconds: 40}
	// +optional
	LivenessProbe *v1.Probe `json:"livenessProbe,omitempty"`
	// ReadinessProbe defines the readiness probe for the pod.
	// +kubebuilder:default:={initialDelaySeconds: 15, periodSeconds: 40}
	// +optional
	ReadinessProbe *v1.Probe `json:"readinessProbe,omitempty"`
}

// ClusterType defines the type of the Dolt cluster. It can be either
// direct-to-standby or remote-based
// Check out https://docs.dolthub.com/sql-reference/server/replication for more information
// +kubebuilder:validation:Enum=DirectStandby;Remote
type ClusterType string

const (
	// DirectStandby enables Direct-to-standby replication
	DirectStandby ClusterType = "DirectStandby"

	// Remote enables Remote-based replication
	Remote ClusterType = "Remote"
)

// UpdateType defines the type of update for a Dolt Cluster resource.
type UpdateType string

const (
	// ReplicasFirstPrimaryLastUpdateType indicates that the update will be applied to all replica Pods first and later on to the primary Pod.
	// The updates are applied one by one waiting until each Pod passes the readiness probe
	// i.e. the Pod gets synced and it is ready to receive traffic.
	ReplicasFirstPrimaryLastUpdateType UpdateType = "ReplicasFirstPrimaryLast"
	// RollingUpdateUpdateType indicates that the update will be applied by the StatefulSet controller using the RollingUpdate strategy.
	// This strategy is unaware of the roles that the Pod have (primary or replica) and it will
	// perform the update following the StatefulSet ordinal, from higher to lower.
	RollingUpdateUpdateType UpdateType = "RollingUpdate"
	// OnDeleteUpdateType indicates that the update will be applied by the StatefulSet controller using the OnDelete strategy.
	// The update will be done when the Pods get manually deleted by the user.
	OnDeleteUpdateType UpdateType = "OnDelete"
	// NeverUpdateType indicates that the StatefulSet will never be updated.
	// This can be used to roll out updates progressively to a fleet of instances.
	NeverUpdateType UpdateType = "Never"
)

// DoltDBStatus defines the observed state of DoltDB
type DoltDBStatus struct {
	// Conditions for the Dolt cluster object.
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`
	// CurrentPrimaryPodIndex is the primary Pod index.
	// +optional
	CurrentPrimaryPodIndex *int `json:"currentPrimaryPodIndex,omitempty"`
	// CurrentPrimary is the primary Pod.
	// +optional
	CurrentPrimary *string `json:"currentPrimary,omitempty"`
	// Replicas current number of replicas
	Replicas int32 `json:"replicas,omitempty"`
	// ReplicationStatus is the replication current state for each Pod.
	// +optional
	ReplicationStatus ReplicationStatus `json:"replicationStatus,omitempty"`
	// ReplicationEpoch holds dolt highest epoch value to perform switchovers
	// +optional
	ReplicationEpoch *int `json:"replicationEpoch,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Ready",type="string",JSONPath=".status.conditions[?(@.type==\"Ready\")].status"
// +kubebuilder:printcolumn:name="Status",type="string",JSONPath=".status.conditions[?(@.type==\"Ready\")].message"
// +kubebuilder:printcolumn:name="CurrentPrimary",type="string",JSONPath=".status.currentPrimary"
// +kubebuilder:printcolumn:name="ReplicationEpoch",type="string",JSONPath=".status.replicationEpoch"
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"

// DoltDB is the Schema for the doltdbs API
type DoltDB struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   DoltDBSpec   `json:"spec,omitempty"`
	Status DoltDBStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// DoltDBList contains a list of DoltDB
type DoltDBList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []DoltDB `json:"items"`
}

// ListItems gets a copy of the Items slice.
func (m *DoltDBList) ListItems() []client.Object {
	items := make([]client.Object, len(m.Items))
	for i, item := range m.Items {
		items[i] = item.DeepCopy()
	}
	return items
}

func init() {
	SchemeBuilder.Register(&DoltDB{}, &DoltDBList{})
}
