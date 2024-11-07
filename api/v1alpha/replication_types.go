package v1alpha

import (
	"k8s.io/utils/ptr"
)

// Replication allows you to enable single-master HA via semi-synchronours replication in your DoltDB cluster.
type Replication struct {
	// ReplicationSpec is the Replication desired state specification.
	ReplicationSpec `json:",inline"`
	// Enabled is a flag to enable Replication.
	// +optional
	Enabled bool `json:"enabled,omitempty"`
}

// PrimaryReplication is the replication configuration for the primary node.
type PrimaryReplication struct {
	// PodIndex is the StatefulSet index of the primary node. The user may change this field to perform a manual switchover.
	// +optional
	PodIndex *int `json:"podIndex,omitempty"`
	// AutomaticFailover indicates whether the operator should automatically update PodIndex to perform an automatic primary failover.
	// +optional
	AutomaticFailover *bool `json:"automaticFailover,omitempty"`
	// MinCaughtUpStandbys is the minimum number of standbys that must be caught up before a primary failover can occur.
	// +optional
	MinCaughtUpStandbys *int `json:"minCaughtUpStandby,omitempty"`
}

// FillWithDefaults fills the current PrimaryReplication object with DefaultReplicationSpec.
// This enables having minimal PrimaryReplication objects and provides sensible defaults.
func (r *PrimaryReplication) FillWithDefaults() {
	if r.PodIndex == nil {
		index := *DefaultReplicationSpec.Primary.PodIndex
		r.PodIndex = &index
	}
	if r.AutomaticFailover == nil {
		failover := *DefaultReplicationSpec.Primary.AutomaticFailover
		r.AutomaticFailover = &failover
	}
}

// ReplicationSpec is the Replication desired state specification.
type ReplicationSpec struct {
	// Primary is the replication configuration for the primary node.
	// +optional
	Primary *PrimaryReplication `json:"primary,omitempty"`
}

// FillWithDefaults fills the current ReplicationSpec object with DefaultReplicationSpec.
// This enables having minimal ReplicationSpec objects and provides sensible defaults.
func (r *ReplicationSpec) FillWithDefaults() {
	if r.Primary == nil {
		primary := *DefaultReplicationSpec.Primary
		r.Primary = &primary
	} else {
		r.Primary.FillWithDefaults()
	}
}

var (
	// DefaultReplicationSpec provides sensible defaults for the ReplicationSpec.
	DefaultReplicationSpec = ReplicationSpec{
		Primary: &PrimaryReplication{
			PodIndex:          ptr.To(0),
			AutomaticFailover: ptr.To(true),
		},
	}
)

// Replication with defaulting accessor
func (d *DoltDB) Replication() Replication {
	if d.Spec.Replication == nil {
		d.Spec.Replication = &Replication{}
	}
	d.Spec.Replication.FillWithDefaults()
	return *d.Spec.Replication
}

type ReplicationState string

const (
	ReplicationStatePrimary       ReplicationState = "primary"
	ReplicationStateStandby       ReplicationState = "standby"
	ReplicationStateNotConfigured ReplicationState = "not_configured"
)

type ReplicationStatus map[string]ReplicationState

func (r ReplicationStatus) IsReplicationConfigured() bool {
	anyReplicaConfigured := false
	for _, state := range r {
		if state == ReplicationStateNotConfigured {
			return false
		}
		if state == ReplicationStateStandby {
			anyReplicaConfigured = true
		}
	}
	// make sure at least one replica is configured. For example, this ensures that
	// a switchover/failover operation will not start if no replica has been configured.
	return anyReplicaConfigured
}

func (d *DoltDB) IsReplicationConfigured() bool {
	return d.Status.ReplicationStatus.IsReplicationConfigured()
}
