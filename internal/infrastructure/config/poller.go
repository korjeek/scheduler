package config

import "time"

type Poller struct {
	PollInterval time.Duration `mapstructure:"poll_interval"`
	LockDuration time.Duration `mapstructure:"lock_duration"`
}
