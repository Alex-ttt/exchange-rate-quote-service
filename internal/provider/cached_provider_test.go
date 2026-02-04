package provider

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestCachedRatesProvider_GetRate(t *testing.T) {
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("failed to start miniredis: %v", err)
	}
	defer mr.Close()

	rdb := redis.NewClient(&redis.Options{
		Addr: mr.Addr(),
	})

	base := "USD"
	quote := "EUR"
	rate := "0.85"
	now := time.Now().Truncate(time.Second).UTC()
	ttl := 10 * time.Second

	t.Run("cache miss then hit", func(t *testing.T) {
		mr.FlushAll()
		mockProv := new(MockProvider)
		mockProv.On("GetRate", mock.Anything, base, quote).Return(rate, now, nil).Once()

		cachedProv := NewCachedRatesProvider(mockProv, rdb, ttl, "test_provider")

		// First call - cache miss
		resRate, resTime, err := cachedProv.GetRate(context.Background(), base, quote)
		assert.NoError(t, err)
		assert.Equal(t, rate, resRate)
		assert.True(t, resTime.Equal(now))
		mockProv.AssertExpectations(t)

		// Second call - cache hit (MockProvider should NOT be called again because of .Once())
		resRate2, resTime2, err := cachedProv.GetRate(context.Background(), base, quote)
		assert.NoError(t, err)
		assert.Equal(t, rate, resRate2)
		assert.True(t, resTime2.Equal(now))
	})

	t.Run("provider error is not cached", func(t *testing.T) {
		mr.FlushAll()
		mockProv := new(MockProvider)
		mockProv.On("GetRate", mock.Anything, base, quote).Return("", time.Time{}, assert.AnError).Once()

		cachedProv := NewCachedRatesProvider(mockProv, rdb, ttl, "test_provider")

		// First call - provider error
		_, _, err := cachedProv.GetRate(context.Background(), base, quote)
		assert.Error(t, err)

		// Second call - provider should be called again
		mockProv.On("GetRate", mock.Anything, base, quote).Return(rate, now, nil).Once()
		resRate, _, err := cachedProv.GetRate(context.Background(), base, quote)
		assert.NoError(t, err)
		assert.Equal(t, rate, resRate)
		mockProv.AssertExpectations(t)
	})

	t.Run("cache expires", func(t *testing.T) {
		mr.FlushAll()
		mockProv := new(MockProvider)
		mockProv.On("GetRate", mock.Anything, base, quote).Return(rate, now, nil).Once()

		cachedProv := NewCachedRatesProvider(mockProv, rdb, ttl, "test_provider")

		_, _, _ = cachedProv.GetRate(context.Background(), base, quote)

		mr.FastForward(ttl + time.Second)

		// Second call - cache expired, should call provider again
		mockProv.On("GetRate", mock.Anything, base, quote).Return(rate, now, nil).Once()
		_, _, err := cachedProv.GetRate(context.Background(), base, quote)
		assert.NoError(t, err)
		mockProv.AssertExpectations(t)
	})
}
