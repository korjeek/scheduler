//go:build unit

package cleaner_test

import (
	"context"
	"errors"
	"os"
	"sync"
	"testing"
	"time"

	"log/slog"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"scheduler/internal/infrastructure/config"
	"scheduler/internal/infrastructure/daemon"
	"scheduler/internal/usecases/cleaner"
	"scheduler/internal/usecases/cleaner/mocks"
)

func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug}))
}

func TestCleaner_SuccessfulBatchDelete(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockRepo := mocks.NewMockRepository(ctrl)
	cfg := config.Cleaner{
		Enabled:         true,
		Interval:        100 * time.Millisecond,
		RetentionPeriod: 720 * time.Hour,
		BatchSize:       2,
		BatchDelay:      1 * time.Millisecond,
		QueryTimeout:    5 * time.Second,
	}

	mockRepo.EXPECT().
		DeleteCompletedTasks(gomock.Any(), gomock.Any(), cfg.BatchSize).
		Return(int64(cfg.BatchSize), nil).
		Times(1)
	mockRepo.EXPECT().
		DeleteCompletedTasks(gomock.Any(), gomock.Any(), cfg.BatchSize).
		Return(int64(1), nil).
		Times(1)

	c := cleaner.NewTaskCleaner(mockRepo, cfg, testLogger())

	err := c.Run(context.Background())
	require.NoError(t, err)
}

func TestCleaner_DeleteErrorHandled(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockRepo := mocks.NewMockRepository(ctrl)
	cfg := config.Cleaner{
		Enabled:      true,
		BatchSize:    2,
		QueryTimeout: 5 * time.Second,
	}

	mockRepo.EXPECT().
		DeleteCompletedTasks(gomock.Any(), gomock.Any(), cfg.BatchSize).
		Return(int64(0), errors.New("db error")).
		Times(1)

	c := cleaner.NewTaskCleaner(mockRepo, cfg, testLogger())

	err := c.Run(context.Background())
	require.Error(t, err)
}

func TestCleaner_ShutdownDuringLongDelete(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockRepo := mocks.NewMockRepository(ctrl)
	cfg := config.Cleaner{
		Enabled:         true,
		Interval:        5 * time.Millisecond,
		RetentionPeriod: 720 * time.Hour,
		BatchSize:       1,
		BatchDelay:      0,
		ShutdownTimeout: 300 * time.Millisecond,
		QueryTimeout:    5 * time.Second,
	}

	mockRepo.EXPECT().
		DeleteCompletedTasks(gomock.Any(), gomock.Any(), cfg.BatchSize).
		DoAndReturn(func(ctx context.Context, _ time.Time, _ int) (int64, error) {
			<-ctx.Done()
			return 0, ctx.Err()
		}).
		AnyTimes()

	c := cleaner.NewTaskCleaner(mockRepo, cfg, testLogger())

	worker := daemon.NewWorker(c, testLogger())

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		_ = worker.Start(context.Background())
	}()

	time.Sleep(20 * time.Millisecond)

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), cfg.ShutdownTimeout)
	defer shutdownCancel()

	start := time.Now()
	err := worker.Shutdown(shutdownCtx)
	elapsed := time.Since(start)

	wg.Wait()
	require.NoError(t, err)
	assert.Less(t, elapsed, cfg.ShutdownTimeout+50*time.Millisecond)
}
