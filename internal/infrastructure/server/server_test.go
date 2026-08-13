//go:build integration

package server_test

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"scheduler/internal/infrastructure/server"
	"scheduler/internal/infrastructure/server/mocks"
	"testing"
	"time"

	"io"
	"log/slog"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	"scheduler/internal/domain/entities"
	"scheduler/internal/infrastructure/config"
	"scheduler/pkg/pb"
)

func getFreePort(t *testing.T) int {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	port := listener.Addr().(*net.TCPAddr).Port
	_ = listener.Close()
	return port
}

func TestServerIntegration_StartShutdown(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockSvc := mocks.NewMockTaskService(ctrl)
	mockParser := mocks.NewMockCronParser(ctrl)

	handler := server.NewTaskHandler(mockSvc, mockParser)

	httpPort := getFreePort(t)
	grpcPort := getFreePort(t)

	cfg := config.Server{
		HttpAddr:          fmt.Sprintf("127.0.0.1:%d", httpPort),
		GrpcAddr:          fmt.Sprintf("127.0.0.1:%d", grpcPort),
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

	startCtx, cancelStart := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancelStart()
	require.NoError(t, srv.Start(startCtx))

	grpcAddr := cfg.GrpcAddr
	require.Eventually(t, func() bool {
		conn, err := net.DialTimeout("tcp", grpcAddr, 100*time.Millisecond)
		if err == nil {
			_ = conn.Close()
			return true
		}
		return false
	}, 2*time.Second, 50*time.Millisecond, "gRPC server not ready")

	httpURL := "http://" + cfg.HttpAddr
	require.Eventually(t, func() bool {
		conn, err := net.DialTimeout("tcp", cfg.HttpAddr, 100*time.Millisecond)
		if err == nil {
			_ = conn.Close()
			return true
		}
		return false
	}, 2*time.Second, 50*time.Millisecond, "HTTP server not ready")

	grpcConn, err := grpc.NewClient("passthrough:///"+grpcAddr,
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

	httpResp, err := http.Get(fmt.Sprintf("%s/api/tasks?limit=10&offset=0", httpURL))
	require.NoError(t, err)
	defer func(Body io.ReadCloser) { _ = Body.Close() }(httpResp.Body)
	assert.Equal(t, http.StatusOK, httpResp.StatusCode)

	shutdownCtx, cancelShutdown := context.WithTimeout(context.Background(), cfg.ShutdownTimeout)
	defer cancelShutdown()
	require.NoError(t, srv.Shutdown(shutdownCtx))

	_, err = net.DialTimeout("tcp", grpcAddr, 100*time.Millisecond)
	assert.Error(t, err, "gRPC port should be closed after shutdown")

	_, err = net.DialTimeout("tcp", cfg.HttpAddr, 100*time.Millisecond)
	assert.Error(t, err, "HTTP port should be closed after shutdown")
}
