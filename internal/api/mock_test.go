package api

import (
	"context"

	"quoteservice/internal/service"
)

// mockQuoteService implements service.QuoteServiceInterface for testing.
type mockQuoteService struct {
	requestUpdateFunc  func(ctx context.Context, pair string) (string, string, error)
	getQuoteResultFunc func(ctx context.Context, updateID string) (*service.QuoteResult, error)
	getLatestQuoteFunc func(ctx context.Context, base, quote string) (*service.QuoteResult, error)
}

func (m *mockQuoteService) RequestQuoteUpdate(ctx context.Context, pair string) (string, string, error) {
	return m.requestUpdateFunc(ctx, pair)
}

func (m *mockQuoteService) GetQuoteResult(ctx context.Context, updateID string) (*service.QuoteResult, error) {
	return m.getQuoteResultFunc(ctx, updateID)
}

func (m *mockQuoteService) GetLatestQuote(ctx context.Context, base, quote string) (*service.QuoteResult, error) {
	return m.getLatestQuoteFunc(ctx, base, quote)
}

func (m *mockQuoteService) ProcessUpdate(_ context.Context, _, _, _ string) error {
	return nil // Not used in handler tests
}
