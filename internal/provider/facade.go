package provider

import (
	"context"
	"errors"
	"fmt"
	"time"
)

var _ RatesProvider = (*ExchangeProviderFacade)(nil)

// ExchangeProviderFacade is an abstraction that calls providers sequentially.
type ExchangeProviderFacade struct {
	providers []RatesProvider
}

// NewExchangeProviderFacade creates a new ExchangeProviderFacade with the given list of providers.
func NewExchangeProviderFacade(providers ...RatesProvider) *ExchangeProviderFacade {
	return &ExchangeProviderFacade{
		providers: providers,
	}
}

// GetRate calls providers sequentially until one succeeds.
func (p *ExchangeProviderFacade) GetRate(ctx context.Context, base, quote string) (string, time.Time, error) {
	var errs []error
	for _, prov := range p.providers {
		rate, timestamp, err := prov.GetRate(ctx, base, quote)
		if err == nil {
			return rate, timestamp, nil
		}
		errs = append(errs, err)
	}

	return "", time.Time{}, fmt.Errorf("all providers failed: %w", errors.Join(errs...))
}
