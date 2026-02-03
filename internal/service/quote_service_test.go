package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"

	"quoteservice/internal/config"
	"quoteservice/internal/repository"
)

// Mock repository
type mockQuoteRepo struct {
	createUpdateFunc     func(ctx context.Context, base, quote, id string) (string, error)
	markRunningFunc      func(ctx context.Context, id string) error
	markCompletedFunc    func(ctx context.Context, id string, price string, status repository.Status, errorMsg *string) error
	getByIDFunc          func(ctx context.Context, id string) (*repository.Quote, error)
	getLatestSuccessFunc func(ctx context.Context, base, quote string) (*repository.Quote, error)
}

func (m *mockQuoteRepo) CreateUpdate(ctx context.Context, base, quote, id string) (string, error) {
	return m.createUpdateFunc(ctx, base, quote, id)
}

func (m *mockQuoteRepo) MarkRunning(ctx context.Context, id string) error {
	return m.markRunningFunc(ctx, id)
}

func (m *mockQuoteRepo) MarkCompleted(ctx context.Context, id string, price string, status repository.Status, errorMsg *string) error {
	return m.markCompletedFunc(ctx, id, price, status, errorMsg)
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

func (m *mockRatesProvider) GetRate(base string, quote string) (string, time.Time, error) {
	return m.getRateFunc(base, quote)
}

var testCacheCfg = config.CacheConfig{
	TTLSec:         3600,
	TaskMaxRetry:   3,
	TaskTimeoutSec: 30,
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
			// No taskClient needed for validation errors
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
	if err != ErrInvalidUpdateID {
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
		markCompletedFunc: func(ctx context.Context, id string, price string, status repository.Status, errorMsg *string) error {
			if status != repository.StatusSuccess {
				t.Errorf("Expected status SUCCESS, got %s", status)
			}
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

	// Create a minimal redis client for testing (won't actually connect)
	rdb := redis.NewClient(&redis.Options{Addr: "localhost:6379"})

	svc := NewQuoteService(repo, provider, v, nil, rdb, sugar, testCacheCfg)

	err := svc.ProcessUpdate(context.Background(), "test-id", "EUR", "MXN")
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
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
		markCompletedFunc: func(ctx context.Context, id string, price string, status repository.Status, errorMsg *string) error {
			if status != repository.StatusFailed {
				t.Errorf("Expected status FAILED, got %s", status)
			}
			if errorMsg == nil {
				t.Error("Expected error message, got nil")
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
