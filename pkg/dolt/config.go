package dolt

const (
	// LogLevel defines the logging level for the application.
	LogLevel = "trace"
	// MaxConnections is the maximum number of connections allowed.
	MaxConnections int32 = 128
	// RemotesAPIPort is the port number for the remotes API.
	RemotesAPIPort int32 = 50051
	// DatabasePort is the port number for the database.
	DatabasePort int32 = 3306
	// DatabasePortName is the name of the port used in the Service.
	DatabasePortName string = "dolt"
)

// Config represents the structure of the configuration data.
type Config struct {
	LogLevel   string     `yaml:"log_level"`
	Cluster    Cluster    `yaml:"cluster"`
	RemotesAPI RemotesAPI `yaml:"remotesapi"`
	Listener   Listener   `yaml:"listener"`
}

// Cluster represents the cluster section of the configuration.
type Cluster struct {
	StandbyRemotes []StandbyRemote `yaml:"standby_remotes"`
	BootstrapEpoch int32           `yaml:"bootstrap_epoch"`
	BootstrapRole  string          `yaml:"bootstrap_role"`
}

// StandbyRemote represents a standby remote in the cluster configuration.
type StandbyRemote struct {
	Name              string `yaml:"name"`
	RemoteURLTemplate string `yaml:"remote_url_template"`
}

// RemotesAPI represents the remotes API section of the configuration.
type RemotesAPI struct {
	Port int32 `yaml:"port"`
}

// Listener represents the listener section of the configuration.
type Listener struct {
	Host           string `yaml:"host"`
	Port           int32  `yaml:"port"`
	MaxConnections int32  `yaml:"max_connections"`
}
