package cleaner

import (
	"context"
	"log/slog"
	"scheduler/internal/infrastructure/config"
	"scheduler/internal/infrastructure/daemon"
	"time"

	"go.uber.org/fx"
)

type TaskCleaner struct {
	repo   Repository
	cfg    config.Cleaner
	logger *slog.Logger
}

var Module = fx.Module("cleaner",
	fx.Provide(NewTaskCleaner),
	fx.Provide(
		fx.Annotate(
			func(c *TaskCleaner) daemon.Task { return c },
			fx.ResultTags(`group:"daemon_tasks"`),
		),
	),
)

func NewTaskCleaner(repo Repository, cfg config.Cleaner, logger *slog.Logger) *TaskCleaner {
	return &TaskCleaner{
		repo:   repo,
		cfg:    cfg,
		logger: logger.With("worker", "cleaner"),
	}
}

func (c *TaskCleaner) Run(ctx context.Context) error {
	cutoff := time.Now().Add(-c.cfg.RetentionPeriod)
	totalDeleted := int64(0)
	for {
		queryCtx, cancel := context.WithTimeout(ctx, c.cfg.QueryTimeout)
		n, err := c.repo.DeleteCompletedTasks(queryCtx, cutoff, c.cfg.BatchSize)
		cancel()

		if err != nil {
			return err
		}

		totalDeleted += n
		if n < int64(c.cfg.BatchSize) {
			break
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(c.cfg.BatchDelay):
		}
	}

	if totalDeleted > 0 {
		c.logger.InfoContext(ctx, "deleted old tasks", "count", totalDeleted)
	}

	return nil
}

func (c *TaskCleaner) Interval() time.Duration {
	return c.cfg.Interval
}

func (c *TaskCleaner) ShutdownTimeout() time.Duration {
	return c.cfg.ShutdownTimeout
}
