package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"

	"quoteservice/internal/config"
	"quoteservice/internal/repository"
)

// Mock repository
type mockQuoteRepo struct {
	createUpdateFunc     func(ctx context.Context, base, quote, id string) (string, error)
	markRunningFunc      func(ctx context.Context, id string) error
	markSuccessFunc      func(ctx context.Context, id, price string) error
	markFailedFunc       func(ctx context.Context, id, errorMsg string) error
	getByIDFunc          func(ctx context.Context, id string) (*repository.Quote, error)
	getLatestSuccessFunc func(ctx context.Context, base, quote string) (*repository.Quote, error)
}

func (m *mockQuoteRepo) CreateUpdate(ctx context.Context, base, quote, id string) (string, error) {
	return m.createUpdateFunc(ctx, base, quote, id)
}

func (m *mockQuoteRepo) MarkRunning(ctx context.Context, id string) error {
	return m.markRunningFunc(ctx, id)
}

func (m *mockQuoteRepo) MarkSuccess(ctx context.Context, id, price string) error {
	return m.markSuccessFunc(ctx, id, price)
}

func (m *mockQuoteRepo) MarkFailed(ctx context.Context, id, errorMsg string) error {
	return m.markFailedFunc(ctx, id, errorMsg)
}

func (m *mockQuoteRepo) GetByID(ctx context.Context, id string) (*repository.Quote, error) {
	return m.getByIDFunc(ctx, id)
}

func (m *mockQuoteRepo) GetLatestSuccess(ctx context.Context, base, quote string) (*repository.Quote, error) {
	return m.getLatestSuccessFunc(ctx, base, quote)
}

// Mock provider
type mockRatesProvider struct {
	getRateFunc func(base string, quote string) (string, time.Time, error)
}

func (m *mockRatesProvider) GetRate(_ context.Context, base string, quote string) (string, time.Time, error) {
	return m.getRateFunc(base, quote)
}

var testCacheCfg = config.CacheConfig{
	LatestPriceTTLSec:           3600,
	ExchangeProviderPriceTTLSec: 3600,
}

func TestIsValidCurrencyCode(t *testing.T) {
	tests := []struct {
		code  string
		valid bool
	}{
		{"USD", true},
		{"EUR", true},
		{"MXN", true},
		{"usd", true},   // should accept lowercase and convert
		{"US", false},   // too short
		{"USDA", false}, // too long
		{"US1", false},  // contains number
		{"US$", false},  // contains special char
		{"", false},     // empty
	}

	for _, tc := range tests {
		t.Run(tc.code, func(t *testing.T) {
			result := IsValidCurrencyCode(tc.code)
			if result != tc.valid {
				t.Errorf("IsValidCurrencyCode(%q) = %v, want %v", tc.code, result, tc.valid)
			}
		})
	}
}

func TestRequestQuoteUpdate_Validation(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	sugar := logger.Sugar()
	v := NewValidator()

	// For validation-only tests, we don't need real implementations
	// We just test that invalid formats are rejected early
	tests := []struct {
		pair      string
		shouldErr bool
		errType   error
	}{
		{"INVALID", true, ErrInvalidPairFormat},
		{"EU/MXN", true, ErrInvalidPairFormat},   // too short
		{"EURO/MXN", true, ErrInvalidPairFormat}, // too long
		{"EUR/MX", true, ErrInvalidPairFormat},   // quote too short
		{"EUR/MXNA", true, ErrInvalidPairFormat}, // quote too long
		{"123/MXN", true, ErrInvalidPairFormat},  // contains numbers
		{"EUR/12N", true, ErrInvalidPairFormat},  // quote contains numbers
		{"EUR-MXN", true, ErrInvalidPairFormat},  // wrong separator
		{"", true, ErrInvalidPairFormat},         // empty
		{"ABC/USD", true, ErrUnsupportedCurrency},
		{"USD/XYZ", true, ErrUnsupportedCurrency},
	}

	for _, tc := range tests {
		t.Run(tc.pair, func(t *testing.T) {
			repo := &mockQuoteRepo{}
			// No taskEnqueuer needed for validation errors
			svc := NewQuoteService(repo, nil, v, nil, nil, sugar, testCacheCfg)

			_, _, err := svc.RequestQuoteUpdate(context.Background(), tc.pair)
			if tc.shouldErr && err == nil {
				t.Errorf("Expected error for pair %q, got nil", tc.pair)
			}
			if tc.shouldErr && err != tc.errType {
				t.Errorf("Expected error %v, got %v", tc.errType, err)
			}
		})
	}
}

