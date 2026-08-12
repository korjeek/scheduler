package server_test

import (
	"context"
	"errors"
	"net"
	"scheduler/internal/infrastructure/server"
	"scheduler/internal/infrastructure/server/mocks"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"
	"google.golang.org/protobuf/proto"

	"scheduler/internal/domain/entities"
	"scheduler/pkg/pb"
)

const bufSize = 1024 * 1024

func setupGRPCServer(t *testing.T, handler *server.TaskHandler) (pb.TaskServiceClient, *grpc.Server) {
	lis := bufconn.Listen(bufSize)
	s := grpc.NewServer()
	pb.RegisterTaskServiceServer(s, handler)
	go func() {
		if err := s.Serve(lis); err != nil {
			panic(err)
		}
	}()

	conn, err := grpc.NewClient("passthrough:///bufnet",
		grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) {
			return lis.Dial()
		}),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	require.NoError(t, err)

	return pb.NewTaskServiceClient(conn), s
}

// ----------------- CreateTask -----------------
func TestCreateTaskIntegration_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockSvc := mocks.NewMockTaskService(ctrl)
	mockParser := mocks.NewMockCronParser(ctrl)
	handler := server.NewTaskHandler(mockSvc, mockParser)

	client, grpcSrv := setupGRPCServer(t, handler)
	defer grpcSrv.GracefulStop()

	now := time.Now().UTC()
	taskID := uuid.New()
	expected := &entities.Task{
		ID:        taskID,
		Name:      "success",
		QueueName: "default",
		Status:    entities.StatusPending,
		NextRunAt: &now,
		CreatedAt: &now,
	}

	mockParser.EXPECT().Parse("@every 1m").Return(nil, nil)
	mockSvc.EXPECT().
		Create(gomock.Any(), gomock.Any()).
		Return(expected, nil)

	resp, err := client.CreateTask(context.Background(), &pb.CreateTaskRequest{
		Name:       "success",
		QueueName:  "default",
		MaxRetries: 2,
		Payload:    []byte("{}"),
		Cron:       proto.String("@every 1m"),
	})
	require.NoError(t, err)
	assert.Equal(t, taskID.String(), resp.Id)
	assert.Equal(t, pb.TaskStatus_TASK_STATUS_PENDING, resp.Status)
	assert.NotNil(t, resp.NextRunAt)
}

func TestCreateTaskIntegration_InvalidCron(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockSvc := mocks.NewMockTaskService(ctrl)
	mockParser := mocks.NewMockCronParser(ctrl)
	handler := server.NewTaskHandler(mockSvc, mockParser)

	client, grpcSrv := setupGRPCServer(t, handler)
	defer grpcSrv.GracefulStop()

	mockParser.EXPECT().Parse("bad").Return(nil, errors.New("invalid expression"))

	_, err := client.CreateTask(context.Background(), &pb.CreateTaskRequest{
		Name: "test",
		Cron: proto.String("bad"),
	})
	require.Error(t, err)
	st, ok := status.FromError(err)
	require.True(t, ok)
	assert.Equal(t, codes.InvalidArgument, st.Code())
}

// ----------------- GetTask -----------------
func TestGetTaskIntegration_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockSvc := mocks.NewMockTaskService(ctrl)
	mockParser := mocks.NewMockCronParser(ctrl)
	handler := server.NewTaskHandler(mockSvc, mockParser)

	client, grpcSrv := setupGRPCServer(t, handler)
	defer grpcSrv.GracefulStop()

	now := time.Now().UTC()
	taskID := uuid.New()
	taskEntity := &entities.Task{
		ID:         taskID,
		Name:       "test",
		Cron:       "",
		Status:     entities.StatusSuccess,
		MaxRetries: 1,
		NextRunAt:  &now,
		CreatedAt:  &now,
	}

	mockSvc.EXPECT().Get(gomock.Any(), taskID.String()).Return(taskEntity, nil)

	resp, err := client.GetTask(context.Background(), &pb.GetTaskRequest{Id: taskID.String()})
	require.NoError(t, err)
	assert.Equal(t, taskID.String(), resp.Id)
	assert.Equal(t, pb.TaskStatus_TASK_STATUS_SUCCESS, resp.Status)
}

// ----------------- ListTasks -----------------
func TestListTasksIntegration_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockSvc := mocks.NewMockTaskService(ctrl)
	mockParser := mocks.NewMockCronParser(ctrl)
	handler := server.NewTaskHandler(mockSvc, mockParser)

	client, grpcSrv := setupGRPCServer(t, handler)
	defer grpcSrv.GracefulStop()

	now := time.Now().UTC()
	tasks := []entities.Task{
		{ID: uuid.New(), Name: "t1", NextRunAt: &now, CreatedAt: &now},
		{ID: uuid.New(), Name: "t2", NextRunAt: &now, CreatedAt: &now},
	}
	mockSvc.EXPECT().List(gomock.Any(), 5, 0).Return(tasks, nil)

	resp, err := client.ListTasks(context.Background(), &pb.ListTasksRequest{Limit: 5, Offset: 0})
	require.NoError(t, err)
	assert.Len(t, resp.Tasks, 2)
}

