package v1alpha

type Server struct {
	// Profiler defines the profiler configuration for the DoltDB server.
	// +optional
	Profiler Profiler `json:"profiler,omitempty"`
	// Behavior defines the behavior configuration for the DoltDB server.
	// +optional
	Behavior `json:"behavior,omitempty"`
	// LogLevel defines the logging level for the DoltDB server.
	// +kubebuilder:default:="trace"
	// +optional
	LogLevel string `json:"logLevel,omitempty"`
	// Metrics defines the metrics configuration for the DoltDB server.
	// +optional
	Metrics *Metrics `json:"metrics,omitempty"`
	// Listener defines the listener configuration for the DoltDB server.
	// +optional
	Listener Listener `json:"listener,omitempty"`
	// Cluster defines the cluster configuration for the DoltDB server.
	// +optional
	Cluster Cluster `json:"cluster,omitempty"`
}

type Profiler struct {
	// Enabled is a flag to enable the profiler.
	// +kubebuilder:default:=false
	EnablePProf bool `json:"enablePProf,omitempty"`
}

// Behavior defines the behavior configuration for the DoltDB server.
type Behavior struct {
	// AutoGCBehavior defines the auto GC behavior for the DoltDB server.
	// +optional
	AutoGCBehavior AutoGCBehavior `json:"autoGCBehavior,omitempty"`
}

type AutoGCBehavior struct {
	// Enable is a flag to enable the auto GC behavior.
	// +kubebuilder:default:=false
	// +optional
	Enable bool `json:"enabled,omitempty"`
}

// Metrics defines the metrics configuration for the DoltDB server.
type Metrics struct {
	// Enabled is a flag to enable the metrics server.
	// +kubebuilder:default:=false
	// +optional
	Enabled bool `json:"enabled,omitempty"`
	// Labels defines the labels for the metrics server.
	// +optional
	Labels map[string]string `json:"labels,omitempty"`
	// Host defines the host for the metrics server.
	// +kubebuilder:default:="0.0.0.0"
	// +optional
	Host string `json:"host,omitempty"`
	// Port defines the port for the metrics server.
	// +kubebuilder:default:=11228
	// +optional
	Port int32 `json:"port,omitempty"`
}

// Listener defines the listener configuration for the DoltDB server.
type Listener struct {
	// Host defines the host for the metrics server.
	// +kubebuilder:default:="0.0.0.0"
	// +optional
	Host string `json:"host,omitempty"`
	// Port defines the port for the metrics server.
	// +kubebuilder:default:=3306
	// +optional
	Port int32 `json:"port,omitempty"`
	// MaxConnections specifies the maximum number of connections for DoltDB.
	// +kubebuilder:validation:Minimum=10
	// +kubebuilder:default:=1024
	// +optional
	MaxConnections int32 `json:"maxConnections,omitempty"`
}

// Cluster defines the cluster configuration for the DoltDB server.
type Cluster struct {
	// RemotesAPI defines the remotes API configuration for the DoltDB server.
	// +optional
	RemotesAPI RemotesAPI `json:"remotesAPI,omitempty"`
}

type RemotesAPI struct {
	// Port defines the port for the remotes API.
	// +kubebuilder:default:=50051
	// +optional
	Port int32 `json:"port,omitempty"`
}

func (s *Server) ApplyDefaults() {
	if s.LogLevel == "" {
		s.LogLevel = "trace"
	}
	if s.Metrics == nil {
		s.Metrics = &Metrics{}
	}

	s.Metrics.ApplyDefaults()
	s.Listener.ApplyDefaults()
	s.Cluster.ApplyDefaults()
}

func (m *Metrics) ApplyDefaults() {
	if !m.Enabled {
		m.Enabled = false
	}
	if m.Host == "" {
		m.Host = "0.0.0.0"
	}
	if m.Port == 0 {
		m.Port = 11228
	}
	if m.Labels == nil {
		m.Labels = make(map[string]string)
	}
}

func (l *Listener) ApplyDefaults() {
	if l.Host == "" {
		l.Host = "0.0.0.0"
	}
	if l.Port == 0 {
		l.Port = 3306
	}
	if l.MaxConnections == 0 {
		l.MaxConnections = 1024
	}
}

func (c *Cluster) ApplyDefaults() {
	if c.RemotesAPI.Port == 0 {
		c.RemotesAPI.Port = 50051
	}
}
