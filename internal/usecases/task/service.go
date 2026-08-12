package task

import (
	"context"
	"scheduler/internal/usecases/scheduler"
	"time"

	"scheduler/internal/domain/entities"

	"github.com/google/uuid"
	"go.uber.org/fx"
)

type CreateDto struct {
	Name       string
	QueueName  string
	MaxRetries int
	Payload    []byte
	Cron       string
	RunAt      *time.Time
}

type Service struct {
	repo   Repository
	ts     Scheduler
	poller Poller
}

var Module = fx.Module("task_service",
	fx.Provide(
		NewService,
		func(ts *scheduler.TaskScheduler) Scheduler { return ts },
	),
)

func NewService(r Repository, ts Scheduler, poller Poller) *Service {
	return &Service{
		repo:   r,
		ts:     ts,
		poller: poller,
	}
}

func (s *Service) Create(ctx context.Context, dto CreateDto) (*entities.Task, error) {
	nextRunAt, err := s.ts.ComputeNextRun(dto.Cron, dto.RunAt)
	if err != nil {
		return nil, NewServiceError("failed to compute next run time", "task.Create", err)
	}

	if nextRunAt == nil {
		nextRunAt = new(time.Now().UTC())
	}

	task := entities.NewTask(
		dto.Name,
		dto.QueueName,
		dto.Cron,
		dto.Payload,
		nextRunAt,
		dto.MaxRetries,
	)

	if err = s.repo.Save(ctx, task); err != nil {
		return nil, NewServiceError("failed to save task", "task.Create", err)
	}
	return task, nil
}

func (s *Service) Get(ctx context.Context, idStr string) (*entities.Task, error) {
	id, err := uuid.Parse(idStr)
	if err != nil {
		return nil, NewServiceError("failed to parse uuid", "task.Get", err)
	}

	task, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, NewServiceError("failed to get task", "task.Get", err)
	}
	return task, nil
}

func (s *Service) List(ctx context.Context, limit, offset int) ([]entities.Task, error) {
	tasks, err := s.repo.FindAll(ctx, limit, offset)
	if err != nil {
		return nil, NewServiceError("failed to get list of tasks", "task.List", err)
	}
	return tasks, nil
}

func (s *Service) Poll(ctx context.Context, queueName, workerIDStr string) (*entities.Task, error) {
	workerId, err := uuid.Parse(workerIDStr)
	if err != nil {
		return nil, NewServiceError("failed to parse worker uuid", "task.Poll", err)
	}

	return s.poller.Poll(ctx, queueName, workerId)
}

func (s *Service) Complete(ctx context.Context, taskIDStr, workerIDStr string) error {
	taskID, err := uuid.Parse(taskIDStr)
	if err != nil {
		return NewServiceError("failed to parse task uuid", "task.Complete", err)
	}

	workerId, err := uuid.Parse(workerIDStr)
	if err != nil {
		return NewServiceError("failed to parse worker uuid", "task.Complete", err)
	}

	task, err := s.repo.FindByID(ctx, taskID)
	if err != nil {
		return NewServiceError("failed to get task", "task.Complete", err)
	}

	nextRunAt, err := s.ts.ComputeNextRun(task.Cron, nil)
	if err != nil {
		return NewServiceError("failed to compute next run time", "task.Complete", err)
	}

	err = s.repo.CompleteTask(ctx, taskID, workerId, nextRunAt)
	if err != nil {
		return NewServiceError("failed to complete task", "task.Complete", err)
	}
	return nil
}

func (s *Service) Fail(ctx context.Context, taskIDStr, workerIDStr, errorMsg string) error {
	taskID, err := uuid.Parse(taskIDStr)
	if err != nil {
		return NewServiceError("failed to parse task uuid", "task.Fail", err)
	}

	workerId, err := uuid.Parse(workerIDStr)
	if err != nil {
		return NewServiceError("failed to parse worker uuid", "task.Fail", err)
	}

	if err = s.repo.FailTask(ctx, taskID, workerId, errorMsg); err != nil {
		return NewServiceError("failed to fail task", "task.Fail", err)
	}
	return nil
}

func (s *Service) Heartbeat(ctx context.Context, taskIDStr, workerIDStr string, extend time.Duration) error {
	taskID, err := uuid.Parse(taskIDStr)
	if err != nil {
		return NewServiceError("failed to parse task uuid", "task.Heartbeat", err)
	}

	workerId, err := uuid.Parse(workerIDStr)
	if err != nil {
		return NewServiceError("failed to parse worker uuid", "task.Heartbeat", err)
	}

	if err = s.repo.UpdateHeartbeat(ctx, taskID, workerId, extend); err != nil {
		return NewServiceError("failed to heartbeat task", "task.Heartbeat", err)
	}
	return nil
}

func (s *Service) RecoverOrphanedTasks(ctx context.Context) error {
	return s.repo.RecoverOrphanedTasks(ctx)
}