// ----------------- PollTask -----------------
func TestPollTaskIntegration_TaskReturned(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockSvc := mocks.NewMockTaskService(ctrl)
	mockParser := mocks.NewMockCronParser(ctrl)
	handler := server.NewTaskHandler(mockSvc, mockParser)

	client, grpcSrv := setupGRPCServer(t, handler)
	defer grpcSrv.GracefulStop()

	taskID := uuid.New()
	taskEntity := &entities.Task{
		ID:         taskID,
		QueueName:  "default",
		Payload:    []byte("{}"),
		MaxRetries: 2,
		RetryCount: 1,
	}

	mockSvc.EXPECT().
		Poll(gomock.Any(), "default", gomock.Any()).
		Return(taskEntity, nil)

	resp, err := client.PollTask(context.Background(), &pb.PollTaskRequest{
		QueueName: "default",
		WorkerId:  uuid.New().String(),
	})
	require.NoError(t, err)
	assert.Equal(t, taskID.String(), resp.Id)
	assert.Equal(t, int32(2), resp.Attempt)
	assert.Equal(t, int32(2), resp.MaxRetries)
}

func TestPollTaskIntegration_NoTask(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockSvc := mocks.NewMockTaskService(ctrl)
	mockParser := mocks.NewMockCronParser(ctrl)
	handler := server.NewTaskHandler(mockSvc, mockParser)

	client, grpcSrv := setupGRPCServer(t, handler)
	defer grpcSrv.GracefulStop()

	mockSvc.EXPECT().
		Poll(gomock.Any(), "empty", gomock.Any()).
		Return(nil, nil)

	resp, err := client.PollTask(context.Background(), &pb.PollTaskRequest{
		QueueName: "empty",
		WorkerId:  uuid.New().String(),
	})
	require.NoError(t, err)
	assert.Empty(t, resp.Id)
}

func TestPollTaskIntegration_DeadlineExceeded(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockSvc := mocks.NewMockTaskService(ctrl)
	mockParser := mocks.NewMockCronParser(ctrl)
	handler := server.NewTaskHandler(mockSvc, mockParser)

	client, grpcSrv := setupGRPCServer(t, handler)
	defer grpcSrv.GracefulStop()

	mockSvc.EXPECT().
		Poll(gomock.Any(), "default", gomock.Any()).
		Return(nil, context.DeadlineExceeded)

	resp, err := client.PollTask(context.Background(), &pb.PollTaskRequest{
		QueueName: "default",
		WorkerId:  uuid.New().String(),
	})
	require.NoError(t, err)
	assert.Empty(t, resp.Id)
}

// ----------------- CompleteTask -----------------
func TestCompleteTaskIntegration_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockSvc := mocks.NewMockTaskService(ctrl)
	mockParser := mocks.NewMockCronParser(ctrl)
	handler := server.NewTaskHandler(mockSvc, mockParser)

	client, grpcSrv := setupGRPCServer(t, handler)
	defer grpcSrv.GracefulStop()

	taskID := uuid.New()
	workerID := uuid.New()
	mockSvc.EXPECT().Complete(gomock.Any(), taskID.String(), workerID.String()).Return(nil)

	_, err := client.CompleteTask(context.Background(), &pb.CompleteTaskRequest{
		Id:       taskID.String(),
		WorkerId: workerID.String(),
	})
	require.NoError(t, err)
}

// ----------------- FailTask -----------------
func TestFailTaskIntegration_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockSvc := mocks.NewMockTaskService(ctrl)
	mockParser := mocks.NewMockCronParser(ctrl)
	handler := server.NewTaskHandler(mockSvc, mockParser)

	client, grpcSrv := setupGRPCServer(t, handler)
	defer grpcSrv.GracefulStop()

	taskID := uuid.New()
	workerID := uuid.New()
	mockSvc.EXPECT().Fail(gomock.Any(), taskID.String(), workerID.String(), "error").Return(nil)

	_, err := client.FailTask(context.Background(), &pb.FailTaskRequest{
		Id:           taskID.String(),
		WorkerId:     workerID.String(),
		ErrorMessage: "error",
	})
	require.NoError(t, err)
}

// ----------------- Heartbeat -----------------
func TestHeartbeatIntegration_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockSvc := mocks.NewMockTaskService(ctrl)
	mockParser := mocks.NewMockCronParser(ctrl)
	handler := server.NewTaskHandler(mockSvc, mockParser)

	client, grpcSrv := setupGRPCServer(t, handler)
	defer grpcSrv.GracefulStop()

	taskID := uuid.New()
	workerID := uuid.New()
	extend := 30 * time.Second
	mockSvc.EXPECT().Heartbeat(gomock.Any(), taskID.String(), workerID.String(), extend).Return(nil)

	_, err := client.Heartbeat(context.Background(), &pb.HeartbeatRequest{
		Id:            taskID.String(),
		WorkerId:      workerID.String(),
		ExtendSeconds: int32(extend.Seconds()),
	})
	require.NoError(t, err)
}
