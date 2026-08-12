package cron

import (
	"github.com/robfig/cron/v3"
	"go.uber.org/fx"
)

var Module = fx.Module("cron_parser",
	fx.Provide(New),
)

func New() *cron.Parser {
	return new(cron.NewParser(cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow | cron.Descriptor))
}
