//go:build integration

package health_test

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"

	"scheduler/internal/infrastructure/config"
	"scheduler/internal/infrastructure/health"
)

func TestHealthServerIntegration(t *testing.T) {
	ctx := context.Background()

	pgContainer, err := postgres.Run(ctx,
		"postgres:17-alpine",
		postgres.WithDatabase("testdb"),
		postgres.WithUsername("testuser"),
		postgres.WithPassword("testpass"),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2).
				WithStartupTimeout(30*time.Second),
		),
	)
	require.NoError(t, err)
	defer func() { _ = pgContainer.Terminate(ctx) }()

	connStr, err := pgContainer.ConnectionString(ctx, "sslmode=disable")
	require.NoError(t, err)

	poolCfg, err := pgxpool.ParseConfig(connStr)
	require.NoError(t, err)

	pool, err := pgxpool.NewWithConfig(ctx, poolCfg)
	require.NoError(t, err)
	defer pool.Close()

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	port := listener.Addr().(*net.TCPAddr).Port
	_ = listener.Close()

	healthCfg := config.Health{
		Address:          fmt.Sprintf("127.0.0.1:%d", port),
		ReadTimeout:      2 * time.Second,
		WriteTimeout:     2 * time.Second,
		ShutdownTimeout:  5 * time.Second,
		ComponentName:    "scheduler-test",
		ComponentVersion: "1.0.0",
	}

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	hs, err := health.NewHealthServer(pool, logger, healthCfg)
	require.NoError(t, err)

	startCtx, cancelStart := context.WithTimeout(ctx, 5*time.Second)
	defer cancelStart()
	require.NoError(t, hs.Start(startCtx))

	baseURL := fmt.Sprintf("http://%s", healthCfg.Address)

	require.Eventually(t, func() bool {
		conn, err := net.DialTimeout("tcp", healthCfg.Address, 100*time.Millisecond)
		if err == nil {
			_ = conn.Close()
			return true
		}
		return false
	}, 2*time.Second, 50*time.Millisecond, "health server did not start in time")

	resp, err := http.Get(baseURL + "/healthz")
	require.NoError(t, err)
	defer func(Body io.ReadCloser) { _ = Body.Close() }(resp.Body)
	require.Equal(t, http.StatusOK, resp.StatusCode)

	resp, err = http.Get(baseURL + "/readyz")
	require.NoError(t, err)
	defer func(Body io.ReadCloser) { _ = Body.Close() }(resp.Body)
	require.Equal(t, http.StatusOK, resp.StatusCode)

	require.NoError(t, pgContainer.Terminate(ctx))
	pool.Close()

	require.Eventually(t, func() bool {
		resp, err := http.Get(baseURL + "/readyz")
		if err != nil {
			return false
		}
		defer func(Body io.ReadCloser) { _ = Body.Close() }(resp.Body)
		return resp.StatusCode == http.StatusServiceUnavailable
	}, 5*time.Second, 100*time.Millisecond, "readiness did not become unavailable")

	// Останавливаем health-сервер
	shutdownCtx, cancelShutdown := context.WithTimeout(ctx, healthCfg.ShutdownTimeout)
	defer cancelShutdown()
	require.NoError(t, hs.Shutdown(shutdownCtx))
}
