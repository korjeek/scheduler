//go:build unit

package daemon_test

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"scheduler/internal/infrastructure/daemon"
)

type fakeTask struct {
	interval        time.Duration
	shutdownTimeout time.Duration
	runFunc         func(ctx context.Context) error
	runCalls        atomic.Int32
}

func (f *fakeTask) Run(ctx context.Context) error {
	f.runCalls.Add(1)
	if f.runFunc != nil {
		return f.runFunc(ctx)
	}
	return nil
}

func (f *fakeTask) Interval() time.Duration        { return f.interval }
func (f *fakeTask) ShutdownTimeout() time.Duration { return f.shutdownTimeout }

func newTestLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func TestWorker_RunCalledPeriodically(t *testing.T) {
	task := &fakeTask{
		interval:        10 * time.Millisecond,
		shutdownTimeout: 100 * time.Millisecond,
	}

	worker := daemon.NewWorker(task, newTestLogger())

	ctx := context.Background()
	require.NoError(t, worker.Start(ctx))

	require.Eventually(t, func() bool {
		return task.runCalls.Load() >= 2
	}, 500*time.Millisecond, 10*time.Millisecond)

	require.NoError(t, worker.Shutdown(ctx))
}

func TestWorker_PanicInTaskIsRecovered(t *testing.T) {
	task := &fakeTask{
		interval:        5 * time.Millisecond,
		shutdownTimeout: 100 * time.Millisecond,
		runFunc: func(ctx context.Context) error {
			panic("unexpected panic")
		},
	}

	worker := daemon.NewWorker(task, newTestLogger())
	ctx := context.Background()

	require.NoError(t, worker.Start(ctx))

	time.Sleep(30 * time.Millisecond)

	require.NoError(t, worker.Shutdown(ctx))

	assert.True(t, task.runCalls.Load() > 0, "Run should have been called at least once")
}

func TestWorker_ShutdownGracefulWithinTimeout(t *testing.T) {
	task := &fakeTask{
		interval:        10 * time.Millisecond,
		shutdownTimeout: 50 * time.Millisecond,
		runFunc: func(ctx context.Context) error {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(1 * time.Second):
				return nil
			}
		},
	}

	worker := daemon.NewWorker(task, newTestLogger())
	ctx := context.Background()

	require.NoError(t, worker.Start(ctx))
	time.Sleep(20 * time.Millisecond)

	start := time.Now()
	err := worker.Shutdown(ctx)
	elapsed := time.Since(start)

	require.NoError(t, err, "Shutdown should complete without error")
	assert.Less(t, elapsed, 200*time.Millisecond, "Shutdown took too long")
}

func TestWorker_ShutdownTimeout(t *testing.T) {
	task := &fakeTask{
		interval:        10 * time.Millisecond,
		shutdownTimeout: 30 * time.Millisecond,
		runFunc: func(ctx context.Context) error {
			select {
			case <-time.After(1 * time.Second):
				return nil
			}
		},
	}

	worker := daemon.NewWorker(task, newTestLogger())
	ctx := context.Background()

	require.NoError(t, worker.Start(ctx))
	time.Sleep(20 * time.Millisecond)

	start := time.Now()
	err := worker.Shutdown(ctx)
	elapsed := time.Since(start)

	require.Error(t, err, "Shutdown should time out")
	assert.True(t, errors.Is(err, context.DeadlineExceeded), "expected DeadlineExceeded")
	assert.GreaterOrEqual(t, elapsed, 30*time.Millisecond)
}
