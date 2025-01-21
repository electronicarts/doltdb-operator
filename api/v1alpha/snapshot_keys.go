package v1alpha

import (
	"fmt"

	"k8s.io/apimachinery/pkg/types"
)

// CronJobKey defines the key for the CronJob for each VolumeSnapshot. appends the name with job
func (d *Snapshot) CronJobKey(name string) types.NamespacedName {
	return types.NamespacedName{
		Name:      fmt.Sprintf("%s-job", name),
		Namespace: d.Namespace,
	}
}

// ConfigMapKey defines the key for the ConfigMap used in Cronjob for each VolumeSnapshot- appends the name with cm
func (d *Snapshot) ConfigMapKey(name string) types.NamespacedName {
	return types.NamespacedName{
		Name:      fmt.Sprintf("%s-cm", name),
		Namespace: d.Namespace,
	}
}
