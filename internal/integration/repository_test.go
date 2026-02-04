//go:build integration

package integration

import (
	"context"
	"testing"

	"github.com/google/uuid"

	"quoteservice/internal/repository"
)

func newRepo() repository.QuoteRepository {
	return repository.NewPostgresQuoteRepository(testDB)
}

func TestCreateUpdate(t *testing.T) {
	resetTestData(t)
	ctx := testContext(t)
	repo := newRepo()

	id := uuid.New().String()
	got, err := repo.CreateUpdate(ctx, "USD", "EUR", id)
	if err != nil {
		t.Fatalf("CreateUpdate: %v", err)
	}
	if got != id {
		t.Fatalf("expected id %s, got %s", id, got)
	}

	// Verify DB state.
	q, err := repo.GetByID(ctx, id)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if q == nil {
		t.Fatal("expected quote record, got nil")
	}
	if q.Base != "USD" || q.Quote != "EUR" {
		t.Fatalf("expected USD/EUR, got %s/%s", q.Base, q.Quote)
	}
	if q.Status != repository.StatusPending {
		t.Fatalf("expected PENDING, got %s", q.Status)
	}
}

func TestCreateUpdate_Dedup(t *testing.T) {
	resetTestData(t)
	ctx := testContext(t)
	repo := newRepo()

	id1 := uuid.New().String()
	got1, err := repo.CreateUpdate(ctx, "USD", "EUR", id1)
	if err != nil {
		t.Fatalf("first CreateUpdate: %v", err)
	}
	if got1 != id1 {
		t.Fatalf("expected id1 %s, got %s", id1, got1)
	}

	// Second call for same pair while PENDING should return existing ID.
	id2 := uuid.New().String()
	got2, err := repo.CreateUpdate(ctx, "USD", "EUR", id2)
	if err != nil {
		t.Fatalf("second CreateUpdate: %v", err)
	}
	if got2 != id1 {
		t.Fatalf("expected dedup to return %s, got %s", id1, got2)
	}
}

func TestCreateUpdate_AfterCompletion(t *testing.T) {
	resetTestData(t)
	ctx := testContext(t)
	repo := newRepo()

	id1 := uuid.New().String()
	_, err := repo.CreateUpdate(ctx, "USD", "EUR", id1)
	if err != nil {
		t.Fatalf("CreateUpdate: %v", err)
	}

	if err := repo.MarkRunning(ctx, id1); err != nil {
		t.Fatalf("MarkRunning: %v", err)
	}
	if err := repo.MarkSuccess(ctx, id1, "1.1234"); err != nil {
		t.Fatalf("MarkSuccess: %v", err)
	}

	id2 := uuid.New().String()
	got, err := repo.CreateUpdate(ctx, "USD", "EUR", id2)
	if err != nil {
		t.Fatalf("CreateUpdate after completion: %v", err)
	}
	if got != id2 {
		t.Fatalf("expected new id %s, got %s", id2, got)
	}
}

func TestMarkRunning(t *testing.T) {
	resetTestData(t)
	ctx := testContext(t)
	repo := newRepo()

	id := uuid.New().String()
	if _, err := repo.CreateUpdate(ctx, "GBP", "JPY", id); err != nil {
		t.Fatalf("CreateUpdate: %v", err)
	}
	if err := repo.MarkRunning(ctx, id); err != nil {
		t.Fatalf("MarkRunning: %v", err)
	}

	t.Run("status is RUNNING", func(t *testing.T) {
		q, err := repo.GetByID(ctx, id)
		if err != nil {
			t.Fatalf("GetByID: %v", err)
		}
		if q == nil {
			t.Fatal("expected record, got nil")
		}
		if q.Status != repository.StatusRunning {
			t.Fatalf("expected RUNNING, got %s", q.Status)
		}
	})

	t.Run("second call fails", func(t *testing.T) {
		if err := repo.MarkRunning(ctx, id); err == nil {
			t.Fatal("expected error for MarkRunning on non-PENDING record, got nil")
		}
	})
}

