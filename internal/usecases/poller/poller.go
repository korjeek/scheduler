package poller

import (
	"context"
	"errors"
	"scheduler/internal/infrastructure/config"
	"scheduler/internal/usecases/task"
	"time"

	"scheduler/internal/domain/entities"

	"github.com/google/uuid"
	"go.uber.org/fx"
)

var Module = fx.Module("poller",
	fx.Provide(
		NewDatabasePoller,
		func(poller *DatabasePoller) task.Poller { return poller },
	),
)

type DatabasePoller struct {
	repo Repository
	cfg  config.Poller
}

func NewDatabasePoller(repo Repository, cfg config.Poller) *DatabasePoller {
	return &DatabasePoller{
		repo: repo,
		cfg:  cfg,
	}
}

func (p *DatabasePoller) Poll(ctx context.Context, queueName string, workerID uuid.UUID) (*entities.Task, error) {
	deadline, ok := ctx.Deadline()
	if !ok {
		deadline = time.Now().Add(30 * time.Second)
	}

	for {
		select {
		case <-ctx.Done():
			err := ctx.Err()
			if errors.Is(err, context.DeadlineExceeded) {
				return nil, nil
			}
			return nil, err
		default:
		}

		t, err := p.repo.AcquireTask(ctx, queueName, p.cfg.LockDuration, workerID)
		if err != nil {
			return nil, NewPollerError("acquire task", err)
		}
		if t != nil {
			return t, nil
		}

		sleepDuration := p.cfg.PollInterval
		if remaining := time.Until(deadline); remaining < sleepDuration {
			sleepDuration = remaining
		}
		if sleepDuration <= 0 {
			return nil, nil
		}

		timer := time.NewTimer(sleepDuration)
		select {
		case <-ctx.Done():
			timer.Stop()
			return nil, ctx.Err()
		case <-timer.C:
		}
	}
}
