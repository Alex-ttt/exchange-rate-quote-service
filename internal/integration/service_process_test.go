//go:build integration

package integration

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"

	"quoteservice/internal/config"
	"quoteservice/internal/provider"
	"quoteservice/internal/repository"
	"quoteservice/internal/service"
)

// fakeProvider implements provider.RatesProvider with a fixed rate.
type fakeProvider struct {
	rate string
}

func (f *fakeProvider) GetRate(_ context.Context, base, quote string) (string, time.Time, error) {
	return f.rate, time.Now().UTC(), nil
}

// Compile-time check that fakeProvider implements the interface.
var _ provider.RatesProvider = (*fakeProvider)(nil)

func TestProcessUpdate_FullLifecycle(t *testing.T) {
	resetTestData(t)
	ctx := testContext(t)

	repo := repository.NewPostgresQuoteRepository(testDB)
	logger := zap.NewNop().Sugar()
	prov := &fakeProvider{rate: "1.0850"}
	cacheCfg := config.CacheConfig{
		LatestPriceTTLSec:           3600,
		ExchangeProviderPriceTTLSec: 3600,
	}
	v := service.NewValidator()
	svc := service.NewQuoteService(repo, prov, v, nil, testRDB, logger, cacheCfg)

	// 1. Create a PENDING record.
	id := uuid.New().String()
	if _, err := repo.CreateUpdate(ctx, "USD", "EUR", id); err != nil {
		t.Fatalf("CreateUpdate: %v", err)
	}

	// 2. Process the update (marks RUNNING, fetches rate, marks SUCCESS, caches).
	if err := svc.ProcessUpdate(ctx, id, "USD", "EUR"); err != nil {
		t.Fatalf("ProcessUpdate: %v", err)
	}

	// 3. Verify DB record is SUCCESS with correct price.
	q, err := svc.GetQuoteResult(ctx, id)
	if err != nil {
		t.Fatalf("GetQuoteResult: %v", err)
	}
	if q.Status != "SUCCESS" {
		t.Fatalf("expected SUCCESS, got %s", q.Status)
	}
	if q.Price == nil || *q.Price != "1.085000" {
		var got string
		if q.Price != nil {
			got = *q.Price
		}
		t.Fatalf("expected price 1.085000, got %s", got)
	}

	// 4. Verify cache was populated via GetLatestQuote after truncating DB.
	if _, err := testDB.ExecContext(ctx, "TRUNCATE TABLE quotes CASCADE"); err != nil {
		t.Fatalf("truncate: %v", err)
	}

	cached, err := svc.GetLatestQuote(ctx, "USD", "EUR")
	if err != nil {
		t.Fatalf("GetLatestQuote (from cache): %v", err)
	}
	if cached == nil || cached.Price == nil || *cached.Price != "1.0850" {
		t.Fatal("expected cached rate 1.0850 after DB truncate")
	}
}
