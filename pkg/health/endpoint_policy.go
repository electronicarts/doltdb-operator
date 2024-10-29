package health

type EndpointPolicy string

const (
	EndpointPolicyAll        EndpointPolicy = "All"
	EndpointPolicyAtLeastOne EndpointPolicy = "AtLeastOne"
)
