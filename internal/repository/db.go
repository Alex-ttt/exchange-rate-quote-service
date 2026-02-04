// Package repository implements data access for quote storage and retrieval.
package repository

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"quoteservice/internal/config"

	_ "github.com/jackc/pgx/v5/stdlib" // pgx driver registration
)

// NewPostgresDB opens a database connection using the provided configuration.
func NewPostgresDB(cfg *config.DatabaseConfig) (*sql.DB, error) {
	db, err := sql.Open("pgx", cfg.DSN)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(cfg.MaxOpenConns)
	db.SetMaxIdleConns(cfg.MaxIdleConns)
	db.SetConnMaxLifetime(time.Duration(cfg.ConnMaxLifetimeSec) * time.Second)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("unable to connect to database: %w", err)
	}
	return db, nil
}
