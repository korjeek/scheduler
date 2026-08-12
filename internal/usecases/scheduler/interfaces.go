package scheduler

import "github.com/robfig/cron/v3"

//go:generate mockgen -source=$GOFILE -destination=mocks/mock_$GOFILE -package=mocks
type CronParser interface {
	Parse(expr string) (cron.Schedule, error)
}
