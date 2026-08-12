package logger

import (
	"log/slog"
	"os"

	"go.uber.org/fx"
)

var Module = fx.Module("logger",
	fx.Provide(New),
)

func New() *slog.Logger {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))

	slog.SetDefault(logger)
	return logger
}
