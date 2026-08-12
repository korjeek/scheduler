package task

import (
	"context"
	"scheduler/internal/domain/entities"
	"time"

	"github.com/google/uuid"
)

//go:generate mockgen -source=$GOFILE -destination=mocks/mock_$GOFILE -package=mocks
type Repository interface {
	Save(ctx context.Context, task *entities.Task) error
	FindByID(ctx context.Context, id uuid.UUID) (*entities.Task, error)
	FindAll(ctx context.Context, limit, offset int) ([]entities.Task, error)
	CompleteTask(ctx context.Context, taskID uuid.UUID, workerID uuid.UUID, nextRunAt *time.Time) error
	FailTask(ctx context.Context, taskID uuid.UUID, workerID uuid.UUID, errorMsg string) error
	UpdateHeartbeat(ctx context.Context, taskID uuid.UUID, workerID uuid.UUID, duration time.Duration) error
	RecoverOrphanedTasks(ctx context.Context) error
}

type Poller interface {
	Poll(ctx context.Context, queueName string, workerID uuid.UUID) (*entities.Task, error)
}

type Scheduler interface {
	ComputeNextRun(cronExpr string, runAt *time.Time) (*time.Time, error)
}
