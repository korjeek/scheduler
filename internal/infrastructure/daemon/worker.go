package daemon

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"go.uber.org/fx"
)

type Task interface {
	Run(ctx context.Context) error
	Interval() time.Duration
	ShutdownTimeout() time.Duration
}

type Worker struct {
	task   Task
	logger *slog.Logger

	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup

	interval        time.Duration
	shutdownTimeout time.Duration
}

func NewWorker(task Task, logger *slog.Logger) *Worker {
	ctx, cancel := context.WithCancel(context.Background())
	return &Worker{
		task:            task,
		logger:          logger,
		ctx:             ctx,
		cancel:          cancel,
		interval:        task.Interval(),
		shutdownTimeout: task.ShutdownTimeout(),
	}
}

func (w *Worker) Start(fxCtx context.Context) error {
	w.wg.Add(1)
	w.logger.DebugContext(fxCtx, "starting task")

	go func() {
		defer w.wg.Done()

		ticker := time.NewTicker(w.interval)
		defer ticker.Stop()

		w.logger.InfoContext(w.ctx, "worker loop started", "interval", w.interval)

		for {
			select {
			case <-w.ctx.Done():
				w.logger.InfoContext(w.ctx, "worker loop stopped")
				return
			case <-ticker.C:
				func() {
					defer func() {
						if r := recover(); r != nil {
							w.logger.ErrorContext(w.ctx, "recovered from panic in task", "panic", r)
						}
					}()
					if err := w.task.Run(w.ctx); err != nil {
						w.logger.ErrorContext(w.ctx, "task execution failed", "error", err)
					}
				}()
			}
		}
	}()

	return nil
}

func (w *Worker) Shutdown(fxCtx context.Context) error {
	w.logger.InfoContext(fxCtx, "shutting down worker...")
	w.cancel()

	shutdownCtx, cancel := context.WithTimeout(fxCtx, w.shutdownTimeout)
	defer cancel()

	done := make(chan struct{})
	go func() {
		w.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		w.logger.InfoContext(fxCtx, "worker stopped gracefully")
		return nil
	case <-shutdownCtx.Done():
		w.logger.ErrorContext(fxCtx, "worker shutdown timed out", "timeout", w.shutdownTimeout)
		return shutdownCtx.Err()
	}
}

func RunAllWorkers(lc fx.Lifecycle, logger *slog.Logger, params struct {
	fx.In
	Tasks []Task `group:"daemon_tasks"`
}) {
	for _, task := range params.Tasks {
		worker := NewWorker(task, logger)
		lc.Append(fx.Hook{
			OnStart: worker.Start,
			OnStop:  worker.Shutdown,
		})
	}
}
