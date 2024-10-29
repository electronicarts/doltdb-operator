package v1alpha

const (
	// ReasonReplicationConfiguring indicates that replication is being configured.
	ReasonReplicationConfiguring = "ReplicationConfiguring"
	// ReasonReplicationConfigured indicates that replication has been configured.
	ReasonReplicationConfigured = "ReplicationConfigured"
	// ReasonReplicationPrimaryLock indicates that primary tables have a read lock.
	ReasonReplicationPrimaryLock = "PrimaryLock"
	// ReasonReplicationPrimaryReadonly indicates that primary is being changed to readonly mode.
	ReasonReplicationPrimaryReadonly = "PrimaryReadonly"
	// ReasonReplicationReplicaSync indicates that replicas are being synced with primary.
	ReasonReplicationReplicaSync = "ReplicaSync"
	// ReasonReplicationReplicaSyncErr indicates that an error has happened while replicas were being synced with primary.
	ReasonReplicationReplicaSyncErr = "ReplicaSyncErr"
	// ReasonReplicationPrimaryNew indicates that a new primary is being configured.
	ReasonReplicationPrimaryNew = "PrimaryNew"
	// ReasonReplicationReplicaConn indicates that replicas are connecting to the new primary.
	ReasonReplicationReplicaConn = "ReplicaConn"
	// ReasonReplicationPrimaryToReplica indicates that current primary is being unlocked to become a replica.
	ReasonReplicationPrimaryToReplica = "PrimaryToReplica"
	// ReasonPrimarySwitching indicates that primary is being switched.
	ReasonPrimarySwitching = "PrimarySwitching"
	// ReasonPrimarySwitched indicates that primary has been switched.
	ReasonPrimarySwitched = "PrimarySwitched"
	// ReasonCRDNotFound indicates that a third party CRD is not present in the cluster.
	ReasonCRDNotFound = "CRDNotFound"
)