func setupRunningUpdate(t *testing.T, base, quote string) (context.Context, repository.QuoteRepository, string) {
	t.Helper()
	resetTestData(t)
	ctx := testContext(t)
	repo := newRepo()

	id := uuid.New().String()
	if _, err := repo.CreateUpdate(ctx, base, quote, id); err != nil {
		t.Fatalf("CreateUpdate: %v", err)
	}
	if err := repo.MarkRunning(ctx, id); err != nil {
		t.Fatalf("MarkRunning: %v", err)
	}
	return ctx, repo, id
}

func TestMarkSuccess(t *testing.T) {
	ctx, repo, id := setupRunningUpdate(t, "USD", "GBP")

	if err := repo.MarkSuccess(ctx, id, "0.7890"); err != nil {
		t.Fatalf("MarkSuccess: %v", err)
	}

	q, err := repo.GetByID(ctx, id)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if q == nil {
		t.Fatal("expected record, got nil")
	}
	if q.Status != repository.StatusSuccess {
		t.Fatalf("expected SUCCESS, got %s", q.Status)
	}
	if q.Price == nil || *q.Price != "0.789000" {
		var got string
		if q.Price != nil {
			got = *q.Price
		}
		t.Fatalf("expected price 0.789000, got %s", got)
	}
	if q.UpdatedAt == nil {
		t.Fatal("expected updated_at to be set")
	}
}

func TestMarkFailed_FromRunning(t *testing.T) {
	ctx, repo, id := setupRunningUpdate(t, "USD", "GBP")

	errMsg := "provider timeout"
	if err := repo.MarkFailed(ctx, id, errMsg); err != nil {
		t.Fatalf("MarkFailed: %v", err)
	}

	q, err := repo.GetByID(ctx, id)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if q == nil {
		t.Fatal("expected record, got nil")
	}
	if q.Status != repository.StatusFailed {
		t.Fatalf("expected FAILED, got %s", q.Status)
	}
	if q.Price != nil {
		t.Fatalf("expected nil price for FAILED status, got %s", *q.Price)
	}
	if q.ErrorMsg == nil || *q.ErrorMsg != errMsg {
		t.Fatalf("expected error message %q, got %v", errMsg, q.ErrorMsg)
	}
}

func TestMarkFailed_FromPending(t *testing.T) {
	resetTestData(t)
	ctx := testContext(t)
	repo := newRepo()

	id := uuid.New().String()
	if _, err := repo.CreateUpdate(ctx, "USD", "GBP", id); err != nil {
		t.Fatalf("CreateUpdate: %v", err)
	}

	errMsg := "enqueue error"
	if err := repo.MarkFailed(ctx, id, errMsg); err != nil {
		t.Fatalf("MarkFailed from PENDING: %v", err)
	}

	q, err := repo.GetByID(ctx, id)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if q == nil {
		t.Fatal("expected record, got nil")
	}
	if q.Status != repository.StatusFailed {
		t.Fatalf("expected FAILED, got %s", q.Status)
	}
	if q.ErrorMsg == nil || *q.ErrorMsg != errMsg {
		t.Fatalf("expected error message %q, got %v", errMsg, q.ErrorMsg)
	}
}

func TestMarkSuccess_WrongStatus(t *testing.T) {
	resetTestData(t)
	ctx := testContext(t)
	repo := newRepo()

	id := uuid.New().String()
	if _, err := repo.CreateUpdate(ctx, "USD", "GBP", id); err != nil {
		t.Fatalf("CreateUpdate: %v", err)
	}

	// Try to mark success while still PENDING (not RUNNING).
	if err := repo.MarkSuccess(ctx, id, "1.0000"); err == nil {
		t.Fatal("expected error for MarkSuccess on non-RUNNING record, got nil")
	}
}

