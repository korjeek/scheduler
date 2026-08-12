package health

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"

	"github.com/hellofresh/health-go/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/fx"

	"scheduler/internal/infrastructure/config"
)

type Server struct {
	server *http.Server
	log    *slog.Logger
	cfg    config.Health
}

var Module = fx.Module("health",
	fx.Provide(
		func(pool *pgxpool.Pool, log *slog.Logger, cfg config.Health) (*Server, error) {
			return NewHealthServer(pool, log, cfg)
		},
	),
	fx.Invoke(func(hs *Server, lc fx.Lifecycle) {
		lc.Append(fx.Hook{
			OnStart: hs.Start,
			OnStop: func(ctx context.Context) error {
				shutdownCtx, cancel := context.WithTimeout(ctx, hs.cfg.ShutdownTimeout)
				defer cancel()
				return hs.Shutdown(shutdownCtx)
			},
		})
	}),
)

func NewHealthServer(pool *pgxpool.Pool, log *slog.Logger, cfg config.Health) (*Server, error) {
	h, err := health.New(health.WithComponent(health.Component{
		Name:    cfg.ComponentName,
		Version: cfg.ComponentVersion,
	}))
	if err != nil {
		return nil, fmt.Errorf("create health: %w", err)
	}

	err = h.Register(health.Config{
		Name:      "postgres",
		Timeout:   cfg.ReadTimeout,
		SkipOnErr: true,
		Check: func(ctx context.Context) error {
			return pool.Ping(ctx)
		},
	})
	if err != nil {
		return nil, fmt.Errorf("register postgres check: %w", err)
	}

	mux := http.NewServeMux()
	mux.Handle("/healthz", h.Handler())
	mux.Handle("/readyz", h.Handler())

	return &Server{
		server: &http.Server{
			Addr:         cfg.Address,
			Handler:      mux,
			ReadTimeout:  cfg.ReadTimeout,
			WriteTimeout: cfg.WriteTimeout,
		},
		log: log.With("component", "health"),
		cfg: cfg,
	}, nil
}

func (hs *Server) Start(ctx context.Context) error {
	hs.log.InfoContext(ctx, "health server starting", "addr", hs.server.Addr)
	go func() {
		if err := hs.server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			hs.log.ErrorContext(ctx, "health server failed", "error", err)
		}
	}()
	return nil
}

func (hs *Server) Shutdown(ctx context.Context) error {
	hs.log.InfoContext(ctx, "health server stopping")
	return hs.server.Shutdown(ctx)
}
