package v1alpha

import "k8s.io/utils/ptr"

type GlobalConfig struct {
	// DisableClientUsageMetricsCollection defines the global configuration for enabling client usage metrics collection.
	// Reference: https://docs.dolthub.com/other/faq#does-dolt-collect-client-metrics-how-can-i-disable-it
	// +kubebuilder:default:=false
	// +optional
	DisableClientUsageMetricsCollection *bool `json:"disableClientUsageMetricsCollection,omitempty"`
	// CommitAuthor defines the commit author configuration for the DoltDB server.
	// +optional
	CommitAuthor CommitAuthor `json:"commitAuthor,omitempty"`
}

// CommitAuthor defines the commit author configuration for the DoltDB server.
type CommitAuthor struct {
	// Name is the name of the commit author.
	// +kubebuilder:default:="dolt kubernetes deployment"
	// +optional
	Name string `json:"name,omitempty"`
	// Email is the email of the commit author.
	// +kubebuilder:default:="dolt@kubernetes.deployment"
	// +optional
	Email string `json:"email,omitempty"`
}

func (s *GlobalConfig) ApplyDefaults() {
	if s.DisableClientUsageMetricsCollection == nil {
		s.DisableClientUsageMetricsCollection = ptr.To(false)
	}

	s.CommitAuthor.ApplyDefaults()
}

func (c *CommitAuthor) ApplyDefaults() {
	if c.Name == "" {
		c.Name = "dolt kubernetes deployment"
	}
	if c.Email == "" {
		c.Email = "dolt@kubernetes.deployment"
	}
}
