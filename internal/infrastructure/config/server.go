package config

import "time"

type Server struct {
	DevMode           bool          `mapstructure:"dev_mode"`
	HttpAddr          string        `mapstructure:"http_address"`
	GrpcAddr          string        `mapstructure:"grpc_address"`
	ReadTimeout       time.Duration `mapstructure:"read_timeout"`
	ReadHeaderTimeout time.Duration `mapstructure:"read_header_timeout"`
	WriteTimeout      time.Duration `mapstructure:"write_timeout"`
	IdleTimeout       time.Duration `mapstructure:"idle_timeout"`
	ShutdownTimeout   time.Duration `mapstructure:"shutdown_timeout"`
	MaxHeaderBytes    int           `mapstructure:"max_header_bytes"`
	RateLimiter       RateLimiter   `mapstructure:"rate_limiter"`
}

type RateLimiter struct {
	RequestsPerSecond float64 `mapstructure:"requests_per_second"`
	Burst             int     `mapstructure:"burst"`
}
