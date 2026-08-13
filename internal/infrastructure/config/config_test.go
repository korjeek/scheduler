package config_test

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"scheduler/internal/infrastructure/config"
)

func TestLoad_Defaults(t *testing.T) {
	t.Setenv("CONFIG_PATH", filepath.Join(t.TempDir(), "no-config.yaml"))

	cfg, err := config.Load()
	require.NoError(t, err)
	require.NotNil(t, cfg)

	// Database
	assert.EqualValues(t, 1, cfg.Database.MinCons)
	assert.EqualValues(t, 20, cfg.Database.MaxCons)

	// Server
	assert.Equal(t, "0.0.0.0:8080", cfg.Server.HttpAddr)
	assert.Equal(t, "0.0.0.0:50051", cfg.Server.GrpcAddr)
	assert.Equal(t, 100.0, cfg.Server.RateLimiter.RequestsPerSecond)
	assert.Equal(t, 200, cfg.Server.RateLimiter.Burst)

	// Poller
	assert.Equal(t, time.Second, cfg.Poller.PollInterval)
	assert.Equal(t, 30*time.Second, cfg.Poller.LockDuration)

	// Cleaner
	assert.True(t, cfg.Cleaner.Enabled)

	// Health
	assert.Equal(t, ":9090", cfg.Health.Address)
}

func TestLoad_WithDefaultsOnly(t *testing.T) {
	t.Setenv("CONFIG_PATH", filepath.Join(t.TempDir(), "no-config.yaml"))

	cfg, err := config.Load()
	require.NoError(t, err)
	require.NotNil(t, cfg)

	assert.EqualValues(t, 1, cfg.Database.MinCons)
	assert.EqualValues(t, 20, cfg.Database.MaxCons)
	assert.Equal(t, "0.0.0.0:8080", cfg.Server.HttpAddr)
	assert.Equal(t, "0.0.0.0:50051", cfg.Server.GrpcAddr)
	assert.Equal(t, 100.0, cfg.Server.RateLimiter.RequestsPerSecond)
	assert.Equal(t, 200, cfg.Server.RateLimiter.Burst)
	assert.Equal(t, time.Second, cfg.Poller.PollInterval)
	assert.Equal(t, 30*time.Second, cfg.Poller.LockDuration)
	assert.True(t, cfg.Cleaner.Enabled)
	assert.Equal(t, ":9090", cfg.Health.Address)
	assert.Equal(t, "scheduler", cfg.Health.ComponentName)
}

func TestLoad_WithYAMLOverride(t *testing.T) {
	// Создаём временный YAML-файл
	dir := t.TempDir()
	yamlContent := `
database:
  connection_string: "postgres://custom:pass@localhost:5432/custom"
  min_cons: 5
`
	configFile := filepath.Join(dir, "config.yaml")
	require.NoError(t, os.WriteFile(configFile, []byte(yamlContent), 0644))

	t.Setenv("CONFIG_PATH", configFile)

	cfg, err := config.Load()
	require.NoError(t, err)
	require.NotNil(t, cfg)

	assert.Equal(t, "postgres://custom:pass@localhost:5432/custom", cfg.Database.ConnString)
	assert.Equal(t, int32(5), cfg.Database.MinCons)
}

func TestLoad_WithEnvOverride(t *testing.T) {
	t.Setenv("database_connection_string", "postgres://env:env@localhost:5432/envdb")

	cfg, err := config.Load()
	require.NoError(t, err)
	require.NotNil(t, cfg)

	assert.Equal(t, "postgres://env:env@localhost:5432/envdb", cfg.Database.ConnString)
}
