// Copyright (c) 2025 Electronic Arts Inc. All rights reserved.

package builder

import (
	"fmt"

	doltv1alpha "github.com/electronicarts/doltdb-operator/api/v1alpha"
	v1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

// doltServicePorts returns the service ports for the Dolt cluster.
func doltServicePorts(doltdb *doltv1alpha.DoltDB) []v1.ServicePort {
	ports := []v1.ServicePort{
		{
			Port: doltdb.Spec.Server.Listener.Port,
			Name: DoltMySQLPortName,
		},
	}

	if doltdb.Spec.Server.MCPServer != nil {
		ports = append(ports, v1.ServicePort{
			Port: doltdb.Spec.Server.MCPServer.Port,
			Name: DoltMCPPortName,
		})
	}

	return ports
}

// doltInternalServicePorts returns the service ports for the internal headless service.
func doltInternalServicePorts(doltdb *doltv1alpha.DoltDB) []v1.ServicePort {
	ports := []v1.ServicePort{
		{
			Port: doltdb.Spec.Server.Listener.Port,
			Name: DoltMySQLPortName,
		},
		{
			Port: doltdb.Spec.Server.Cluster.RemotesAPI.Port,
			Name: DoltRemotesAPIPortName,
		},
	}

	if doltdb.Spec.Server.MCPServer != nil {
		ports = append(ports, v1.ServicePort{
			Port: doltdb.Spec.Server.MCPServer.Port,
			Name: DoltMCPPortName,
		})
	}

	return ports
}

// BuildDoltInternalService creates a headless service for the Dolt cluster.
func (b *Builder) BuildDoltInternalService(doltdb *doltv1alpha.DoltDB) (*v1.Service, error) {
	objMeta := NewMetadataBuilder(doltdb.InternalServiceKey()).
		WithMetadata(&doltdb.ObjectMeta).Build()

	labels := NewLabelsBuilder().WithDoltSelectorLabels(doltdb).Build()

	svc := &v1.Service{
		ObjectMeta: objMeta,
		Spec: v1.ServiceSpec{
			Ports:     doltInternalServicePorts(doltdb),
			ClusterIP: "None",
			Selector:  labels,
		},
	}

	if err := controllerutil.SetControllerReference(doltdb, svc, b.scheme); err != nil {
		return nil, fmt.Errorf("error setting controller reference to Service: %v", err)
	}

	return svc, nil
}

// BuildDoltService creates a service for standalone mode when replication is disabled.
func (b *Builder) BuildDoltService(doltdb *doltv1alpha.DoltDB) (*v1.Service, error) {
	objMeta := NewMetadataBuilder(doltdb.ServiceKey()).
		WithMetadata(&doltdb.ObjectMeta).Build()

	labels := NewLabelsBuilder().WithDoltSelectorLabels(doltdb).Build()

	svc := &v1.Service{
		ObjectMeta: objMeta,
		Spec: v1.ServiceSpec{
			Ports:    doltServicePorts(doltdb),
			Type:     v1.ServiceTypeClusterIP,
			Selector: labels,
		},
	}

	if err := controllerutil.SetControllerReference(doltdb, svc, b.scheme); err != nil {
		return nil, fmt.Errorf("error setting controller reference to Service: %v", err)
	}

	return svc, nil
}

// BuildDoltPrimaryService creates a primary service for the Dolt cluster.
func (b *Builder) BuildDoltPrimaryService(doltdb *doltv1alpha.DoltDB) (*v1.Service, error) {
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
			Ports:    doltServicePorts(doltdb),
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
func (b *Builder) BuildDoltReaderService(doltdb *doltv1alpha.DoltDB) (*v1.Service, error) {
	objMeta := NewMetadataBuilder(doltdb.ReaderServiceKey()).
		WithMetadata(&doltdb.ObjectMeta).Build()

	labels := NewLabelsBuilder().WithDoltSelectorLabels(doltdb).WithPodStandbyRole().Build()

	svc := &v1.Service{
		ObjectMeta: objMeta,
		Spec: v1.ServiceSpec{
			Ports:    doltServicePorts(doltdb),
			Type:     v1.ServiceTypeClusterIP,
			Selector: labels,
		},
	}

	if err := controllerutil.SetControllerReference(doltdb, svc, b.scheme); err != nil {
		return nil, fmt.Errorf("error setting controller reference to Service: %v", err)
	}

	return svc, nil
}
