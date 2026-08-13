//go:build unit

package entities_test

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"scheduler/internal/domain/entities"
)

func TestNewTask(t *testing.T) {
	nextRunAt := time.Now().UTC().Add(time.Hour)
	payload := []byte(`{"key":"value"}`)
	name := "test-task"
	queueName := "default"
	cron := "*/5 * * * *"
	maxRetries := 3

	task := entities.NewTask(name, queueName, cron, payload, &nextRunAt, maxRetries)

	require.NotNil(t, task)

	assert.NotEqual(t, uuid.Nil, task.ID)
	assert.Equal(t, name, task.Name)
	assert.Equal(t, queueName, task.QueueName)

	assert.NotNil(t, task.CreatedAt)
	assert.NotNil(t, task.UpdatedAt)
	assert.Equal(t, *task.CreatedAt, *task.UpdatedAt) // при создании одинаковы

	assert.Equal(t, payload, task.Payload)

	assert.Equal(t, entities.StatusPending, task.Status)
	assert.Equal(t, cron, task.Cron)
	require.NotNil(t, task.NextRunAt)
	assert.Equal(t, nextRunAt, *task.NextRunAt)

	assert.Equal(t, maxRetries, task.MaxRetries)
	assert.Equal(t, 0, task.RetryCount)
	assert.Nil(t, task.LastError)

	assert.Nil(t, task.WorkerID)
	assert.Nil(t, task.LockedUntil)
}
