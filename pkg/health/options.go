package health

import "k8s.io/utils/ptr"

type HealthOpts struct {
	DesiredReplicas int32
	Port            *int32
	EndpointPolicy  *EndpointPolicy
}

type HealthOpt func(*HealthOpts)

func WithDesiredReplicas(r int32) HealthOpt {
	return func(ho *HealthOpts) {
		ho.DesiredReplicas = r
	}
}

func WithPort(p int32) HealthOpt {
	return func(ho *HealthOpts) {
		ho.Port = ptr.To(p)
	}
}

func WithEndpointPolicy(e EndpointPolicy) HealthOpt {
	return func(ho *HealthOpts) {
		ho.EndpointPolicy = ptr.To(e)
	}
}
