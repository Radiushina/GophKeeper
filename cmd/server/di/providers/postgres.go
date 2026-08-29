package providers

import (
	"context"
	"fmt"

	"github.com/Radiushina/GophKeeper/config"
	"github.com/Radiushina/GophKeeper/internal/migrations"
	"github.com/Radiushina/GophKeeper/pgk/pgmigrator"
	"github.com/jackc/pgx/v5/pgxpool"
)

func NewPostgres(ctx context.Context, cfg *config.Config) (*pgxpool.Pool, func(), error) {
	if err := pgmigrator.MigrateFromEmbeddedFS(migrations.Postgres, "postgres", cfg.Database.DSN); err != nil {
		return nil, nil, fmt.Errorf("migrate: %w", err)
	}

	pool, err := pgxpool.New(ctx, cfg.Database.DSN)
	if err != nil {
		return nil, nil, fmt.Errorf("postgres: %w", err)
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, nil, fmt.Errorf("postgres ping: %w", err)
	}

	return pool, pool.Close, nil
}
