package provider

import (
	"context"
	"time"
)

// RatesProvider defines an interface for fetching exchange rates from external sources.
type RatesProvider interface {
	GetRate(ctx context.Context, base, quote string) (string, time.Time, error)
}
