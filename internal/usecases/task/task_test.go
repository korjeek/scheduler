//go:build unit

package task_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"scheduler/internal/domain/entities"
	"scheduler/internal/usecases/task"
	"scheduler/internal/usecases/task/mocks"
)

// ---------------------- Create ----------------------
func TestService_Create_SuccessWithCron(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockRepo := mocks.NewMockRepository(ctrl)
	mockScheduler := mocks.NewMockScheduler(ctrl)
	mockPoller := mocks.NewMockPoller(ctrl)

	svc := task.NewService(mockRepo, mockScheduler, mockPoller)

	nextRun := time.Date(2026, 8, 11, 12, 0, 0, 0, time.UTC)
	dto := task.CreateDto{
		Name:       "cron-task",
		QueueName:  "default",
		MaxRetries: 2,
		Payload:    []byte("{}"),
		Cron:       "@every 1h",
	}

	mockScheduler.EXPECT().
		ComputeNextRun(dto.Cron, nil).
		Return(&nextRun, nil).
		Times(1)

	mockRepo.EXPECT().
		Save(gomock.Any(), gomock.AssignableToTypeOf(&entities.Task{})).
		Return(nil).
		Times(1)

	res, err := svc.Create(context.Background(), dto)
	require.NoError(t, err)
	require.NotNil(t, res)
	assert.Equal(t, dto.Name, res.Name)
	assert.Equal(t, dto.QueueName, res.QueueName)
	assert.Equal(t, dto.MaxRetries, res.MaxRetries)
	assert.WithinDuration(t, nextRun, *res.NextRunAt, time.Second)
	assert.Equal(t, entities.StatusPending, res.Status)
}

func TestService_Create_SuccessWithoutCronAndRunAt(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockRepo := mocks.NewMockRepository(ctrl)
	mockScheduler := mocks.NewMockScheduler(ctrl)
	mockPoller := mocks.NewMockPoller(ctrl)

	svc := task.NewService(mockRepo, mockScheduler, mockPoller)

	dto := task.CreateDto{
		Name:       "immediate",
		QueueName:  "default",
		MaxRetries: 0,
		Payload:    []byte("{}"),
	}

	mockScheduler.EXPECT().
		ComputeNextRun("", nil).
		Return(nil, nil).
		Times(1)

	mockRepo.EXPECT().
		Save(gomock.Any(), gomock.AssignableToTypeOf(&entities.Task{})).
		Return(nil).
		Times(1)

	res, err := svc.Create(context.Background(), dto)
	require.NoError(t, err)
	require.NotNil(t, res)
	assert.NotNil(t, res.NextRunAt)
	assert.False(t, res.NextRunAt.IsZero(), "next run time should be now")
}

func TestService_Create_SchedulerError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockRepo := mocks.NewMockRepository(ctrl)
	mockScheduler := mocks.NewMockScheduler(ctrl)
	mockPoller := mocks.NewMockPoller(ctrl)

	svc := task.NewService(mockRepo, mockScheduler, mockPoller)

	dto := task.CreateDto{
		Name:      "fail",
		QueueName: "default",
		Cron:      "invalid",
	}

	mockScheduler.EXPECT().
		ComputeNextRun(dto.Cron, nil).
		Return(nil, errors.New("bad cron")).
		Times(1)

	_, err := svc.Create(context.Background(), dto)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to compute next run time")
}

func TestService_Create_RepositorySaveError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockRepo := mocks.NewMockRepository(ctrl)
	mockScheduler := mocks.NewMockScheduler(ctrl)
	mockPoller := mocks.NewMockPoller(ctrl)

	svc := task.NewService(mockRepo, mockScheduler, mockPoller)

	dto := task.CreateDto{
		Name:      "fail-save",
		QueueName: "default",
	}

	mockScheduler.EXPECT().
		ComputeNextRun(gomock.Any(), nil).
		Return(nil, nil)

	mockRepo.EXPECT().
		Save(gomock.Any(), gomock.Any()).
		Return(errors.New("db error")).
		Times(1)

	_, err := svc.Create(context.Background(), dto)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to save task")
}

// ---------------------- Get ----------------------
func TestService_Get_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockRepo := mocks.NewMockRepository(ctrl)
	mockScheduler := mocks.NewMockScheduler(ctrl)
	mockPoller := mocks.NewMockPoller(ctrl)

	svc := task.NewService(mockRepo, mockScheduler, mockPoller)

	taskID := uuid.New()
	expected := &entities.Task{ID: taskID, Name: "test"}

	mockRepo.EXPECT().
		FindByID(gomock.Any(), taskID).
		Return(expected, nil).
		Times(1)

	res, err := svc.Get(context.Background(), taskID.String())
	require.NoError(t, err)
	assert.Equal(t, expected.ID, res.ID)
}

func TestService_Get_InvalidUUID(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockRepo := mocks.NewMockRepository(ctrl)
	mockScheduler := mocks.NewMockScheduler(ctrl)
	mockPoller := mocks.NewMockPoller(ctrl)

	svc := task.NewService(mockRepo, mockScheduler, mockPoller)

	_, err := svc.Get(context.Background(), "not-a-uuid")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to parse uuid")
}

