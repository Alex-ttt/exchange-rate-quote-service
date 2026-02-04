package provider

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

// CachedRatesProviderDecorator wraps a RatesProvider with Redis caching.
type CachedRatesProviderDecorator struct {
	provider     RatesProvider
	cache        *redis.Client
	ttl          time.Duration
	providerName string
}

// NewCachedRatesProvider creates a new CachedRatesProviderDecorator.
func NewCachedRatesProvider(provider RatesProvider, cache *redis.Client, ttl time.Duration, providerName string) *CachedRatesProviderDecorator {
	return &CachedRatesProviderDecorator{
		provider:     provider,
		cache:        cache,
		ttl:          ttl,
		providerName: providerName,
	}
}

func (p *CachedRatesProviderDecorator) cacheKey(base, quote string) string {
	return fmt.Sprintf("provider_cache:%s:{%s:%s}", p.providerName, base, quote)
}

// GetRate attempts to fetch the rate from cache before calling the underlying provider.
func (p *CachedRatesProviderDecorator) GetRate(ctx context.Context, base, quote string) (string, time.Time, error) {
	if p.cache == nil {
		return p.provider.GetRate(ctx, base, quote)
	}

	key := p.cacheKey(base, quote)

	// check cache
	vals, err := p.cache.HMGet(ctx, key, "price", "updated_at").Result()
	if err == nil && len(vals) == 2 && vals[0] != nil && vals[1] != nil {
		price, ok1 := vals[0].(string)
		tsStr, ok2 := vals[1].(string)
		if ok1 && ok2 {
			ts, err2 := time.Parse(time.RFC3339, tsStr)
			if err2 == nil {
				return price, ts, nil
			}
		}
	}

	price, ts, err := p.provider.GetRate(ctx, base, quote)
	if err != nil {
		return "", time.Time{}, err
	}

	pipe := p.cache.Pipeline()
	pipe.HSet(ctx, key, "price", price, "updated_at", ts.Format(time.RFC3339))
	pipe.Expire(ctx, key, p.ttl)
	_, _ = pipe.Exec(ctx)

	return price, ts, nil
}

var _ RatesProvider = (*CachedRatesProviderDecorator)(nil)
