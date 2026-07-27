package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

func openDB(ctx context.Context, databaseURL string) (*pgxpool.Pool, error) {
	cfg, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		return nil, fmt.Errorf("parse database url: %w", err)
	}
	cfg.MaxConns = 10
	cfg.MinConns = 1
	cfg.MaxConnLifetime = time.Hour
	cfg.HealthCheckPeriod = 30 * time.Second

	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("connect: %w", err)
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("ping: %w", err)
	}
	if err := migrate(ctx, pool); err != nil {
		pool.Close()
		return nil, fmt.Errorf("migrate: %w", err)
	}
	return pool, nil
}

func migrate(ctx context.Context, pool *pgxpool.Pool) error {
	_, err := pool.Exec(ctx, `
CREATE TABLE IF NOT EXISTS live_session (
  id SMALLINT PRIMARY KEY DEFAULT 1 CHECK (id = 1),
  data JSONB NOT NULL,
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS session_archives (
  id TEXT PRIMARY KEY,
  started_at TIMESTAMPTZ NOT NULL,
  ended_at TIMESTAMPTZ NOT NULL,
  started_label TEXT NOT NULL,
  ended_label TEXT NOT NULL,
  total_human TEXT NOT NULL,
  work_human TEXT NOT NULL,
  late_human TEXT NOT NULL,
  worked_hours DOUBLE PRECISION NOT NULL DEFAULT 0,
  transitions INT NOT NULL DEFAULT 0,
  late_min DOUBLE PRECISION NOT NULL DEFAULT 0,
  archive JSONB NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS session_archives_ended_at_idx
  ON session_archives (ended_at DESC);
`)
	return err
}

func databaseURL() string {
	if v := os.Getenv("DATABASE_URL"); v != "" {
		return v
	}
	return "postgres://durus:durus@127.0.0.1:5433/durus?sslmode=disable"
}
