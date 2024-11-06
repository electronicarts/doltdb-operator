package builder

import (
	"fmt"

	doltv1alpha "github.com/electronicarts/doltdb-operator/api/v1alpha"
	v1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

// doltServicePorts returns the service ports for the Dolt cluster.
func doltServicePorts() []v1.ServicePort {
	return []v1.ServicePort{
		{
			Port: DoltMySQLPort,
			Name: DoltMySQLPortName,
		},
	}
}

// BuildDoltInternalService creates a headless service for the Dolt cluster.
func (b *Builder) BuildDoltInternalService(doltdb *doltv1alpha.DoltCluster) (*v1.Service, error) {
	objMeta := NewMetadataBuilder(doltdb.InternalServiceKey()).
		WithMetadata(&doltdb.ObjectMeta).Build()

	labels := NewLabelsBuilder().WithDoltSelectorLabels(doltdb).Build()

	svc := &v1.Service{
		ObjectMeta: objMeta,
		Spec: v1.ServiceSpec{
			Ports:     doltServicePorts(),
			ClusterIP: "None",
			Selector:  labels,
		},
	}

	if err := controllerutil.SetControllerReference(doltdb, svc, b.scheme); err != nil {
		return nil, fmt.Errorf("error setting controller reference to Service: %v", err)
	}

	return svc, nil
}

// BuildDoltPrimaryService creates a primary service for the Dolt cluster.
func (b *Builder) BuildDoltPrimaryService(doltdb *doltv1alpha.DoltCluster) (*v1.Service, error) {
	objMeta := NewMetadataBuilder(doltdb.PrimaryServiceKey()).
		WithMetadata(&doltdb.ObjectMeta).Build()

	labels := NewLabelsBuilder().
		WithDoltSelectorLabels(doltdb).
		WithPodPrimaryRole().
		WithStatefulSetPod(doltdb, *doltdb.Status.CurrentPrimaryPodIndex).
		Build()

	svc := &v1.Service{
		ObjectMeta: objMeta,
		Spec: v1.ServiceSpec{
			Ports:    doltServicePorts(),
			Type:     v1.ServiceTypeClusterIP,
			Selector: labels,
		},
	}

	if err := controllerutil.SetControllerReference(doltdb, svc, b.scheme); err != nil {
		return nil, fmt.Errorf("error setting controller reference to Service: %v", err)
	}

	return svc, nil
}

// BuildDoltReaderService creates a reader service for the Dolt cluster.
func (b *Builder) BuildDoltReaderService(doltdb *doltv1alpha.DoltCluster) (*v1.Service, error) {
	objMeta := NewMetadataBuilder(doltdb.ReaderServiceKey()).
		WithMetadata(&doltdb.ObjectMeta).Build()

	labels := NewLabelsBuilder().WithDoltSelectorLabels(doltdb).WithPodStandbyRole().Build()

	svc := &v1.Service{
		ObjectMeta: objMeta,
		Spec: v1.ServiceSpec{
			Ports:    doltServicePorts(),
			Type:     v1.ServiceTypeClusterIP,
			Selector: labels,
		},
	}

	if err := controllerutil.SetControllerReference(doltdb, svc, b.scheme); err != nil {
		return nil, fmt.Errorf("error setting controller reference to Service: %v", err)
	}

	return svc, nil
}
