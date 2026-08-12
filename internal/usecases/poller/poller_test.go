package poller_test

import (
	"context"
	"database/sql"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"scheduler/internal/domain/entities"
	"scheduler/internal/infrastructure/config"
	"scheduler/internal/usecases/poller"
	"scheduler/internal/usecases/poller/mocks"
)

func TestDatabasePoller_AcquireImmediateSuccess(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockRepo := mocks.NewMockRepository(ctrl)
	cfg := config.Poller{
		PollInterval: 100 * time.Millisecond,
		LockDuration: 30 * time.Second,
	}

	expectedTask := &entities.Task{
		ID:     uuid.New(),
		Name:   "test-task",
		Status: entities.StatusPending,
	}

	mockRepo.EXPECT().
		AcquireTask(gomock.Any(), "default", cfg.LockDuration, gomock.Any()).
		Return(expectedTask, nil).
		Times(1)

	p := poller.NewDatabasePoller(mockRepo, cfg)
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	task, err := p.Poll(ctx, "default", uuid.New())
	require.NoError(t, err)
	assert.Equal(t, expectedTask.ID, task.ID)
}

func TestDatabasePoller_NoTasksThenSuccess(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockRepo := mocks.NewMockRepository(ctrl)
	cfg := config.Poller{
		PollInterval: 50 * time.Millisecond,
		LockDuration: 30 * time.Second,
	}

	expectedTask := &entities.Task{
		ID:     uuid.New(),
		Status: entities.StatusPending,
	}

	firstCall := mockRepo.EXPECT().
		AcquireTask(gomock.Any(), "default", cfg.LockDuration, gomock.Any()).
		Return(nil, sql.ErrNoRows)
	secondCall := mockRepo.EXPECT().
		AcquireTask(gomock.Any(), "default", cfg.LockDuration, gomock.Any()).
		Return(expectedTask, nil).
		After(firstCall)

	_ = secondCall

	p := poller.NewDatabasePoller(mockRepo, cfg)
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	start := time.Now()
	task, err := p.Poll(ctx, "default", uuid.New())
	elapsed := time.Since(start)

	require.NoError(t, err)
	assert.Equal(t, expectedTask.ID, task.ID)
	assert.True(t, elapsed >= cfg.PollInterval, "должен был подождать pollInterval")
}

func TestDatabasePoller_ContextTimeout(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockRepo := mocks.NewMockRepository(ctrl)
	cfg := config.Poller{
		PollInterval: 100 * time.Millisecond,
		LockDuration: 30 * time.Second,
	}

	mockRepo.EXPECT().
		AcquireTask(gomock.Any(), "default", cfg.LockDuration, gomock.Any()).
		Return(nil, sql.ErrNoRows).
		AnyTimes()

	p := poller.NewDatabasePoller(mockRepo, cfg)
	ctx, cancel := context.WithTimeout(context.Background(), 250*time.Millisecond)
	defer cancel()

	task, err := p.Poll(ctx, "default", uuid.New())
	assert.Nil(t, task)
	require.NoError(t, err)
}

func TestDatabasePoller_RepositoryError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockRepo := mocks.NewMockRepository(ctrl)
	cfg := config.Poller{
		PollInterval: 10 * time.Millisecond,
		LockDuration: 30 * time.Second,
	}

	dbErr := errors.New("connection refused")
	mockRepo.EXPECT().
		AcquireTask(gomock.Any(), "default", cfg.LockDuration, gomock.Any()).
		Return(nil, dbErr).
		Times(1)

	p := poller.NewDatabasePoller(mockRepo, cfg)
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	_, err := p.Poll(ctx, "default", uuid.New())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "acquire task")

	var pollerErr *poller.Error
	require.True(t, errors.As(err, &pollerErr), "должен быть тип PollerError")
}

func TestDatabasePoller_ContextCancelled(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockRepo := mocks.NewMockRepository(ctrl)
	cfg := config.Poller{
		PollInterval: 500 * time.Millisecond,
		LockDuration: 30 * time.Second,
	}

	p := poller.NewDatabasePoller(mockRepo, cfg)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	task, err := p.Poll(ctx, "default", uuid.New())
	assert.Nil(t, task)
	assert.ErrorIs(t, err, context.Canceled)
}
