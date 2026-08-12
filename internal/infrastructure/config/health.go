package config

import "time"

type Health struct {
	Address          string        `mapstructure:"address" default:":9090"`
	ReadTimeout      time.Duration `mapstructure:"read_timeout" default:"2s"`
	WriteTimeout     time.Duration `mapstructure:"write_timeout" default:"2s"`
	ShutdownTimeout  time.Duration `mapstructure:"shutdown_timeout" default:"5s"`
	ComponentName    string        `mapstructure:"component_name" default:"scheduler"`
	ComponentVersion string        `mapstructure:"component_version" default:"1.0.0"`
}