func TestService_Get_NotFound(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockRepo := mocks.NewMockRepository(ctrl)
	mockScheduler := mocks.NewMockScheduler(ctrl)
	mockPoller := mocks.NewMockPoller(ctrl)

	svc := task.NewService(mockRepo, mockScheduler, mockPoller)

	taskID := uuid.New()
	mockRepo.EXPECT().
		FindByID(gomock.Any(), taskID).
		Return(nil, errors.New("not found")).
		Times(1)

	_, err := svc.Get(context.Background(), taskID.String())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to get task")
}

// ---------------------- List ----------------------
func TestService_List_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockRepo := mocks.NewMockRepository(ctrl)
	mockScheduler := mocks.NewMockScheduler(ctrl)
	mockPoller := mocks.NewMockPoller(ctrl)

	svc := task.NewService(mockRepo, mockScheduler, mockPoller)

	expected := []entities.Task{{Name: "t1"}, {Name: "t2"}}
	mockRepo.EXPECT().
		FindAll(gomock.Any(), 10, 0).
		Return(expected, nil).
		Times(1)

	res, err := svc.List(context.Background(), 10, 0)
	require.NoError(t, err)
	assert.Len(t, res, 2)
}

func TestService_List_Error(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockRepo := mocks.NewMockRepository(ctrl)
	mockScheduler := mocks.NewMockScheduler(ctrl)
	mockPoller := mocks.NewMockPoller(ctrl)

	svc := task.NewService(mockRepo, mockScheduler, mockPoller)

	mockRepo.EXPECT().
		FindAll(gomock.Any(), 5, 5).
		Return(nil, errors.New("db error")).
		Times(1)

	_, err := svc.List(context.Background(), 5, 5)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to get list of tasks")
}

// ---------------------- Poll ----------------------
func TestService_Poll_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockRepo := mocks.NewMockRepository(ctrl)
	mockScheduler := mocks.NewMockScheduler(ctrl)
	mockPoller := mocks.NewMockPoller(ctrl)

	svc := task.NewService(mockRepo, mockScheduler, mockPoller)

	workerID := uuid.New()
	expected := &entities.Task{ID: uuid.New(), Status: entities.StatusPending}

	mockPoller.EXPECT().
		Poll(gomock.Any(), "default", workerID).
		Return(expected, nil).
		Times(1)

	res, err := svc.Poll(context.Background(), "default", workerID.String())
	require.NoError(t, err)
	assert.Equal(t, expected.ID, res.ID)
}

func TestService_Poll_InvalidWorkerID(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockRepo := mocks.NewMockRepository(ctrl)
	mockScheduler := mocks.NewMockScheduler(ctrl)
	mockPoller := mocks.NewMockPoller(ctrl)

	svc := task.NewService(mockRepo, mockScheduler, mockPoller)

	_, err := svc.Poll(context.Background(), "default", "bad-uuid")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to parse worker uuid")
}

// ---------------------- Complete ----------------------
func TestService_Complete_CronTask(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockRepo := mocks.NewMockRepository(ctrl)
	mockScheduler := mocks.NewMockScheduler(ctrl)
	mockPoller := mocks.NewMockPoller(ctrl)

	svc := task.NewService(mockRepo, mockScheduler, mockPoller)

	taskID := uuid.New()
	workerID := uuid.New()
	cronExpr := "@every 1h"

	taskEntity := &entities.Task{
		ID:   taskID,
		Cron: cronExpr,
	}

	nextRun := time.Now().Add(1 * time.Hour)

	gomock.InOrder(
		mockRepo.EXPECT().
			FindByID(gomock.Any(), taskID).
			Return(taskEntity, nil),
		mockScheduler.EXPECT().
			ComputeNextRun(cronExpr, nil).
			Return(&nextRun, nil),
		mockRepo.EXPECT().
			CompleteTask(gomock.Any(), taskID, workerID, &nextRun).
			Return(nil),
	)

	err := svc.Complete(context.Background(), taskID.String(), workerID.String())
	require.NoError(t, err)
}

func TestService_Complete_OneOffTask(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockRepo := mocks.NewMockRepository(ctrl)
	mockScheduler := mocks.NewMockScheduler(ctrl)
	mockPoller := mocks.NewMockPoller(ctrl)

	svc := task.NewService(mockRepo, mockScheduler, mockPoller)

	taskID := uuid.New()
	workerID := uuid.New()

	taskEntity := &entities.Task{
		ID:   taskID,
		Cron: "",
	}

	gomock.InOrder(
		mockRepo.EXPECT().
			FindByID(gomock.Any(), taskID).
			Return(taskEntity, nil),
		mockScheduler.EXPECT().
			ComputeNextRun("", nil).
			Return(nil, nil),
		mockRepo.EXPECT().
			CompleteTask(gomock.Any(), taskID, workerID, nil).
			Return(nil),
	)

	err := svc.Complete(context.Background(), taskID.String(), workerID.String())
	require.NoError(t, err)
}

