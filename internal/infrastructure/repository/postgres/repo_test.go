//go:build integration

package postgres

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"sync"
	"testing"
	"time"

	"scheduler/internal/domain/entities"
	"scheduler/internal/infrastructure/config"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
)

var (
	testDB   *Database
	testRepo *TaskRepository
	baseTime = time.Date(2026, 8, 11, 12, 0, 0, 0, time.UTC)
)

func TestMain(m *testing.M) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	dbUser := "postgres"
	dbPass := "password"
	dbName := "scheduler_test"

	pgContainer, err := postgres.Run(ctx,
		"postgres:16-alpine",
		postgres.WithDatabase(dbName),
		postgres.WithUsername(dbUser),
		postgres.WithPassword(dbPass),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2).
				WithStartupTimeout(30*time.Second),
		),
	)
	if err != nil {
		panic(fmt.Sprintf("failed to start postgres container: %v", err))
	}

	defer func() {
		if err := pgContainer.Terminate(context.Background()); err != nil {
			fmt.Printf("failed to terminate container: %v\n", err)
		}
	}()

	connStr, err := pgContainer.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		panic(fmt.Sprintf("failed to get connection string: %v", err))
	}

	cfg := config.Database{
		ConnString:        connStr,
		MaxCons:           10,
		MinCons:           2,
		MaxConnLifetime:   time.Hour,
		MaxConnIdleTime:   time.Minute * 30,
		HealthCheckPeriod: time.Second * 5,
		MigrationTimeout:  time.Second * 15,
	}

	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug}))

	testDB, err = NewDatabase(cfg, logger)
	if err != nil {
		panic(fmt.Sprintf("failed to init test database: %v", err))
	}

	migrateCtx, migrateCancel := context.WithTimeout(ctx, cfg.MigrationTimeout)
	if err := testDB.Migrate(migrateCtx); err != nil {
		migrateCancel()
		panic(fmt.Sprintf("failed to run migrations: %v", err))
	}
	migrateCancel()

	testRepo = NewTaskRepository(testDB)
	code := m.Run()

	if testDB != nil && testDB.pool != nil {
		testDB.pool.Close()
	}

	os.Exit(code)
}

func cleanTasksTable(t *testing.T, pool *pgxpool.Pool) {
	_, err := pool.Exec(context.Background(), "TRUNCATE TABLE tasks RESTART IDENTITY CASCADE;")
	require.NoError(t, err, "failed to truncate tasks table")
}

func TestTaskRepository_SaveAndFindByID(t *testing.T) {
	cleanTasksTable(t, testDB.pool)
	ctx := context.Background()

	taskID := uuid.New()
	task := &entities.Task{
		ID:         taskID,
		Name:       "test-task",
		QueueName:  "default",
		Cron:       "*/5 * * * *",
		Payload:    []byte(`{"key":"value"}`),
		Status:     entities.StatusPending,
		MaxRetries: 3,
		RetryCount: 0,
		NextRunAt:  timePtr(baseTime.Add(time.Minute)),
		CreatedAt:  &baseTime,
		UpdatedAt:  &baseTime,
	}

	err := testRepo.Save(ctx, task)
	require.NoError(t, err)

	found, err := testRepo.FindByID(ctx, taskID)
	require.NoError(t, err)
	assert.Equal(t, task.ID, found.ID)
	assert.Equal(t, task.Name, found.Name)
	assert.Equal(t, string(task.Payload), string(found.Payload))
}

func TestTaskRepository_FindByID_NotFound(t *testing.T) {
	cleanTasksTable(t, testDB.pool)
	ctx := context.Background()

	_, err := testRepo.FindByID(ctx, uuid.New())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to find task")
}

func TestTaskRepository_FindAll_Empty(t *testing.T) {
	cleanTasksTable(t, testDB.pool)
	ctx := context.Background()

	tasks, err := testRepo.FindAll(ctx, 10, 0)
	require.NoError(t, err)
	assert.Empty(t, tasks)
}