func TestGetLatestQuote_Validation(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	sugar := logger.Sugar()
	v := NewValidator()

	tests := []struct {
		base      string
		quote     string
		shouldErr bool
		errType   error
	}{
		{"EU", "MXN", true, ErrInvalidPairFormat},   // too short
		{"EURO", "MXN", true, ErrInvalidPairFormat}, // too long
		{"EUR", "MX", true, ErrInvalidPairFormat},   // quote too short
		{"123", "MXN", true, ErrInvalidPairFormat},  // contains numbers
		{"EUR", "12N", true, ErrInvalidPairFormat},  // quote contains numbers
		{"", "MXN", true, ErrInvalidPairFormat},     // empty base
		{"EUR", "", true, ErrInvalidPairFormat},     // empty quote
		{"ABC", "USD", true, ErrUnsupportedCurrency},
		{"USD", "XYZ", true, ErrUnsupportedCurrency},
	}

	for _, tc := range tests {
		t.Run(tc.base+"/"+tc.quote, func(t *testing.T) {
			repo := &mockQuoteRepo{}
			svc := NewQuoteService(repo, nil, v, nil, nil, sugar, testCacheCfg)

			_, err := svc.GetLatestQuote(context.Background(), tc.base, tc.quote)
			if tc.shouldErr && err != tc.errType {
				t.Errorf("Expected error %v for %s/%s, got %v", tc.errType, tc.base, tc.quote, err)
			}
		})
	}
}

func TestGetQuoteResult_InvalidUUID(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	sugar := logger.Sugar()
	v := NewValidator()

	svc := NewQuoteService(nil, nil, v, nil, nil, sugar, testCacheCfg)

	_, err := svc.GetQuoteResult(context.Background(), "not-a-uuid")
	if !errors.Is(err, ErrInvalidUpdateID) {
		t.Errorf("Expected ErrInvalidUpdateID, got %v", err)
	}
}

func TestProcessUpdate_Success(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	sugar := logger.Sugar()
	v := NewValidator()

	repo := &mockQuoteRepo{
		markRunningFunc: func(ctx context.Context, id string) error {
			return nil
		},
		markSuccessFunc: func(ctx context.Context, id, price string) error {
			if price != "18.7543" {
				t.Errorf("Expected price 18.7543, got %s", price)
			}
			return nil
		},
	}

	provider := &mockRatesProvider{
		getRateFunc: func(base string, quote string) (string, time.Time, error) {
			return "18.7543", time.Now(), nil
		},
	}

	// Use miniredis for proper mocking
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("failed to start miniredis: %v", err)
	}
	defer mr.Close()

	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})

	svc := NewQuoteService(repo, provider, v, nil, rdb, sugar, testCacheCfg)

	err = svc.ProcessUpdate(context.Background(), "test-id", "EUR", "MXN")
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	// Verify cache was updated
	key := "latest:{EUR:MXN}"
	if !mr.Exists(key) {
		t.Errorf("Expected key %s to exist in Redis", key)
	}
	price := mr.HGet(key, "price")
	if price != "18.7543" {
		t.Errorf("Expected cached price 18.7543, got %s", price)
	}
}

func TestProcessUpdate_Failure(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	sugar := logger.Sugar()
	v := NewValidator()

	repo := &mockQuoteRepo{
		markRunningFunc: func(ctx context.Context, id string) error {
			return nil
		},
		markFailedFunc: func(ctx context.Context, id, errorMsg string) error {
			if errorMsg == "" {
				t.Error("Expected error message, got empty string")
			}
			return nil
		},
	}

	provider := &mockRatesProvider{
		getRateFunc: func(base string, quote string) (string, time.Time, error) {
			return "", time.Time{}, errors.New("provider error")
		},
	}

	svc := NewQuoteService(repo, provider, v, nil, nil, sugar, testCacheCfg)

	err := svc.ProcessUpdate(context.Background(), "test-id", "EUR", "MXN")
	if err == nil {
		t.Error("Expected error, got nil")
	}
}

func TestGetLatestQuote_Cached(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	sugar := logger.Sugar()
	v := NewValidator()

	// Use miniredis
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("failed to start miniredis: %v", err)
	}
	defer mr.Close()

	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})

	// Set value in cache
	key := "latest:{EUR:MXN}"
	mr.HSet(key, "price", "18.7543")
	mr.HSet(key, "updated_at", time.Now().Format(time.RFC3339))

	// Repo should NOT be called if cached
	repo := &mockQuoteRepo{
		getLatestSuccessFunc: func(ctx context.Context, base, quote string) (*repository.Quote, error) {
			t.Error("Repo should not be called when value is cached")
			return nil, nil
		},
	}

	svc := NewQuoteService(repo, nil, v, nil, rdb, sugar, testCacheCfg)

	res, err := svc.GetLatestQuote(context.Background(), "EUR", "MXN")
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if res.Price == nil || *res.Price != "18.7543" {
		t.Errorf("Expected price 18.7543, got %v", res.Price)
	}
}

