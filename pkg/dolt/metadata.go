// Copyright (c) 2025 Electronic Arts Inc. All rights reserved.

package dolt

const (
	RoleLabel             = "k8s.dolthub.com/cluster-role"
	VolumeRoleLabel       = "pvc.k8s.dolthub.com/role"
	WatchLabel            = "k8s.dolthub.com/watch"
	Annotation            = "k8s.dolthub.com/doltdb"
	ReplicationAnnotation = "k8s.dolthub.com/replication"
	// ConfigMapHashAnnotationKey is the annotation key used to store the ConfigMap content hash
	// in the pod template. When the ConfigMap content changes, this hash changes, causing
	// Kubernetes to treat the pod template as updated and trigger pod restarts.
	ConfigMapHashAnnotation = "k8s.dolthub.com/configmap-hash"

	UserFinalizerName     = "user.k8s.dolthub.com/finalizer"
	DatabaseFinalizerName = "database.k8s.dolthub.com/finalizer"
	GrantFinalizerName    = "grant.k8s.dolthub.com/finalizer"
)

type Role string

const (
	PrimaryRoleValue Role = "primary"
	StandbyRoleValue Role = "standby"
)

func (d Role) String() string {
	return string(d)
}
