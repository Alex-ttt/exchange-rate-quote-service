package repository

import (
	"database/sql"
	"embed"
	"fmt"
)

//go:embed migrations/*.sql
var migrationsFS embed.FS

// RunMigrations applies SQL migrations from the migrations folder using transactions.
func RunMigrations(db *sql.DB) error {
	// read all .sql files and execute them in order.
	files, err := migrationsFS.ReadDir("migrations")
	if err != nil {
		return fmt.Errorf("migrations read error: %w", err)
	}

	for _, file := range files {
		if file.IsDir() {
			continue
		}
		name := file.Name()
		sqlBytes, err := migrationsFS.ReadFile("migrations/" + name)
		if err != nil {
			return fmt.Errorf("read migration file %s: %w", name, err)
		}
		sqlScript := string(sqlBytes)

		if err := executeMigration(db, name, sqlScript); err != nil {
			return err
		}
	}
	return nil
}

func executeMigration(db *sql.DB, name, sqlScript string) error {
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

	if err = tx.Commit(); err != nil {
		return fmt.Errorf("commit migration %s: %w", name, err)
	}

	fmt.Printf("Applied migration %s\n", name)
	return nil
}
