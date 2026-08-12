package main

import (
	"scheduler/internal/infrastructure/config"
	"scheduler/internal/infrastructure/cron"
	"scheduler/internal/infrastructure/daemon"
	"scheduler/internal/infrastructure/health"
	"scheduler/internal/infrastructure/logger"
	"scheduler/internal/infrastructure/server"
	"scheduler/internal/usecases/cleaner"
	"scheduler/internal/usecases/poller"
	"scheduler/internal/usecases/recoverer"
	"scheduler/internal/usecases/scheduler"
	"scheduler/internal/usecases/task"

	"scheduler/internal/infrastructure/repository/postgres"

	"go.uber.org/fx"
)

func main() {
	app := fx.New(
		// infrastructure
		config.Module,
		logger.Module,
		postgres.Module,
		cron.Module,
		server.Module,
		health.Module,

		// usecases
		task.Module,
		cleaner.Module,
		recoverer.Module,
		poller.Module,
		scheduler.Module,
		fx.Invoke(daemon.RunAllWorkers),
	)

	app.Run()
}
