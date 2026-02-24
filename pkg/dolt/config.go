// Copyright (c) 2025 Electronic Arts Inc. All rights reserved.

package dolt

// Reference: https://docs.dolthub.com/sql-reference/server/configuration

const (
	// LogLevel defines the logging level for the application.
	LogLevel = "trace"
	// MaxConnections is the maximum number of connections allowed.
	MaxConnections int32 = 128
	// RemotesAPIPort is the port number for the remotes API.
	RemotesAPIPort int32 = 50051
	// DatabasePortName is the name of the port used in the Service.
	DatabasePortName string = "dolt"
)

// Config represents the structure of the configuration data.
type Config struct {
	Behavior Behavior `yaml:"behavior"`
	LogLevel string   `yaml:"log_level"`
	Cluster  *Cluster `yaml:"cluster,omitempty"`
	Listener Listener `yaml:"listener"`
	Metrics  Metrics  `yaml:"metrics,omitempty"`
}

type Behavior struct {
	AutoGCBehavior AutoGCBehavior `yaml:"auto_gc_behavior"`
}

type AutoGCBehavior struct {
	Enable bool `yaml:"enable"`
}

// Metrics represents the metrics section of the configuration.
type Metrics struct {
	Labels map[string]string `yaml:"labels"`
	Host   string            `yaml:"host"`
	Port   int32             `yaml:"port"`
}

// Cluster represents the cluster section of the configuration.
type Cluster struct {
	StandbyRemotes []StandbyRemote `yaml:"standby_remotes"`
	BootstrapEpoch int32           `yaml:"bootstrap_epoch"`
	BootstrapRole  string          `yaml:"bootstrap_role"`
	RemotesAPI     RemotesAPI      `yaml:"remotesapi"`
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
