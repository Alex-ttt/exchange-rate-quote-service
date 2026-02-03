//go:build integration

package integration

import (
	"errors"
	"testing"

	"github.com/google/uuid"
	"go.uber.org/zap"

	"quoteservice/internal/config"
	"quoteservice/internal/repository"
	"quoteservice/internal/service"
)

// newCacheTestService creates a QuoteService wired to real Postgres and Redis
// but with nil provider and taskClient. Only suitable for testing GetLatestQuote.
func newCacheTestService() *service.QuoteService {
	repo := repository.NewPostgresQuoteRepository(testDB)
	logger := zap.NewNop().Sugar()
	cacheCfg := config.CacheConfig{
		TTLSec:         3600,
		TaskMaxRetry:   3,
		TaskTimeoutSec: 30,
	}
	v := service.NewValidator()
	return service.NewQuoteService(repo, nil, v, nil, testRDB, logger, cacheCfg)
}

// insertSuccessRecord is a test helper that creates a quote record and
// transitions it through PENDING → RUNNING → SUCCESS.
func insertSuccessRecord(t *testing.T, base, quote, price string) string {
	t.Helper()
	ctx := testContext(t)
	repo := repository.NewPostgresQuoteRepository(testDB)

	id := uuid.New().String()
	if _, err := repo.CreateUpdate(ctx, base, quote, id); err != nil {
		t.Fatalf("CreateUpdate: %v", err)
	}
	if err := repo.MarkRunning(ctx, id); err != nil {
		t.Fatalf("MarkRunning: %v", err)
	}
	if err := repo.MarkCompleted(ctx, id, price, repository.StatusSuccess, nil); err != nil {
		t.Fatalf("MarkCompleted: %v", err)
	}
	return id
}

func TestGetLatestQuote_CacheMiss_DBHit(t *testing.T) {
	resetTestData(t)
	ctx := testContext(t)

	insertSuccessRecord(t, "USD", "EUR", "1.0500")

	svc := newCacheTestService()
	q, err := svc.GetLatestQuote(ctx, "USD", "EUR")
	if err != nil {
		t.Fatalf("GetLatestQuote: %v", err)
	}
	if q == nil {
		t.Fatal("expected quote, got nil")
	}
	if q.Price == nil || *q.Price != "1.050000" {
		var got string
		if q.Price != nil {
			got = *q.Price
		}
		t.Fatalf("expected price 1.050000, got %s", got)
	}

	// Verify cache was populated: truncate DB and call again.
	// If the result still comes back, it must be from cache.
	if _, err := testDB.ExecContext(ctx, "TRUNCATE TABLE quotes CASCADE"); err != nil {
		t.Fatalf("truncate: %v", err)
	}

	q2, err := svc.GetLatestQuote(ctx, "USD", "EUR")
	if err != nil {
		t.Fatalf("GetLatestQuote (after truncate): %v", err)
	}
	if q2 == nil || q2.Price == nil || *q2.Price != "1.050000" {
		t.Fatal("expected cached result after DB truncate")
	}
}

func TestGetLatestQuote_CacheHit(t *testing.T) {
	resetTestData(t)
	ctx := testContext(t)

	// Populate cache by querying a real DB record through the service.
	insertSuccessRecord(t, "GBP", "JPY", "182.5000")
	svc := newCacheTestService()
	svc.GetLatestQuote(ctx, "GBP", "JPY") // populates cache

	// Truncate DB — proves the next call MUST come from cache.
	if _, err := testDB.ExecContext(ctx, "TRUNCATE TABLE quotes CASCADE"); err != nil {
		t.Fatalf("truncate: %v", err)
	}

	q, err := svc.GetLatestQuote(ctx, "GBP", "JPY")
	if err != nil {
		t.Fatalf("GetLatestQuote: %v", err)
	}
	if q == nil {
		t.Fatal("expected quote from cache, got nil")
	}
	if q.Price == nil || *q.Price != "182.500000" {
		var got string
		if q.Price != nil {
			got = *q.Price
		}
		t.Fatalf("expected price 182.500000, got %s", got)
	}
	if q.Base != "GBP" || q.Quote != "JPY" {
		t.Fatalf("expected GBP/JPY, got %s/%s", q.Base, q.Quote)
	}
}

func TestGetLatestQuote_NotFound(t *testing.T) {
	resetTestData(t)
	ctx := testContext(t)

	svc := newCacheTestService()
	_, err := svc.GetLatestQuote(ctx, "USD", "NOK") // Changed to supported currencies that aren't in DB
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !errors.Is(err, service.ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestGetLatestQuote_Unsupported(t *testing.T) {
	resetTestData(t)
	ctx := testContext(t)

	svc := newCacheTestService()
	_, err := svc.GetLatestQuote(ctx, "AAA", "BBB")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !errors.Is(err, service.ErrUnsupportedCurrency) {
		t.Fatalf("expected ErrUnsupportedCurrency, got %v", err)
	}
}
