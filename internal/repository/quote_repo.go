package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
)

// Status represents the state of a quote update request.
type Status string

// Status values for quote update lifecycle.
const (
	StatusPending Status = "PENDING"
	StatusRunning Status = "RUNNING"
	StatusSuccess Status = "SUCCESS"
	StatusFailed  Status = "FAILED"
)

// Quote represents a quote update record in the DB.
type Quote struct {
	ID          string
	Base        string
	Quote       string
	Price       *string
	Status      Status
	ErrorMsg    *string
	RequestedAt time.Time
	UpdatedAt   *time.Time
}

// QuoteRepository defines DB operations for quotes.
type QuoteRepository interface {
	CreateUpdate(ctx context.Context, base, quote, id string) (string, error)
	MarkRunning(ctx context.Context, id string) error
	MarkSuccess(ctx context.Context, id, price string) error
	MarkFailed(ctx context.Context, id, errorMsg string) error
	GetByID(ctx context.Context, id string) (*Quote, error)
	GetLatestSuccess(ctx context.Context, base, quote string) (*Quote, error)
}

// PostgresQuoteRepository is an implementation of QuoteRepository using PostgreSQL.
type PostgresQuoteRepository struct {
	db *sql.DB
}

// NewPostgresQuoteRepository creates a new PostgresQuoteRepository.
func NewPostgresQuoteRepository(db *sql.DB) QuoteRepository {
	return &PostgresQuoteRepository{db: db}
}

// CreateUpdate inserts a new quote update request. If an update for the same pair is already pending/running, it returns the existing one's ID.
func (r *PostgresQuoteRepository) CreateUpdate(ctx context.Context, base, quote, id string) (string, error) {
	query := `INSERT INTO quotes (id, base, quote, status, requested_at)
              VALUES ($1::uuid, $2, $3, 'PENDING'::quotes_status, NOW())
              ON CONFLICT (base, quote) WHERE status IN ('PENDING', 'RUNNING')
              DO UPDATE SET base = quotes.base  -- no-op, changes nothing
              RETURNING id::text`

	var returnedID string
	err := r.db.QueryRowContext(ctx, query, id, base, quote).Scan(&returnedID)
	if err != nil {
		return "", fmt.Errorf("failed to create update: %w", err)
	}
	return returnedID, nil
}

// MarkRunning updates a quote record status to RUNNING.
func (r *PostgresQuoteRepository) MarkRunning(ctx context.Context, id string) error {
	// Failed status can occur on Asynq retry
	query := `UPDATE quotes
				SET status=$1::quotes_status, updated_at=NOW()
				WHERE id=$2::uuid AND status IN ($3::quotes_status, $4::quotes_status)`
	result, err := r.db.ExecContext(ctx, query, StatusRunning, id, StatusPending, StatusFailed)
	if err != nil {
		return err
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return fmt.Errorf("quote %s not found or not in PENDING/FAILED status", id)
	}
	return nil
}

// MarkSuccess updates the quote record to SUCCESS with the fetched price.
func (r *PostgresQuoteRepository) MarkSuccess(ctx context.Context, id, price string) error {
	query := `UPDATE quotes
				SET status=$1::quotes_status,
				    price=$2::numeric,
				    updated_at=NOW()
				WHERE id=$3::uuid AND status=$4::quotes_status`

	result, err := r.db.ExecContext(ctx, query, StatusSuccess, price, id, StatusRunning)
	if err != nil {
		return err
	}
	return checkRowsAffected(result, id)
}

// MarkFailed updates the quote record to FAILED with an error message and NULL price.
func (r *PostgresQuoteRepository) MarkFailed(ctx context.Context, id, errorMsg string) error {
	query := `UPDATE quotes
				SET status=$1::quotes_status,
				    price=NULL,
				    error=$2,
				    updated_at=NOW()
				WHERE id=$3::uuid AND status IN ($4::quotes_status, $5::quotes_status)`

	result, err := r.db.ExecContext(ctx, query, StatusFailed, errorMsg, id, StatusPending, StatusRunning)
	if err != nil {
		return err
	}
	return checkRowsAffected(result, id)
}

func checkRowsAffected(result sql.Result, id string) error {
	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return fmt.Errorf("quote %s not found", id)
	}
	return nil
}

// GetByID retrieves a quote record by update_id.
func (r *PostgresQuoteRepository) GetByID(ctx context.Context, id string) (*Quote, error) {
	query := `SELECT id::text, base, quote, price, status, error, requested_at, updated_at
              FROM quotes
              WHERE id=$1::uuid`

	row := r.db.QueryRowContext(ctx, query, id)
	return scanQuote(row)
}

// GetLatestSuccess finds the most recent successful quote for the given currency pair.
func (r *PostgresQuoteRepository) GetLatestSuccess(ctx context.Context, base, quote string) (*Quote, error) {
	query := `SELECT id::text, base, quote, price, status, error, requested_at, updated_at
              FROM quotes
              WHERE base=$1 AND quote=$2 AND status=$3::quotes_status
              ORDER BY updated_at DESC
              LIMIT 1`

	row := r.db.QueryRowContext(ctx, query, base, quote, StatusSuccess)
	return scanQuote(row)
}

// scanQuote maps a single row into a Quote, returning (nil, nil) for sql.ErrNoRows.
func scanQuote(row *sql.Row) (*Quote, error) {
	var q Quote
	var price sql.NullString
	var updatedAt sql.NullTime
	var errMsg sql.NullString
	var statusStr string

	err := row.Scan(&q.ID, &q.Base, &q.Quote, &price, &statusStr, &errMsg, &q.RequestedAt, &updatedAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}

	q.Status = Status(statusStr)
	if price.Valid {
		q.Price = &price.String
	}
	if updatedAt.Valid {
		q.UpdatedAt = &updatedAt.Time
	}
	if errMsg.Valid {
		q.ErrorMsg = &errMsg.String
	}
	return &q, nil
}
