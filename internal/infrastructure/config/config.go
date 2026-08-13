package config

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/viper"
	"go.uber.org/fx"
)

type AppConfig struct {
	Database  Database  `mapstructure:"database"`
	Server    Server    `mapstructure:"server"`
	Cleaner   Cleaner   `mapstructure:"cleaner"`
	Recoverer Recoverer `mapstructure:"recoverer"`
	Poller    Poller    `mapstructure:"poller"`
	Health    Health    `mapstructure:"health"`
}

var Module = fx.Module("config",
	fx.Provide(
		Load,
		func(cfg *AppConfig) Database { return cfg.Database },
		func(cfg *AppConfig) Server { return cfg.Server },
		func(cfg *AppConfig) Cleaner { return cfg.Cleaner },
		func(cfg *AppConfig) Recoverer { return cfg.Recoverer },
		func(cfg *AppConfig) Poller { return cfg.Poller },
		func(cfg *AppConfig) Health { return cfg.Health },
	),
)

func Load() (*AppConfig, error) {
	v := viper.New()

	v.SetConfigName("config")
	v.SetConfigType("yaml")

	v.AddConfigPath(".")
	v.AddConfigPath("./config")
	v.AddConfigPath("/etc/app")

	if envPath := os.Getenv("CONFIG_PATH"); envPath != "" {
		v.SetConfigFile(envPath)
	}

	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_", "-", "_"))
	v.AutomaticEnv()

	setDefaults(v)

	if err := v.ReadInConfig(); err != nil {
		if _, ok := errors.AsType[viper.ConfigFileNotFoundError](err); ok {
			fmt.Println("Configuration file not found. Loading default config.yaml")
		}
	}

	var cfg AppConfig
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config: %w", err)
	}

	return &cfg, nil
}

func setDefaults(v *viper.Viper) {
	// database
	v.SetDefault("database.connection_string", "")
	v.SetDefault("database.min_cons", 1)
	v.SetDefault("database.max_cons", 20)
	v.SetDefault("database.max_connection_lifetime", "30m")
	v.SetDefault("database.max_connection_idle_time", "15m")
	v.SetDefault("database.health_check_period", "1m")
	v.SetDefault("database.connect_timeout", "30s")
	v.SetDefault("database.migration_timeout", "30s")

	// recoverer
	v.SetDefault("recoverer.interval", "15s")
	v.SetDefault("recoverer.query_timeout", "5s")
	v.SetDefault("recoverer.shutdown_timeout", "10s")

	// servers
	v.SetDefault("server.http_server", "0.0.0.0:8080")
	v.SetDefault("server.grpc_address", "0.0.0.0:50051")
	v.SetDefault("server.read_timeout", "10s")
	v.SetDefault("server.read_header_timeout", "5s")
	v.SetDefault("server.write_timeout", "10s")
	v.SetDefault("server.idle_timeout", "120s")
	v.SetDefault("server.shutdown_timeout", "30s")
	v.SetDefault("server.max_header_bytes", 1048576)
	v.SetDefault("server.rate_limit.requests_per_second", 100.0)
	v.SetDefault("server.rate_limit.burst", 200)

	// clients
	v.SetDefault("clients.http.timeout", "30s")
	v.SetDefault("clients.http.retry_count", 3)
	v.SetDefault("clients.http.retry_wait_time", "5s")
	v.SetDefault("clients.http.max_idle_connections", 100)
	v.SetDefault("clients.http.max_connections_per_host", 20)
	v.SetDefault("clients.http.idle_connection_timeout", "90s")

	// poller
	v.SetDefault("poller.poll_interval", "1s")
	v.SetDefault("poller.lock_duration", "30s")

	// cleaner
	v.SetDefault("cleaner.enabled", true)
	v.SetDefault("cleaner.interval", "1h")
	v.SetDefault("cleaner.retention_period", "720h")
	v.SetDefault("cleaner.batch_size", 1000)
	v.SetDefault("cleaner.batch_delay", "100ms")
	v.SetDefault("cleaner.shutdown_timeout", "10s")
	v.SetDefault("cleaner.query_timeout", "5s")

	// health
	v.SetDefault("health.address", ":9090")
	v.SetDefault("health.read_timeout", "2s")
	v.SetDefault("health.write_timeout", "2s")
	v.SetDefault("health.shutdown_timeout", "5s")
	v.SetDefault("health.component_name", "scheduler")
	v.SetDefault("health.component_version", "1.0.0")
}
