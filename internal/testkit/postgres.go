package testkit

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"

	_ "github.com/jackc/pgx/v5/stdlib" // pgx driver registration
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
)

// PostgresModule wraps a Postgres testcontainer and the DSN for the test database.
type PostgresModule struct {
	container testcontainers.Container
	dsn       string
}

// DSN returns the connection string for the Postgres instance.
func (p *PostgresModule) DSN() string { return p.dsn }

// Terminate stops the Postgres container.
func (p *PostgresModule) Terminate(ctx context.Context) error {
	if p.container == nil {
		return nil
	}
	return p.container.Terminate(ctx)
}

// StartPostgres starts a Postgres container or uses an external DSN from config.
func StartPostgres(ctx context.Context, cfg *Config) (*PostgresModule, error) {
	if cfg.PGDSN != "" {
		return &PostgresModule{dsn: cfg.PGDSN}, nil
	}

	ctr, err := postgres.Run(ctx,
		cfg.PGImage,
		postgres.WithDatabase(randomDBName()),
		postgres.WithUsername("testuser"),
		postgres.WithPassword("testpass"),
		testcontainers.WithWaitStrategyAndDeadline(cfg.StartupTimeout,
			wait.ForLog("database system is ready to accept connections").WithOccurrence(2),
		),
	)
	if err != nil {
		return nil, fmt.Errorf("start postgres container: %w", err)
	}

	connStr, err := ctr.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		_ = ctr.Terminate(ctx)
		return nil, fmt.Errorf("get postgres connection string: %w", err)
	}

	return &PostgresModule{
		container: ctr,
		dsn:       connStr,
	}, nil
}

// randomDBName generates a random database name like "test_a1b2c3d4".
func randomDBName() string {
	b := make([]byte, 4)
	if _, err := rand.Read(b); err != nil {
		return "test_fallback"
	}
	return "test_" + hex.EncodeToString(b)
}