func TestMarkFailed_WrongStatus(t *testing.T) {
	ctx, repo, id := setupRunningUpdate(t, "USD", "GBP")

	if err := repo.MarkSuccess(ctx, id, "1.0000"); err != nil {
		t.Fatalf("MarkSuccess: %v", err)
	}

	// Try to mark failed on an already completed (SUCCESS) record.
	if err := repo.MarkFailed(ctx, id, "some error"); err == nil {
		t.Fatal("expected error for MarkFailed on SUCCESS record, got nil")
	}
}

func TestGetByID(t *testing.T) {
	resetTestData(t)
	ctx := testContext(t)
	repo := newRepo()

	id := uuid.New().String()
	if _, err := repo.CreateUpdate(ctx, "EUR", "CHF", id); err != nil {
		t.Fatalf("CreateUpdate: %v", err)
	}

	q, err := repo.GetByID(ctx, id)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if q == nil {
		t.Fatal("expected record, got nil")
	}
	if q.ID != id {
		t.Fatalf("expected ID %s, got %s", id, q.ID)
	}
	if q.Base != "EUR" || q.Quote != "CHF" {
		t.Fatalf("expected EUR/CHF, got %s/%s", q.Base, q.Quote)
	}
	if q.Status != repository.StatusPending {
		t.Fatalf("expected PENDING, got %s", q.Status)
	}
	if q.RequestedAt.IsZero() {
		t.Fatal("expected requested_at to be set")
	}
}

func TestGetByID_NotFound(t *testing.T) {
	resetTestData(t)
	ctx := testContext(t)
	repo := newRepo()

	q, err := repo.GetByID(ctx, uuid.New().String())
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if q != nil {
		t.Fatalf("expected nil for unknown UUID, got %+v", q)
	}
}

func TestGetLatestSuccess(t *testing.T) {
	resetTestData(t)
	ctx := testContext(t)
	repo := newRepo()

	// Create two successful records for same pair.
	id1 := uuid.New().String()
	if _, err := repo.CreateUpdate(ctx, "USD", "EUR", id1); err != nil {
		t.Fatalf("CreateUpdate 1: %v", err)
	}
	if err := repo.MarkRunning(ctx, id1); err != nil {
		t.Fatalf("MarkRunning 1: %v", err)
	}
	if err := repo.MarkSuccess(ctx, id1, "1.1000"); err != nil {
		t.Fatalf("MarkSuccess 1: %v", err)
	}

	// Need to complete first before inserting second (unique partial index).
	id2 := uuid.New().String()
	if _, err := repo.CreateUpdate(ctx, "USD", "EUR", id2); err != nil {
		t.Fatalf("CreateUpdate 2: %v", err)
	}
	if err := repo.MarkRunning(ctx, id2); err != nil {
		t.Fatalf("MarkRunning 2: %v", err)
	}
	if err := repo.MarkSuccess(ctx, id2, "1.2000"); err != nil {
		t.Fatalf("MarkSuccess 2: %v", err)
	}

	q, err := repo.GetLatestSuccess(ctx, "USD", "EUR")
	if err != nil {
		t.Fatalf("GetLatestSuccess: %v", err)
	}
	if q == nil {
		t.Fatal("expected record, got nil")
	}
	// Should return the most recent one (id2).
	if q.ID != id2 {
		t.Fatalf("expected latest id %s, got %s", id2, q.ID)
	}
	if q.Price == nil || *q.Price != "1.200000" {
		var got string
		if q.Price != nil {
			got = *q.Price
		}
		t.Fatalf("expected price 1.200000, got %s", got)
	}
}

func TestGetLatestSuccess_NotFound(t *testing.T) {
	resetTestData(t)
	ctx := testContext(t)
	repo := newRepo()

	q, err := repo.GetLatestSuccess(ctx, "AAA", "BBB")
	if err != nil {
		t.Fatalf("GetLatestSuccess: %v", err)
	}
	if q != nil {
		t.Fatalf("expected nil for unknown pair, got %+v", q)
	}
}
