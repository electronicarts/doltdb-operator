package dolt

const (
	RoleLabel  = "k8s.dolthub.com/cluster-role"
	VolumeRole = "pvc.k8s.dolthub.com/role"

	Annotation            = "k8s.dolthub.com/doltdb"
	ReplicationAnnotation = "k8s.dolthub.com/replication"
)

type Role string

const (
	PrimaryRoleValue Role = "primary"
	StandbyRoleValue Role = "standby"
)

func (d Role) String() string {
	return string(d)
}