func TestTaskRepository_FindAll_Pagination(t *testing.T) {
	cleanTasksTable(t, testDB.pool)
	ctx := context.Background()

	for i := 0; i < 3; i++ {
		task := &entities.Task{
			ID: uuid.New(), Name: "t", QueueName: "q", Payload: []byte("{}"),
			Status: "pending", NextRunAt: &baseTime, CreatedAt: &baseTime, UpdatedAt: &baseTime,
		}
		require.NoError(t, testRepo.Save(ctx, task))
	}

	page1, err := testRepo.FindAll(ctx, 2, 0)
	require.NoError(t, err)
	assert.Len(t, page1, 2)

	page2, err := testRepo.FindAll(ctx, 2, 2)
	require.NoError(t, err)
	assert.Len(t, page2, 1)
}

func TestTaskRepository_CompleteTask_NotRunning(t *testing.T) {
	cleanTasksTable(t, testDB.pool)
	ctx := context.Background()

	taskID := uuid.New()
	workerID := uuid.New()

	task := &entities.Task{
		ID: taskID, Name: "already-done", QueueName: "q", Payload: []byte("{}"),
		Status: "success", NextRunAt: &baseTime, CreatedAt: &baseTime, UpdatedAt: &baseTime,
	}
	require.NoError(t, testRepo.Save(ctx, task))

	err := testRepo.CompleteTask(ctx, taskID, workerID, nil)
	require.NoError(t, err)

	updated, err := testRepo.FindByID(ctx, taskID)
	require.NoError(t, err)
	assert.Equal(t, entities.StatusSuccess, updated.Status)
}

func TestTaskRepository_FailTask_NoRetriesLeft(t *testing.T) {
	cleanTasksTable(t, testDB.pool)
	ctx := context.Background()

	taskID := uuid.New()
	workerID := uuid.New()

	task := &entities.Task{
		ID: taskID, Name: "fail-final", QueueName: "q", Payload: []byte("{}"),
		Status: "running", WorkerID: &workerID, MaxRetries: 1, RetryCount: 0,
		NextRunAt: &baseTime, CreatedAt: &baseTime, UpdatedAt: &baseTime,
	}
	require.NoError(t, testRepo.Save(ctx, task))

	err := testRepo.FailTask(ctx, taskID, workerID, "fatal error")
	require.NoError(t, err)

	updated, err := testRepo.FindByID(ctx, taskID)
	require.NoError(t, err)
	assert.Equal(t, entities.StatusFailed, updated.Status)
	assert.Nil(t, updated.NextRunAt)
	assert.Equal(t, 1, updated.RetryCount)
}

func TestTaskRepository_AcquireTask(t *testing.T) {
	cleanTasksTable(t, testDB.pool)
	ctx := context.Background()

	taskID := uuid.New()
	workerID := uuid.New()

	task := &entities.Task{
		ID:         taskID,
		Name:       "processable-task",
		QueueName:  "emails",
		Payload:    []byte("{}"),
		Status:     "pending",
		MaxRetries: 3,
		NextRunAt:  timePtr(baseTime.Add(-time.Minute)),
		CreatedAt:  &baseTime,
		UpdatedAt:  &baseTime,
	}
	require.NoError(t, testRepo.Save(ctx, task))

	acquired, err := testRepo.AcquireTask(ctx, "emails", 30*time.Second, workerID)
	require.NoError(t, err)
	require.NotNil(t, acquired)
	assert.Equal(t, taskID, acquired.ID)
	assert.Equal(t, entities.StatusRunning, acquired.Status)
	assert.Equal(t, workerID, *acquired.WorkerID)

	assert.True(t, acquired.LockedUntil.After(time.Now()), "locked_until должен быть в будущем")
	assert.WithinDuration(t, time.Now().Add(30*time.Second), *acquired.LockedUntil, 2*time.Second,
		"locked_until должен быть примерно через 30 секунд от текущего момента")

	anotherWorker := uuid.New()
	secondAcquired, err := testRepo.AcquireTask(ctx, "emails", 30*time.Second, anotherWorker)
	require.NoError(t, err)
	assert.Nil(t, secondAcquired)
}

