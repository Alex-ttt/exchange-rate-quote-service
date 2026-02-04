package service

import (
	"context"
	"time"

	"quoteservice/internal/repository"
)

const cacheKeyPrefixLatest = "latest:"

func latestCacheKey(base, quote string) string {
	return cacheKeyPrefixLatest + "{" + base + ":" + quote + "}"
}

func (s *QuoteService) cacheGetLatest(ctx context.Context, base, quote string) (*repository.Quote, bool) {
	if s.cache == nil {
		return nil, false
	}

	key := latestCacheKey(base, quote)
	vals, err := s.cache.HMGet(ctx, key, "price", "updated_at").Result()
	if err != nil || len(vals) != 2 || vals[0] == nil || vals[1] == nil {
		return nil, false
	}

	price, ok := asString(vals[0])
	if !ok {
		return nil, false
	}
	ts, ok := asString(vals[1])
	if !ok {
		return nil, false
	}
	t, err := timeParse(ts)
	if err != nil {
		return nil, false
	}

	return &repository.Quote{
		Base:      base,
		Quote:     quote,
		Status:    repository.StatusSuccess,
		Price:     &price,
		UpdatedAt: &t,
	}, true
}

func (s *QuoteService) cacheSetLatestFromQuote(ctx context.Context, q *repository.Quote) {
	if q == nil || q.Price == nil || q.UpdatedAt == nil {
		return
	}
	s.cacheSetLatest(ctx, q.Base, q.Quote, *q.Price, *q.UpdatedAt)
}

func (s *QuoteService) cacheSetLatest(ctx context.Context, base, quote, rate string, t time.Time) {
	if s.cache == nil {
		return
	}

	key := latestCacheKey(base, quote)
	pipe := s.cache.Pipeline()
	pipe.HSet(ctx, key, "price", rate, "updated_at", t.Format(time.RFC3339))
	pipe.Expire(ctx, key, s.latestPriceTTL)

	if _, err := pipe.Exec(ctx); err != nil {
		s.log.Warnw("Failed to update cache", "key", key, "error", err)
	}
}

func asString(v any) (string, bool) {
	switch x := v.(type) {
	case string:
		return x, true
	case []byte:
		return string(x), true
	default:
		return "", false
	}
}

func timeParse(s string) (time.Time, error) {
	return time.Parse(time.RFC3339, s)
}
