//go:build integration

package server_test

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	"scheduler/internal/domain/entities"
	"scheduler/internal/infrastructure/config"
	"scheduler/internal/infrastructure/server"
	"scheduler/internal/infrastructure/server/mocks"
	"scheduler/pkg/pb"
)

func getFreePort(t *testing.T) int {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	port := listener.Addr().(*net.TCPAddr).Port
	require.NoError(t, listener.Close())
	return port
}

func waitForPort(t *testing.T, addr string) {
	t.Helper()
	require.Eventually(t, func() bool {
		conn, err := net.DialTimeout("tcp", addr, 100*time.Millisecond)
		if err == nil {
			_ = conn.Close()
			return true
		}
		return false
	}, 2*time.Second, 50*time.Millisecond, "server did not start on %s", addr)
}

func newTestServer(t *testing.T, devMode bool) (*server.Server, string, string, *mocks.MockTaskService, *mocks.MockCronParser) {
	t.Helper()

	ctrl := gomock.NewController(t)
	t.Cleanup(ctrl.Finish)

	mockSvc := mocks.NewMockTaskService(ctrl)
	mockParser := mocks.NewMockCronParser(ctrl)
	handler := server.NewTaskHandler(mockSvc, mockParser)

	httpPort := getFreePort(t)
	grpcPort := getFreePort(t)

	httpAddr := fmt.Sprintf("127.0.0.1:%d", httpPort)
	grpcAddr := fmt.Sprintf("127.0.0.1:%d", grpcPort)

	cfg := config.Server{
		HttpAddr:          httpAddr,
		GrpcAddr:          grpcAddr,
		ReadTimeout:       5 * time.Second,
		ReadHeaderTimeout: 5 * time.Second,
		WriteTimeout:      5 * time.Second,
		IdleTimeout:       30 * time.Second,
		ShutdownTimeout:   5 * time.Second,
		MaxHeaderBytes:    1 << 20,
		DevMode:           devMode,
		RateLimiter: config.RateLimiter{
			RequestsPerSecond: 1000,
			Burst:             1000,
		},
	}

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	srv := server.NewServerManager(handler, cfg, logger)

	return srv, httpAddr, grpcAddr, mockSvc, mockParser
}

func TestServerIntegration_StartShutdown(t *testing.T) {
	srv, httpAddr, grpcAddr, mockSvc, mockParser := newTestServer(t, false)

	startCtx, cancelStart := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancelStart()
	require.NoError(t, srv.Start(startCtx))

	waitForPort(t, grpcAddr)
	waitForPort(t, httpAddr)

	grpcConn, err := grpc.NewClient(
		"passthrough:///"+grpcAddr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	require.NoError(t, err)
	defer func(grpcConn *grpc.ClientConn) { _ = grpcConn.Close() }(grpcConn)

	grpcClient := pb.NewTaskServiceClient(grpcConn)

	now := time.Now().UTC()
	taskID := uuid.New()
	expectedTask := &entities.Task{
		ID:        taskID,
		Name:      "grpc-integration",
		QueueName: "default",
		Status:    entities.StatusPending,
		NextRunAt: &now,
		CreatedAt: &now,
	}

	mockParser.EXPECT().Parse("").Times(0)
	mockSvc.EXPECT().
		Create(gomock.Any(), gomock.Any()).
		Return(expectedTask, nil)

	createResp, err := grpcClient.CreateTask(context.Background(), &pb.CreateTaskRequest{
		Name:       "grpc-integration",
		QueueName:  "default",
		MaxRetries: 1,
		Payload:    []byte("{}"),
	})
	require.NoError(t, err)
	assert.Equal(t, taskID.String(), createResp.Id)

	mockSvc.EXPECT().
		List(gomock.Any(), 10, 0).
		Return([]entities.Task{*expectedTask}, nil)

	httpResp, err := http.Get(fmt.Sprintf("http://%s/api/tasks?limit=10&offset=0", httpAddr))
	require.NoError(t, err)
	defer func(Body io.ReadCloser) { _ = Body.Close() }(httpResp.Body)
	assert.Equal(t, http.StatusOK, httpResp.StatusCode)

	shutdownCtx, cancelShutdown := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancelShutdown()
	require.NoError(t, srv.Shutdown(shutdownCtx))

	_, err = net.DialTimeout("tcp", grpcAddr, 100*time.Millisecond)
	assert.Error(t, err, "gRPC port should be closed")
	_, err = net.DialTimeout("tcp", httpAddr, 100*time.Millisecond)
	assert.Error(t, err, "HTTP port should be closed")
}

func TestServerIntegration_DevMode(t *testing.T) {
	srv, httpAddr, _, _, _ := newTestServer(t, true)

	startCtx, cancelStart := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancelStart()
	require.NoError(t, srv.Start(startCtx))

	waitForPort(t, httpAddr)

	resp, err := http.Get("http://" + httpAddr + "/swagger/doc.json")
	require.NoError(t, err)
	defer func(Body io.ReadCloser) { _ = Body.Close() }(resp.Body)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Contains(t, resp.Header.Get("Content-Type"), "application/json")

	respUI, err := http.Get("http://" + httpAddr + "/swagger/")
	require.NoError(t, err)
	defer func(Body io.ReadCloser) { _ = Body.Close() }(respUI.Body)
	assert.Equal(t, http.StatusOK, respUI.StatusCode)
}

func TestServerIntegration_ListenError(t *testing.T) {
	lis, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	defer func(lis net.Listener) { _ = lis.Close() }(lis)
	occupiedAddr := lis.Addr().String()

	ctrl := gomock.NewController(t)
	t.Cleanup(ctrl.Finish)

	mockSvc := mocks.NewMockTaskService(ctrl)
	mockParser := mocks.NewMockCronParser(ctrl)
	handler := server.NewTaskHandler(mockSvc, mockParser)

	cfg := config.Server{
		HttpAddr:          "127.0.0.1:0",
		GrpcAddr:          occupiedAddr,
		ReadTimeout:       5 * time.Second,
		ReadHeaderTimeout: 5 * time.Second,
		WriteTimeout:      5 * time.Second,
		IdleTimeout:       30 * time.Second,
		ShutdownTimeout:   5 * time.Second,
		MaxHeaderBytes:    1 << 20,
		DevMode:           false,
		RateLimiter: config.RateLimiter{
			RequestsPerSecond: 1000,
			Burst:             1000,
		},
	}

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	srv := server.NewServerManager(handler, cfg, logger)

	err = srv.Start(context.Background())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to listen tcp port")
}
