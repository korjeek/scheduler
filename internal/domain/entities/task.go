package entities

import (
	"time"

	"github.com/google/uuid"
)

type TaskStatus string

const (
	StatusPending TaskStatus = "pending"
	StatusRunning TaskStatus = "running"
	StatusSuccess TaskStatus = "success"
	StatusFailed  TaskStatus = "failed"
)

type Task struct {
	// system ids
	ID        uuid.UUID  `db:"id"`
	Name      string     `db:"name"`
	QueueName string     `db:"queue_name"`
	CreatedAt *time.Time `db:"created_at"`
	UpdatedAt *time.Time `db:"updated_at"`

	// payload
	Payload []byte `db:"payload"`

	// planning logic
	Status    TaskStatus `db:"status"`
	Cron      string     `db:"cron"`
	NextRunAt *time.Time `db:"next_run_at"`
	LastRunAt *time.Time `db:"last_run_at"`

	// fault tolerance
	MaxRetries int     `db:"max_retries"`
	RetryCount int     `db:"retry_count"`
	LastError  *string `db:"last_error"`

	// distributed locking
	WorkerID    *uuid.UUID `db:"worker_id"`
	LockedUntil *time.Time `db:"locked_until"`
}

func NewTask(name, queueName, cron string, payload []byte, nextRunAt *time.Time, maxRetries int) *Task {
	id := uuid.New()
	now := time.Now().UTC()

	return &Task{
		ID:          id,
		Name:        name,
		QueueName:   queueName,
		CreatedAt:   &now,
		UpdatedAt:   &now,
		Payload:     payload,
		Status:      StatusPending,
		Cron:        cron,
		NextRunAt:   nextRunAt,
		MaxRetries:  maxRetries,
		RetryCount:  0,
		LastError:   nil,
		WorkerID:    nil,
		LockedUntil: nil,
	}
}