func TestService_Complete_TaskNotFound(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockRepo := mocks.NewMockRepository(ctrl)
	mockScheduler := mocks.NewMockScheduler(ctrl)
	mockPoller := mocks.NewMockPoller(ctrl)

	svc := task.NewService(mockRepo, mockScheduler, mockPoller)

	taskID := uuid.New()
	workerID := uuid.New()

	mockRepo.EXPECT().
		FindByID(gomock.Any(), taskID).
		Return(nil, errors.New("not found"))

	err := svc.Complete(context.Background(), taskID.String(), workerID.String())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to get task")
}

func TestService_Complete_SchedulerError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockRepo := mocks.NewMockRepository(ctrl)
	mockScheduler := mocks.NewMockScheduler(ctrl)
	mockPoller := mocks.NewMockPoller(ctrl)

	svc := task.NewService(mockRepo, mockScheduler, mockPoller)

	taskID := uuid.New()
	workerID := uuid.New()
	taskEntity := &entities.Task{ID: taskID, Cron: "bad"}

	mockRepo.EXPECT().FindByID(gomock.Any(), taskID).Return(taskEntity, nil)
	mockScheduler.EXPECT().ComputeNextRun("bad", nil).Return(nil, errors.New("bad cron"))

	err := svc.Complete(context.Background(), taskID.String(), workerID.String())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to compute next run time")
}

// ---------------------- Fail ----------------------
func TestService_Fail_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockRepo := mocks.NewMockRepository(ctrl)
	mockScheduler := mocks.NewMockScheduler(ctrl)
	mockPoller := mocks.NewMockPoller(ctrl)

	svc := task.NewService(mockRepo, mockScheduler, mockPoller)

	taskID := uuid.New()
	workerID := uuid.New()
	errMsg := "something went wrong"

	mockRepo.EXPECT().
		FailTask(gomock.Any(), taskID, workerID, errMsg).
		Return(nil).
		Times(1)

	err := svc.Fail(context.Background(), taskID.String(), workerID.String(), errMsg)
	require.NoError(t, err)
}

func TestService_Fail_InvalidIDs(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockRepo := mocks.NewMockRepository(ctrl)
	mockScheduler := mocks.NewMockScheduler(ctrl)
	mockPoller := mocks.NewMockPoller(ctrl)

	svc := task.NewService(mockRepo, mockScheduler, mockPoller)

	err := svc.Fail(context.Background(), "bad", "also-bad", "msg")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to parse task uuid")
}

func TestService_Fail_RepositoryError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockRepo := mocks.NewMockRepository(ctrl)
	mockScheduler := mocks.NewMockScheduler(ctrl)
	mockPoller := mocks.NewMockPoller(ctrl)

	svc := task.NewService(mockRepo, mockScheduler, mockPoller)

	taskID := uuid.New()
	workerID := uuid.New()
	mockRepo.EXPECT().FailTask(gomock.Any(), taskID, workerID, gomock.Any()).
		Return(errors.New("db error"))

	err := svc.Fail(context.Background(), taskID.String(), workerID.String(), "msg")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to fail task")
}

// ---------------------- Heartbeat ----------------------
func TestService_Heartbeat_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockRepo := mocks.NewMockRepository(ctrl)
	mockScheduler := mocks.NewMockScheduler(ctrl)
	mockPoller := mocks.NewMockPoller(ctrl)

	svc := task.NewService(mockRepo, mockScheduler, mockPoller)

	taskID := uuid.New()
	workerID := uuid.New()
	extend := 30 * time.Second

	mockRepo.EXPECT().
		UpdateHeartbeat(gomock.Any(), taskID, workerID, extend).
		Return(nil).
		Times(1)

	err := svc.Heartbeat(context.Background(), taskID.String(), workerID.String(), extend)
	require.NoError(t, err)
}

func TestService_Heartbeat_InvalidIDs(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockRepo := mocks.NewMockRepository(ctrl)
	mockScheduler := mocks.NewMockScheduler(ctrl)
	mockPoller := mocks.NewMockPoller(ctrl)

	svc := task.NewService(mockRepo, mockScheduler, mockPoller)

	err := svc.Heartbeat(context.Background(), "bad", "also-bad", 5*time.Second)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to parse task uuid")
}

func TestService_Heartbeat_RepositoryError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockRepo := mocks.NewMockRepository(ctrl)
	mockScheduler := mocks.NewMockScheduler(ctrl)
	mockPoller := mocks.NewMockPoller(ctrl)

	svc := task.NewService(mockRepo, mockScheduler, mockPoller)

	taskID := uuid.New()
	workerID := uuid.New()
	mockRepo.EXPECT().
		UpdateHeartbeat(gomock.Any(), taskID, workerID, gomock.Any()).
		Return(errors.New("db error"))

	err := svc.Heartbeat(context.Background(), taskID.String(), workerID.String(), 10*time.Second)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to heartbeat task")
}
