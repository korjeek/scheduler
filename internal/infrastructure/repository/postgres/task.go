package postgres

import (
	"context"
	"errors"
	"fmt"
	"scheduler/internal/domain/entities"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

type TaskRepository struct {
	db *Database
}

func NewTaskRepository(db *Database) *TaskRepository {
	return &TaskRepository{db: db}
}

func (r *TaskRepository) Save(ctx context.Context, task *entities.Task) error {
	sql := `
        INSERT INTO tasks (
            id, name, queue_name, cron, payload, status, max_retries, 
            retry_count, last_error, worker_id, locked_until,
            next_run_at, last_run_at, created_at, updated_at
        ) VALUES (
            @id, @name, @queue_name, @cron, @payload, @status, @max_retries, 
            @retry_count, @last_error, @worker_id, @locked_until,
            @next_run_at, @last_run_at, @created_at, @updated_at
        );`

	args := pgx.NamedArgs{
		"id":           task.ID,
		"name":         task.Name,
		"queue_name":   task.QueueName,
		"cron":         task.Cron,
		"payload":      task.Payload,
		"status":       task.Status,
		"max_retries":  task.MaxRetries,
		"retry_count":  task.RetryCount,
		"last_error":   task.LastError,
		"worker_id":    task.WorkerID,
		"locked_until": task.LockedUntil,
		"next_run_at":  task.NextRunAt,
		"last_run_at":  task.LastRunAt,
		"created_at":   task.CreatedAt,
		"updated_at":   task.UpdatedAt,
	}

	if _, err := r.db.pool.Exec(ctx, sql, args); err != nil {
		return NewDbError("failed to insert task", "task.Save", err)
	}
	return nil
}

func (r *TaskRepository) FindByID(ctx context.Context, id uuid.UUID) (*entities.Task, error) {
	sql := `
        SELECT id, name, queue_name, cron, payload, status, max_retries, 
               retry_count, last_error, worker_id, locked_until,
               next_run_at, last_run_at, created_at, updated_at
        FROM tasks
        WHERE id = $1;`

	var t entities.Task
	err := r.db.pool.QueryRow(ctx, sql, id).Scan(
		&t.ID, &t.Name, &t.QueueName, &t.Cron, &t.Payload,
		&t.Status, &t.MaxRetries, &t.RetryCount, &t.LastError,
		&t.WorkerID, &t.LockedUntil,
		&t.NextRunAt, &t.LastRunAt, &t.CreatedAt, &t.UpdatedAt,
	)
	if err != nil {
		return nil, NewDbError("failed to find task", "task.FindByID", err)
	}
	return &t, nil
}

func (r *TaskRepository) FindAll(ctx context.Context, limit, offset int) ([]entities.Task, error) {
	sql := `
        SELECT id, name, queue_name, cron, payload, status, max_retries, 
               retry_count, last_error, worker_id, locked_until,
               next_run_at, last_run_at, created_at, updated_at
        FROM tasks
		ORDER BY created_at DESC
		LIMIT $1 OFFSET $2;`

	rows, err := r.db.pool.Query(ctx, sql, limit, offset)
	if err != nil {
		return nil, NewDbError("failed to execute select query", "task.FindAll", err)
	}
	defer rows.Close()

	tasks, err := pgx.CollectRows(rows, pgx.RowToStructByName[entities.Task])
	if err != nil {
		return nil, NewDbError("failed to collect tasks rows to structs", "task.FindAll", err)
	}

	if tasks == nil {
		return make([]entities.Task, 0), nil
	}
	return tasks, nil
}

func (r *TaskRepository) AcquireTask(ctx context.Context, queueName string, lockDuration time.Duration, workerID uuid.UUID) (*entities.Task, error) {
	tx, err := r.db.pool.Begin(ctx)
	if err != nil {
		return nil, NewDbError("failed to begin transaction", "task.AcquireTask", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	selectSQL := `
        SELECT id, name, queue_name, cron, payload, status, max_retries, 
               retry_count, last_error, worker_id, locked_until,
               next_run_at, last_run_at, created_at, updated_at
        FROM tasks
        WHERE queue_name = $1 AND status = 'pending' AND next_run_at <= NOW()
        ORDER BY next_run_at ASC
        LIMIT 1
        FOR UPDATE SKIP LOCKED`

	var task entities.Task
	err = tx.QueryRow(ctx, selectSQL, queueName).Scan(
		&task.ID, &task.Name, &task.QueueName, &task.Cron, &task.Payload,
		&task.Status, &task.MaxRetries, &task.RetryCount, &task.LastError,
		&task.WorkerID, &task.LockedUntil,
		&task.NextRunAt, &task.LastRunAt, &task.CreatedAt, &task.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, NewDbError("failed to select task for update", "task.AcquireTask", err)
	}

	updateSQL := `
        UPDATE tasks
        SET status = 'running',
            worker_id = $1,
            locked_until = NOW() + $2::INTERVAL,
            updated_at = NOW()
        WHERE id = $3 AND status = 'pending'`

	interval := fmt.Sprintf("%d seconds", int(lockDuration.Seconds()))
	cmdTag, err := tx.Exec(ctx, updateSQL, workerID, interval, task.ID)
	if err != nil {
		return nil, NewDbError("failed to update task", "task.AcquireTask", err)
	}
	if cmdTag.RowsAffected() == 0 {
		return nil, nil
	}

	if err = tx.Commit(ctx); err != nil {
		return nil, NewDbError("failed to commit transaction", "task.AcquireTask", err)
	}

	now := time.Now().UTC()
	task.Status = entities.StatusRunning
	task.WorkerID = &workerID
	task.LockedUntil = new(now.Add(lockDuration))

	return &task, nil
}

func (r *TaskRepository) CompleteTask(ctx context.Context, taskID uuid.UUID, workerID uuid.UUID, nextRunAt *time.Time) error {
	sql := `
        UPDATE tasks
        SET status = CASE
                WHEN $1::timestamp WITH TIME ZONE IS NOT NULL THEN 'pending'::task_status
                ELSE 'success'::task_status 
            END,
            next_run_at = $1,
            last_run_at = NOW(),
            retry_count = 0,
            worker_id = NULL,
            locked_until = NULL,
            last_error = NULL,
            updated_at = NOW()
        WHERE id = $2 
          AND status = 'running' 
          AND worker_id = $3`

	if _, err := r.db.pool.Exec(ctx, sql, nextRunAt, taskID, workerID.String()); err != nil {
		return NewDbError("failed to complete task", "task.CompleteTask", err)
	}

	return nil
}

func (r *TaskRepository) FailTask(ctx context.Context, taskID uuid.UUID, workerID uuid.UUID, errorMsg string) error {
	sql := `
        UPDATE tasks
        SET 
            status = CASE 
                WHEN retry_count + 1 < max_retries THEN 'pending'::task_status
                ELSE 'failed'::task_status
            END,
            next_run_at = CASE 
                WHEN retry_count + 1 < max_retries THEN NOW() + (pow(2, retry_count + 1) || ' seconds')::INTERVAL
                ELSE NULL
            END,
            retry_count = retry_count + 1,
            last_error = $1,
            worker_id = NULL,
            locked_until = NULL,
            updated_at = NOW()
        WHERE id = $2 
          AND status = 'running'::task_status
          AND worker_id = $3`

	if _, err := r.db.pool.Exec(ctx, sql, errorMsg, taskID, workerID.String()); err != nil {
		return NewDbError("failed to fail task", "task.FailTask", err)
	}
	return nil
}

func (r *TaskRepository) RecoverOrphanedTasks(ctx context.Context) error {
	sql := `
        UPDATE tasks
        SET status = 'pending',
            worker_id = NULL,
            locked_until = NULL,
            last_error = COALESCE(last_error, '') || ' (recovered after timeout)',
            updated_at = NOW()
        WHERE status = 'running' AND locked_until < NOW()`

	if _, err := r.db.pool.Exec(ctx, sql); err != nil {
		return NewDbError("failed to recover orphaned tasks", "task.RecoverOrphanedTasks", err)
	}
	return nil
}

func (r *TaskRepository) UpdateHeartbeat(ctx context.Context, taskID uuid.UUID, workerID uuid.UUID, duration time.Duration) error {
	sql := `
        UPDATE tasks
        SET locked_until = NOW() + $1::INTERVAL,
            updated_at = NOW()
        WHERE id = $2 AND status = 'running' AND worker_id = $3`

	interval := fmt.Sprintf("%d seconds", int(duration.Seconds()))
	if _, err := r.db.pool.Exec(ctx, sql, interval, taskID, workerID); err != nil {
		return NewDbError("failed to update heartbeat", "task.UpdateHeartbeat", err)
	}
	return nil
}

func (r *TaskRepository) DeleteCompletedTasks(ctx context.Context, olderThan time.Time, limit int) (int64, error) {
	sql := `
        WITH deleted AS (
            SELECT id FROM tasks
            WHERE status IN ('success'::task_status, 'failed'::task_status)
              AND updated_at < $1
            ORDER BY updated_at
            LIMIT $2
        )
        DELETE FROM tasks USING deleted WHERE tasks.id = deleted.id`

	tag, err := r.db.pool.Exec(ctx, sql, olderThan, limit)
	if err != nil {
		return 0, NewDbError("failed to delete old tasks", "task.DeleteCompletedTasks", err)
	}
	return tag.RowsAffected(), nil
}
