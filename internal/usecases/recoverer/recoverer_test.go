//go:build unit

package recoverer_test

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"scheduler/internal/infrastructure/config"
	"scheduler/internal/infrastructure/daemon" // Импортируем воркер
	"scheduler/internal/usecases/recoverer"
	"scheduler/internal/usecases/recoverer/mocks"
)

func setupLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug}))
}

func TestRecoverer_Run_CallsRecoverOrphanedTasks(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockRepo := mocks.NewMockRepository(ctrl)
	cfg := config.Recoverer{
		QueryTimeout: 500 * time.Millisecond,
	}

	mockRepo.EXPECT().RecoverOrphanedTasks(gomock.Any()).Return(nil).Times(1)

	r := recoverer.NewTaskRecoverer(mockRepo, cfg, setupLogger())

	err := r.Run(context.Background())
	require.NoError(t, err)
}

func TestRecoverer_ErrorReturned(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockRepo := mocks.NewMockRepository(ctrl)
	cfg := config.Recoverer{
		QueryTimeout: 200 * time.Millisecond,
	}

	testErr := errors.New("db connection lost")
	mockRepo.EXPECT().RecoverOrphanedTasks(gomock.Any()).Return(testErr).Times(1)

	r := recoverer.NewTaskRecoverer(mockRepo, cfg, setupLogger())

	err := r.Run(context.Background())
	require.ErrorIs(t, err, testErr)
}

func TestRecoverer_Shutdown_WithinTimeout(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockRepo := mocks.NewMockRepository(ctrl)
	mockRepo.EXPECT().RecoverOrphanedTasks(gomock.Any()).DoAndReturn(func(ctx context.Context) error {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(10 * time.Second):
			return nil
		}
	}).AnyTimes()

	cfg := config.Recoverer{
		Interval:        10 * time.Millisecond,
		QueryTimeout:    100 * time.Millisecond,
		ShutdownTimeout: 300 * time.Millisecond,
	}

	r := recoverer.NewTaskRecoverer(mockRepo, cfg, setupLogger())

	worker := daemon.NewWorker(r, setupLogger())

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		_ = worker.Start(context.Background())
	}()

	time.Sleep(20 * time.Millisecond)

	ctx, cancel := context.WithTimeout(context.Background(), cfg.ShutdownTimeout)
	defer cancel()

	start := time.Now()
	err := worker.Shutdown(ctx)
	elapsed := time.Since(start)

	wg.Wait()
	require.NoError(t, err, "Shutdown should complete without error")
	assert.Less(t, elapsed, cfg.ShutdownTimeout*2)
}