func TestGetLatestQuote_NotCached(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	sugar := logger.Sugar()
	v := NewValidator()

	// Use miniredis
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("failed to start miniredis: %v", err)
	}
	defer mr.Close()

	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})

	now := time.Now().Truncate(time.Second)
	price := "18.7543"

	repo := &mockQuoteRepo{
		getLatestSuccessFunc: func(ctx context.Context, base, quote string) (*repository.Quote, error) {
			return &repository.Quote{
				Base:      base,
				Quote:     quote,
				Price:     &price,
				UpdatedAt: &now,
				Status:    repository.StatusSuccess,
			}, nil
		},
	}

	svc := NewQuoteService(repo, nil, v, nil, rdb, sugar, testCacheCfg)

	res, err := svc.GetLatestQuote(context.Background(), "EUR", "MXN")
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if res.Price == nil || *res.Price != price {
		t.Errorf("Expected price %s, got %v", price, res.Price)
	}

	// Verify it was cached
	key := "latest:{EUR:MXN}"
	if !mr.Exists(key) {
		t.Errorf("Expected key %s to be cached", key)
	}
	cachedPrice := mr.HGet(key, "price")
	if cachedPrice != price {
		t.Errorf("Expected cached price %s, got %s", price, cachedPrice)
	}
}

// Mock task enqueuer
type mockTaskEnqueuer struct {
	enqueueUpdateTaskFunc func(ctx context.Context, payload UpdateQuotePayload) error
}

func (m *mockTaskEnqueuer) EnqueueUpdateTask(ctx context.Context, payload UpdateQuotePayload) error {
	return m.enqueueUpdateTaskFunc(ctx, payload)
}

func TestRequestQuoteUpdate_EnqueueSuccess(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	sugar := logger.Sugar()
	v := NewValidator()

	repo := &mockQuoteRepo{
		createUpdateFunc: func(ctx context.Context, base, quote, id string) (string, error) {
			// Return the same ID to indicate a new record was created
			return id, nil
		},
	}

	enqueueCalled := false
	enqueuer := &mockTaskEnqueuer{
		enqueueUpdateTaskFunc: func(ctx context.Context, payload UpdateQuotePayload) error {
			enqueueCalled = true
			if payload.Base != "EUR" || payload.Quote != "MXN" {
				t.Errorf("Expected pair EUR/MXN, got %s/%s", payload.Base, payload.Quote)
			}
			return nil
		},
	}

	svc := NewQuoteService(repo, nil, v, enqueuer, nil, sugar, testCacheCfg)

	updateID, status, err := svc.RequestQuoteUpdate(context.Background(), "EUR/MXN")
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}
	if status != string(repository.StatusPending) {
		t.Errorf("Expected status %s, got %s", repository.StatusPending, status)
	}
	if updateID == "" {
		t.Error("Expected non-empty updateID")
	}
	if !enqueueCalled {
		t.Error("Expected EnqueueUpdateTask to be called")
	}
}

func TestRequestQuoteUpdate_EnqueueFailure(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	sugar := logger.Sugar()
	v := NewValidator()

	markFailedCalled := false
	repo := &mockQuoteRepo{
		createUpdateFunc: func(ctx context.Context, base, quote, id string) (string, error) {
			return id, nil
		},
		markFailedFunc: func(ctx context.Context, id, errorMsg string) error {
			markFailedCalled = true
			if errorMsg != "enqueue error" {
				t.Errorf("Expected error message 'enqueue error', got %q", errorMsg)
			}
			return nil
		},
	}

	enqueuer := &mockTaskEnqueuer{
		enqueueUpdateTaskFunc: func(ctx context.Context, payload UpdateQuotePayload) error {
			return errors.New("redis connection refused")
		},
	}

	svc := NewQuoteService(repo, nil, v, enqueuer, nil, sugar, testCacheCfg)

	_, _, err := svc.RequestQuoteUpdate(context.Background(), "EUR/MXN")
	if !errors.Is(err, ErrInternalQueue) {
		t.Errorf("Expected ErrInternalQueue, got %v", err)
	}
	if !markFailedCalled {
		t.Error("Expected MarkFailed to be called")
	}
}

func TestRequestQuoteUpdate_ExistingPending(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	sugar := logger.Sugar()
	v := NewValidator()

	existingID := "existing-uuid-1234"
	repo := &mockQuoteRepo{
		createUpdateFunc: func(ctx context.Context, base, quote, id string) (string, error) {
			// Return a different ID to simulate dedup (existing pending record)
			return existingID, nil
		},
	}

	enqueueCalled := false
	enqueuer := &mockTaskEnqueuer{
		enqueueUpdateTaskFunc: func(ctx context.Context, payload UpdateQuotePayload) error {
			enqueueCalled = true
			return nil
		},
	}

	svc := NewQuoteService(repo, nil, v, enqueuer, nil, sugar, testCacheCfg)

	updateID, status, err := svc.RequestQuoteUpdate(context.Background(), "EUR/MXN")
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}
	if updateID != existingID {
		t.Errorf("Expected existing ID %s, got %s", existingID, updateID)
	}
	if status != string(repository.StatusPending) {
		t.Errorf("Expected status %s, got %s", repository.StatusPending, status)
	}
	if enqueueCalled {
		t.Error("Expected Enqueue NOT to be called for existing pending record")
	}
}
