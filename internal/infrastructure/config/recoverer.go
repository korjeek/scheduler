package config

import "time"

type Recoverer struct {
	Interval        time.Duration `mapstructure:"interval"`
	QueryTimeout    time.Duration `mapstructure:"query_timeout"`
	ShutdownTimeout time.Duration `mapstructure:"shutdown_timeout"`
}