func TestTaskRepository_AcquireTask_Concurrent(t *testing.T) {
	cleanTasksTable(t, testDB.pool)
	ctx := context.Background()

	taskID := uuid.New()
	task := &entities.Task{
		ID: taskID, Name: "concurrent", QueueName: "q", Payload: []byte("{}"),
		Status: "pending", NextRunAt: &baseTime, CreatedAt: &baseTime, UpdatedAt: &baseTime,
	}
	require.NoError(t, testRepo.Save(ctx, task))

	var wg sync.WaitGroup
	workers := []uuid.UUID{uuid.New(), uuid.New(), uuid.New()}
	acquired := make(chan *entities.Task, len(workers))

	for _, wid := range workers {
		wg.Add(1)
		go func(workerID uuid.UUID) {
			defer wg.Done()
			t, _ := testRepo.AcquireTask(ctx, "q", 30*time.Second, workerID)
			if t != nil {
				acquired <- t
			}
		}(wid)
	}
	wg.Wait()
	close(acquired)

	var count int
	for range acquired {
		count++
	}
	assert.Equal(t, 1, count, "only one goroutine should acquire the task")
}

func TestTaskRepository_CompleteTask_Cron(t *testing.T) {
	cleanTasksTable(t, testDB.pool)
	ctx := context.Background()

	taskID := uuid.New()
	workerID := uuid.New()

	task := &entities.Task{
		ID:          taskID,
		Name:        "cron-task",
		QueueName:   "default",
		Payload:     []byte("{}"),
		Status:      "running",
		WorkerID:    &workerID,
		LockedUntil: timePtr(time.Now().Add(time.Minute)),
		NextRunAt:   &baseTime,
		CreatedAt:   &baseTime,
		UpdatedAt:   &baseTime,
	}
	require.NoError(t, testRepo.Save(ctx, task))

	nextRun := baseTime.Add(time.Hour)
	err := testRepo.CompleteTask(ctx, taskID, workerID, &nextRun)
	require.NoError(t, err)

	updated, err := testRepo.FindByID(ctx, taskID)
	require.NoError(t, err)
	assert.Equal(t, entities.StatusPending, updated.Status)
	assert.Nil(t, updated.WorkerID)
	assert.Nil(t, updated.LockedUntil)
	assert.WithinDuration(t, nextRun, *updated.NextRunAt, time.Second)
}

func TestTaskRepository_FailTask_WithRetry(t *testing.T) {
	cleanTasksTable(t, testDB.pool)
	ctx := context.Background()

	taskID := uuid.New()
	workerID := uuid.New()

	task := &entities.Task{
		ID:         taskID,
		Name:       "failing-task",
		QueueName:  "default",
		Payload:    []byte("{}"),
		Status:     "running",
		WorkerID:   &workerID,
		MaxRetries: 3,
		RetryCount: 0,
		CreatedAt:  &baseTime,
		UpdatedAt:  &baseTime,
	}
	require.NoError(t, testRepo.Save(ctx, task))

	nowBefore := time.Now()
	err := testRepo.FailTask(ctx, taskID, workerID, "some critical error")
	require.NoError(t, err)

	updated, err := testRepo.FindByID(ctx, taskID)
	require.NoError(t, err)
	assert.Equal(t, entities.StatusPending, updated.Status)
	assert.Equal(t, 1, updated.RetryCount)
	assert.Equal(t, "some critical error", *updated.LastError)
	assert.NotNil(t, updated.NextRunAt)

	expectedMin := nowBefore.Add(2 * time.Second)
	expectedMax := nowBefore.Add(5 * time.Second)
	assert.True(t, updated.NextRunAt.After(expectedMin) || updated.NextRunAt.Equal(expectedMin),
		"next_run_at must be >= now + 2s")
	assert.True(t, updated.NextRunAt.Before(expectedMax),
		"next_run_at must be <= now + 5s")
}

