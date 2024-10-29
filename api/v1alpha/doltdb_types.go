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

// DoltClusterSpec defines the desired state of DoltCluster
type DoltClusterSpec struct {
	// EngineVersion defines the version of the Dolt DB server to use.
	EngineVersion string `json:"engineVersion"`

	// ServiceAccountName defines the service account for the operator
	// +optional
	ServiceAccountName *string `json:"serviceAccountName,omitempty"`

	// +kubebuilder:default:="dolthub/dolt"

	// Image specifies the container image name.
	// +optional
	Image string `json:"image,omitempty"`

	// +optional
	ImagePullSecrets []v1.LocalObjectReference `json:"imagePullSecrets,omitempty"`

	// +optional
	PodSecurityContext v1.PodSecurityContext `json:"securityContext,omitempty"`

	// +optional
	Affinity *v1.Affinity `json:"affinity,omitempty"`

	// +optional
	Tolerations []v1.Toleration `json:"tolerations,omitempty"`

	// +optional
	NodeSelector map[string]string `json:"nodeSelector,omitempty"`

	// PodDisruptionBudget defines the budget for replica availability.
	// +optional
	PodDisruptionBudget *PodDisruptionBudget `json:"podDisruptionBudget,omitempty"`

	// +optional
	Resources *v1.ResourceRequirements `json:"resource,omitempty"`

	// Volume defines the volume configuration for the Dolt cluster.
	Storage Storage `json:"storage"`

	// +kubebuilder:validation:Minimum=2

	// Specifies the number of replicas for the Dolt cluster.
	// +optional
	Replicas int32 `json:"replicas"`

	// +kubebuilder:validation:Minimum=10
	// +kubebuilder:default:=128

	// Specifies the number of replicas for the Dolt cluster.
	// Default will be 128 connetions.
	// +optional
	MaxConnections *int32 `json:"maxConnections,omitempty"`

	// +kubebuilder:validation:Enum=DirectStandby;Remote
	// +kubebuilder:default:="DirectStandby"

	// Specifies the type of the Dolt cluster. Valid values are:
	//  - DirectStandby (default): Direct-to-standby replication
	//  - Remote: Remote-based replication
	// +optional
	ReplicationStrategy ClusterType `json:"replicationStrategy,omitempty"`

	// +kubebuilder:default:=false

	// AutoMinorVersionUpgrade
	// Enable or disable auto_minor_version_upgrade
	// +optional
	AutoMinorVersionUpgrade *bool `json:"autoMinorVersionUpgrade,omitempty"`

	// UpdateStrategy defines the update strategy for the StatefulSet object.
	// +optional
	UpdateStrategy *appsv1.StatefulSetUpdateStrategy `json:"updateStrategy,omitempty"`

	// Replication configures high availability via replication. This feature is still in alpha, use Galera if you are looking for a more production-ready HA.
	// +optional
	Replication *Replication `json:"replication,omitempty"`

	// +optional
	TopologySpreadConstrains []v1.TopologySpreadConstraint `json:"topologySpreadConstraints,omitempty"`
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
type Storage struct {
	// +kubebuilder:default:="100Gi"

	// Size specifies the size of the volume for the Dolt Cluster.
	Size *resource.Quantity `json:"size,omitempty"`

	// Storage class defines the storage class for the volume,
	// otherwise k8s cluster default will be used.
	// +optional
	StorageClassName *string `json:"storageClassName,omitempty"`

	// +kubebuilder:default:=true

	// ResizeInUseVolumes indicates whether the PVCs can be resized. The 'StorageClassName' used should have 'allowVolumeExpansion' set to 'true' to allow resizing.
	// +optional
	ResizeInUseVolumes *bool `json:"resizeInUseVolumes,omitempty"`

	// +kubebuilder:default:=true

	// WaitForVolumeResize indicates whether to wait for the PVCs to be resized before marking the DoltDB object as ready. This will block other operations such as cluster recovery while the resize is in progress.
	// It defaults to true.
	// +optional
	WaitForVolumeResize *bool `json:"waitForVolumeResize,omitempty"`
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

// DoltClusterStatus defines the observed state of DoltCluster
type DoltClusterStatus struct {
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
	ReplicationEpoch *int `json:"replication_epoch,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status

// DoltCluster is the Schema for the doltclusters API
type DoltCluster struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   DoltClusterSpec   `json:"spec,omitempty"`
	Status DoltClusterStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// DoltClusterList contains a list of DoltCluster
type DoltClusterList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []DoltCluster `json:"items"`
}

// ListItems gets a copy of the Items slice.
func (m *DoltClusterList) ListItems() []client.Object {
	items := make([]client.Object, len(m.Items))
	for i, item := range m.Items {
		items[i] = item.DeepCopy()
	}
	return items
}

func init() {
	SchemeBuilder.Register(&DoltCluster{}, &DoltClusterList{})
}
