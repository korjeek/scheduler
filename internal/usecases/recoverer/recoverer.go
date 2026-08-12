package recoverer

import (
	"context"
	"log/slog"
	"scheduler/internal/infrastructure/config"
	"scheduler/internal/infrastructure/daemon"
	"time"

	"go.uber.org/fx"
)

type TaskRecoverer struct {
	repo   Repository
	logger *slog.Logger
	cfg    config.Recoverer
}

var Module = fx.Module("recoverer",
	fx.Provide(NewTaskRecoverer),
	fx.Provide(
		fx.Annotate(
			func(r *TaskRecoverer) daemon.Task { return r },
			fx.ResultTags(`group:"daemon_tasks"`),
		),
	),
)

func NewTaskRecoverer(repo Repository, cfg config.Recoverer, logger *slog.Logger) *TaskRecoverer {
	return &TaskRecoverer{
		repo:   repo,
		cfg:    cfg,
		logger: logger.With("worker", "recoverer"),
	}
}

func (r *TaskRecoverer) Run(ctx context.Context) error {
	recoverCtx, cancel := context.WithTimeout(ctx, r.cfg.QueryTimeout)
	defer cancel()
	return r.repo.RecoverOrphanedTasks(recoverCtx)
}

func (r *TaskRecoverer) Interval() time.Duration {
	return r.cfg.Interval
}

func (r *TaskRecoverer) ShutdownTimeout() time.Duration {
	return r.cfg.ShutdownTimeout
}