func TestTaskRepository_UpdateHeartbeat(t *testing.T) {
	cleanTasksTable(t, testDB.pool)
	ctx := context.Background()

	taskID := uuid.New()
	workerID := uuid.New()

	task := &entities.Task{
		ID: taskID, Name: "hb", QueueName: "q", Payload: []byte("{}"),
		Status: "running", WorkerID: &workerID,
		LockedUntil: new(time.Now().Add(10 * time.Second)),
		NextRunAt:   &baseTime, CreatedAt: &baseTime, UpdatedAt: &baseTime,
	}
	require.NoError(t, testRepo.Save(ctx, task))

	require.NoError(t, testRepo.UpdateHeartbeat(ctx, taskID, workerID, 60*time.Second))

	updated, err := testRepo.FindByID(ctx, taskID)
	require.NoError(t, err)
	assert.WithinDuration(t, time.Now().Add(60*time.Second), *updated.LockedUntil, 2*time.Second,
		"locked_until должен обновиться до now + 60s")
}

func TestTaskRepository_RecoverOrphanedTasks(t *testing.T) {
	cleanTasksTable(t, testDB.pool)
	ctx := context.Background()

	taskID := uuid.New()
	task := &entities.Task{
		ID:          taskID,
		Name:        "stuck-task",
		QueueName:   "default",
		Payload:     []byte("{}"),
		Status:      "running",
		WorkerID:    uuiPtr(uuid.New()),
		LockedUntil: timePtr(time.Now().Add(-time.Minute)),
		CreatedAt:   &baseTime,
		UpdatedAt:   &baseTime,
	}
	require.NoError(t, testRepo.Save(ctx, task))

	err := testRepo.RecoverOrphanedTasks(ctx)
	require.NoError(t, err)

	updated, err := testRepo.FindByID(ctx, taskID)
	require.NoError(t, err)
	assert.Equal(t, entities.StatusPending, updated.Status)
	assert.Nil(t, updated.WorkerID)
	assert.Contains(t, *updated.LastError, "(recovered after timeout)")
}

func TestTaskRepository_RecoverOrphanedTasks_ValidLock(t *testing.T) {
	cleanTasksTable(t, testDB.pool)
	ctx := context.Background()

	taskID := uuid.New()
	task := &entities.Task{
		ID: taskID, Name: "still-alive", QueueName: "q", Payload: []byte("{}"),
		Status: "running", WorkerID: new(uuid.New()),
		LockedUntil: timePtr(time.Now().Add(5 * time.Minute)),
		NextRunAt:   &baseTime, CreatedAt: &baseTime, UpdatedAt: &baseTime,
	}
	require.NoError(t, testRepo.Save(ctx, task))

	err := testRepo.RecoverOrphanedTasks(ctx)
	require.NoError(t, err)

	updated, err := testRepo.FindByID(ctx, taskID)
	require.NoError(t, err)
	assert.Equal(t, entities.StatusRunning, updated.Status, "задача не должна быть восстановлена")
	assert.NotNil(t, updated.WorkerID)
}

func TestTaskRepository_DeleteCompletedTasks(t *testing.T) {
	cleanTasksTable(t, testDB.pool)
	ctx := context.Background()

	oldTask := &entities.Task{
		ID: uuid.New(), Name: "old", QueueName: "q", Payload: []byte("{}"),
		Status: "success", UpdatedAt: timePtr(time.Now().Add(-48 * time.Hour)),
		NextRunAt: &baseTime, CreatedAt: &baseTime,
	}
	require.NoError(t, testRepo.Save(ctx, oldTask))

	freshTask := &entities.Task{
		ID: uuid.New(), Name: "fresh", QueueName: "q", Payload: []byte("{}"),
		Status: "success", UpdatedAt: timePtr(time.Now().Add(-1 * time.Hour)),
		NextRunAt: &baseTime, CreatedAt: &baseTime,
	}
	require.NoError(t, testRepo.Save(ctx, freshTask))

	deleted, err := testRepo.DeleteCompletedTasks(ctx, time.Now().Add(-24*time.Hour), 100)
	require.NoError(t, err)
	assert.Equal(t, int64(1), deleted)

	_, err = testRepo.FindByID(ctx, oldTask.ID)
	assert.Error(t, err)

	_, err = testRepo.FindByID(ctx, freshTask.ID)
	assert.NoError(t, err)
}

func timePtr(t time.Time) *time.Time { return &t }
func uuiPtr(id uuid.UUID) *uuid.UUID { return &id }
