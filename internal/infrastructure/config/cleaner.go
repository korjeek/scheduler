package config

import "time"

type Cleaner struct {
	Enabled         bool          `mapstructure:"enabled" default:"true"`
	Interval        time.Duration `mapstructure:"interval" default:"1h"`
	RetentionPeriod time.Duration `mapstructure:"retention_period" default:"720h"`
	BatchSize       int           `mapstructure:"batch_size" default:"1000"`
	BatchDelay      time.Duration `mapstructure:"batch_delay" default:"100ms"`
	ShutdownTimeout time.Duration `mapstructure:"shutdown_timeout" default:"10s"`
	QueryTimeout    time.Duration `mapstructure:"query_timeout" default:"5s"`
}
