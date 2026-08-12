package poller

import (
	"context"
	"scheduler/internal/domain/entities"
	"time"

	"github.com/google/uuid"
)

//go:generate mockgen -source=$GOFILE -destination=mocks/mock_$GOFILE -package=mocks
type Repository interface {
	AcquireTask(ctx context.Context, queueName string, lockDuration time.Duration, workerID uuid.UUID) (*entities.Task, error)
}
