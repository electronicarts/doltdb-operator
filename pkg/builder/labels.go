package builder

import (
	doltv1alpha "github.com/electronicarts/doltdb-operator/api/v1alpha"
	"github.com/electronicarts/doltdb-operator/pkg/dolt"
	"github.com/electronicarts/doltdb-operator/pkg/statefulset"
)

const (
	appLabel           = "app.kubernetes.io/name"
	instanceLabel      = "app.kubernetes.io/instance"
	statefulSetPodName = "statefulset.kubernetes.io/pod-name"
	versionLabel       = "app.kubernetes.io/version"
)

type LabelsBuilder struct {
	labels map[string]string
}

// NewLabelsBuilder creates a new instance of LabelsBuilder.
func NewLabelsBuilder() *LabelsBuilder {
	return &LabelsBuilder{
		labels: map[string]string{},
	}
}

// WithApp sets the app label.
func (b *LabelsBuilder) WithApp(app string) *LabelsBuilder {
	b.labels[appLabel] = app
	return b
}

// WithApp sets the engine verison label
func (b *LabelsBuilder) WithVersion(version string) *LabelsBuilder {
	if version == "" {
		return b
	}

	b.labels[versionLabel] = version
	return b
}

// WithInstance sets the instance label.
func (b *LabelsBuilder) WithInstance(instance string) *LabelsBuilder {
	b.labels[instanceLabel] = instance
	return b
}

// WithStatefulSetPod sets the stateful set pod name label.
func (b *LabelsBuilder) WithStatefulSetPod(doltdb *doltv1alpha.DoltDB, podIndex int) *LabelsBuilder {
	b.labels[statefulSetPodName] = statefulset.PodName(doltdb.ObjectMeta, podIndex)
	return b
}

// WithLabels adds multiple labels to the builder.
func (b *LabelsBuilder) WithLabels(labels map[string]string) *LabelsBuilder {
	for k, v := range labels {
		b.labels[k] = v
	}
	return b
}

// WithDoltSelectorLabels sets the app and instance labels for a DoltDB.
func (b *LabelsBuilder) WithDoltSelectorLabels(doltdb *doltv1alpha.DoltDB) *LabelsBuilder {
	return b.WithApp(doltdb.Name)
}

// WithPVCRole sets the PVC role label.
func (b *LabelsBuilder) WithPVCRole(role string) *LabelsBuilder {
	b.labels[dolt.VolumeRoleLabel] = role
	return b
}

// WithPodRole sets the pod role label to primary.
func (b *LabelsBuilder) WithPodPrimaryRole() *LabelsBuilder {
	b.labels[dolt.RoleLabel] = dolt.PrimaryRoleValue.String()
	return b
}

// WithPodStandbyRole sets the pod role label to standby.
func (b *LabelsBuilder) WithPodStandbyRole() *LabelsBuilder {
	b.labels[dolt.RoleLabel] = dolt.StandbyRoleValue.String()
	return b
}

// Build returns the constructed labels map.
func (b *LabelsBuilder) Build() map[string]string {
	return b.labels
}
