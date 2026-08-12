package config

import "time"

type Database struct {
	ConnString        string        `mapstructure:"connection_string"`
	MinCons           int32         `mapstructure:"min_cons"`
	MaxCons           int32         `mapstructure:"max_cons"`
	MaxConnLifetime   time.Duration `mapstructure:"max_connection_lifetime"`
	MaxConnIdleTime   time.Duration `mapstructure:"max_connection_idle_time"`
	HealthCheckPeriod time.Duration `mapstructure:"health_check_period"`
	ConnectTimeout    time.Duration `mapstructure:"connect_timeout"`
	MigrationTimeout  time.Duration `mapstructure:"migration_timeout"`
}
