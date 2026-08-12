package server

import (
	"context"
	"scheduler/internal/domain/entities"
	"scheduler/internal/usecases/task"
	"time"

	"github.com/robfig/cron/v3"
)

//go:generate mockgen -source=$GOFILE -destination=mocks/mock_$GOFILE -package=mocks
type TaskService interface {
	Create(ctx context.Context, dto task.CreateDto) (*entities.Task, error)
	Get(ctx context.Context, idStr string) (*entities.Task, error)
	List(ctx context.Context, limit, offset int) ([]entities.Task, error)
	Poll(ctx context.Context, queueName, workerIDStr string) (*entities.Task, error)
	Complete(ctx context.Context, taskIDStr, workerIDStr string) error
	Fail(ctx context.Context, taskIDStr, workerIDStr, errorMsg string) error
	Heartbeat(ctx context.Context, taskIDStr, workerIDStr string, extend time.Duration) error
}

type CronParser interface {
	Parse(expr string) (cron.Schedule, error)
}
