package postgres

import (
	"context"
	"embed"
	"log/slog"
	"scheduler/internal/infrastructure/config"
	"scheduler/internal/usecases/cleaner"
	"scheduler/internal/usecases/poller"
	"scheduler/internal/usecases/recoverer"
	"scheduler/internal/usecases/task"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"
	"go.uber.org/fx"
)

//go:embed migrations/*.sql
var embedMigrations embed.FS

type Database struct {
	pool *pgxpool.Pool
	log  *slog.Logger
	cfg  config.Database
}

var Module = fx.Module("database",
	fx.Provide(
		NewDatabase,
		NewTaskRepository,
		func(repo *TaskRepository) task.Repository { return repo },
		func(repo *TaskRepository) poller.Repository { return repo },
		func(repo *TaskRepository) recoverer.Repository { return repo },
		func(repo *TaskRepository) cleaner.Repository { return repo },
		func(db *Database) *pgxpool.Pool { return db.pool },
	),
	fx.Invoke(func(db *Database, lc fx.Lifecycle, log *slog.Logger) {
		lc.Append(fx.Hook{
			OnStart: func(ctx context.Context) error {
				migrateCtx, cancel := context.WithTimeout(ctx, db.cfg.MigrationTimeout)
				defer cancel()
				return db.Migrate(migrateCtx)
			},
		})
	}),
)

func NewDatabase(cfg config.Database, log *slog.Logger) (*Database, error) {
	c, err := pgxpool.ParseConfig(cfg.ConnString)
	if err != nil {
		return nil, NewDbError("parse config", "database.NewDatabase", err)
	}
	c.MaxConns = cfg.MaxCons
	c.MinConns = cfg.MinCons
	c.MaxConnLifetime = cfg.MaxConnLifetime
	c.MaxConnIdleTime = cfg.MaxConnIdleTime
	c.HealthCheckPeriod = cfg.HealthCheckPeriod

	ctx, cancel := context.WithTimeout(context.Background(), cfg.ConnectTimeout)
	defer cancel()

	pool, err := pgxpool.NewWithConfig(ctx, c)
	if err != nil {
		return nil, NewDbError("create pool", "database.NewDatabase", err)
	}

	return &Database{
		pool: pool,
		log:  log.With("component", "database"),
		cfg:  cfg,
	}, nil
}

func (d *Database) Migrate(ctx context.Context) error {
	db := stdlib.OpenDB(*d.pool.Config().ConnConfig)
	defer func() {
		if err := db.Close(); err != nil {
			d.log.ErrorContext(ctx, "failed to close migration db", "error", err)
		}
	}()

	goose.SetBaseFS(embedMigrations)
	if err := goose.SetDialect("postgres"); err != nil {
		return NewDbError("failed to set the dialect to use for the goose package", "database.Migrate", err)
	}

	d.log.InfoContext(ctx, "running database migrations...")
	if err := goose.UpContext(ctx, db, "migrations"); err != nil {
		return NewDbError("failed to apply all available migrations", "database.Migrate", err)
	}

	d.log.InfoContext(ctx, "database migrations applied successfully")
	return nil
}
