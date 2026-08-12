package cleaner

import (
	"context"
	"time"
)

//go:generate mockgen -source=$GOFILE -destination=mocks/mock_$GOFILE -package=mocks
type Repository interface {
	DeleteCompletedTasks(ctx context.Context, olderThan time.Time, limit int) (int64, error)
}
