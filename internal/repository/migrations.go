package repository

import (
	"database/sql"
	"embed"
	"fmt"

	"go.uber.org/zap"
)

//go:embed migrations/*.sql
var migrationsFS embed.FS

// RunMigrations applies SQL migrations from the migrations folder using transactions.
func RunMigrations(db *sql.DB, logger *zap.SugaredLogger) error {
	if err := ensureMigrationsTable(db); err != nil {
		return err
	}

	files, err := migrationsFS.ReadDir("migrations")
	if err != nil {
		return fmt.Errorf("migrations read error: %w", err)
	}

	for _, file := range files {
		if file.IsDir() {
			continue
		}
		name := file.Name()

		applied, err := isApplied(db, name)
		if err != nil {
			return err
		}
		if applied {
			logger.Infow("Skipping already applied migration", "migration", name)
			continue
		}

		sqlBytes, err := migrationsFS.ReadFile("migrations/" + name)
		if err != nil {
			return fmt.Errorf("read migration file %s: %w", name, err)
		}
		sqlScript := string(sqlBytes)

		if err := executeMigration(db, name, sqlScript, logger); err != nil {
			return err
		}
	}
	return nil
}

func ensureMigrationsTable(db *sql.DB) error {
	const query = `CREATE TABLE IF NOT EXISTS schema_migrations (
		version    TEXT PRIMARY KEY,
		applied_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
	)`
	if _, err := db.Exec(query); err != nil {
		return fmt.Errorf("create schema_migrations table: %w", err)
	}
	return nil
}

func isApplied(db *sql.DB, version string) (bool, error) {
	var exists bool
	err := db.QueryRow("SELECT EXISTS(SELECT 1 FROM schema_migrations WHERE version = $1)", version).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("check migration %s: %w", version, err)
	}
	return exists, nil
}

func executeMigration(db *sql.DB, name, sqlScript string, logger *zap.SugaredLogger) error {
	tx, err := db.Begin()
	if err != nil {
		return fmt.Errorf("begin transaction for migration %s: %w", name, err)
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback()
		}
	}()

	if _, err = tx.Exec(sqlScript); err != nil {
		return fmt.Errorf("execute migration %s: %w", name, err)
	}

	if _, err = tx.Exec("INSERT INTO schema_migrations (version) VALUES ($1)", name); err != nil {
		return fmt.Errorf("record migration %s: %w", name, err)
	}

	if err = tx.Commit(); err != nil {
		return fmt.Errorf("commit migration %s: %w", name, err)
	}

	logger.Infow("Applied migration", "migration", name)
	return nil
}
