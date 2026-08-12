package recoverer

import (
	"context"
)

//go:generate mockgen -source=$GOFILE -destination=mocks/mock_$GOFILE -package=mocks
type Repository interface {
	RecoverOrphanedTasks(ctx context.Context) error
}
