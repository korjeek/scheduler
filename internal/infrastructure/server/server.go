package server

import (
	"context"
	"errors"
	"log/slog"
	"net"
	"net/http"
	"scheduler/internal/infrastructure/config"
	"scheduler/internal/infrastructure/server/middleware"
	"scheduler/internal/usecases/task"
	"scheduler/pkg/pb"

	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	"github.com/robfig/cron/v3"
	"go.uber.org/fx"
	"golang.org/x/time/rate"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
)

type Server struct {
	grpcServer *grpc.Server
	httpServer *http.Server
	gwMux      *runtime.ServeMux
	handler    *TaskHandler
	log        *slog.Logger
	cfg        config.Server
}

var Module = fx.Module("servers",
	fx.Provide(
		NewTaskHandler,
		NewServerManager,
		func(svc *task.Service) TaskService { return svc },
		func(p *cron.Parser) CronParser { return p },
	),
	fx.Invoke(func(lc fx.Lifecycle, s *Server) {
		lc.Append(fx.Hook{
			OnStart: s.Start,
			OnStop: func(ctx context.Context) error {
				shutdownCtx, cancel := context.WithTimeout(ctx, s.cfg.ShutdownTimeout)
				defer cancel()
				return s.Shutdown(shutdownCtx)
			},
		})
	}),
)

func NewServerManager(handler *TaskHandler, cfg config.Server, log *slog.Logger) *Server {
	limiter := rate.NewLimiter(rate.Limit(cfg.RateLimiter.RequestsPerSecond), cfg.RateLimiter.Burst)
	grpcServer := grpc.NewServer(
		grpc.ChainUnaryInterceptor(
			middleware.UnaryRateLimitInterceptor(limiter),
			middleware.UnaryRecoveryInterceptor(log),
		),
	)
	pb.RegisterTaskServiceServer(grpcServer, handler)

	gwMux := runtime.NewServeMux()
	mainMux := http.NewServeMux()
	mainMux.Handle("/", gwMux)

	if cfg.DevMode {
		reflection.Register(grpcServer)
		registerSwagger(mainMux, log)
	}

	recoveredHandler := middleware.HTTPRecoveryMiddleware(log, mainMux)

	srv := &http.Server{
		Addr:              cfg.HttpAddr,
		Handler:           recoveredHandler,
		ReadTimeout:       cfg.ReadTimeout,
		ReadHeaderTimeout: cfg.ReadHeaderTimeout,
		WriteTimeout:      cfg.WriteTimeout,
		IdleTimeout:       cfg.IdleTimeout,
		MaxHeaderBytes:    cfg.MaxHeaderBytes,
	}

	return &Server{
		grpcServer: grpcServer,
		httpServer: srv,
		gwMux:      gwMux,
		handler:    handler,
		log:        log.With("component", "server"),
		cfg:        cfg,
	}
}

func (s *Server) Start(ctx context.Context) error {
	lis, err := net.Listen("tcp", s.cfg.GrpcAddr)
	if err != nil {
		return NewSrvError("failed to listen tcp port", err)
	}

	s.log.InfoContext(ctx, "gRPC backend server starting", "addr", s.cfg.GrpcAddr)
	go func() {
		if err = s.grpcServer.Serve(lis); err != nil {
			s.log.ErrorContext(ctx, "failed to start gRPC server", "error", err)
		}
	}()

	if err = pb.RegisterTaskServiceHandlerServer(ctx, s.gwMux, s.handler); err != nil {
		return NewSrvError("failed to register grpc-gateway endpoint", err)
	}

	s.log.InfoContext(ctx, "http gateway API entrypoint server starting", "addr", s.cfg.HttpAddr)
	go func() {
		if err = s.httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			s.log.ErrorContext(ctx, "http server failed to listen", "error", err)
		}
	}()

	return nil
}

func (s *Server) Shutdown(ctx context.Context) error {
	s.log.WarnContext(ctx, "stopping network services...")
	s.grpcServer.GracefulStop()

	if err := s.httpServer.Shutdown(ctx); err != nil {
		return NewSrvError("failed to shutdown http server", err)
	}

	return nil
}
