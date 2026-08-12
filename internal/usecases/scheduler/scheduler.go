package scheduler

import (
	"time"

	"github.com/robfig/cron/v3"
	"go.uber.org/fx"
)

type TaskScheduler struct {
	cronParser CronParser
}

var Module = fx.Module("scheduler",
	fx.Provide(
		func(parser *cron.Parser) CronParser { return parser },
		NewTaskScheduler,
	),
)

func NewTaskScheduler(parser CronParser) *TaskScheduler {
	return &TaskScheduler{cronParser: parser}
}

func (s *TaskScheduler) ComputeNextRun(cronExpr string, runAt *time.Time) (*time.Time, error) {
	if cronExpr != "" {
		schedule, err := s.cronParser.Parse(cronExpr)
		if err != nil {
			return nil, err
		}
		return new(schedule.Next(time.Now().UTC())), nil
	}
	if runAt != nil {
		return new(runAt.UTC()), nil
	}
	return nil, nil
}
