package provider

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestFallbackProvider_GetRate(t *testing.T) {
	t.Run("first succeeds", func(t *testing.T) {
		m1 := new(MockProvider)
		m2 := new(MockProvider)
		now := time.Now().UTC()

		m1.On("GetRate", mock.Anything, "EUR", "USD").Return("1.1", now, nil)

		p := NewExchangeProviderFacade(m1, m2)
		rate, timestamp, err := p.GetRate(context.Background(), "EUR", "USD")

		assert.NoError(t, err)
		assert.Equal(t, "1.1", rate)
		assert.Equal(t, now, timestamp)
		m1.AssertExpectations(t)
		m2.AssertNotCalled(t, "GetRate", mock.Anything, mock.Anything, mock.Anything)
	})

	t.Run("first fails, second succeeds", func(t *testing.T) {
		m1 := new(MockProvider)
		m2 := new(MockProvider)
		now := time.Now().UTC()

		m1.On("GetRate", mock.Anything, "EUR", "USD").Return("", time.Time{}, errors.New("m1 failed"))
		m2.On("GetRate", mock.Anything, "EUR", "USD").Return("1.2", now, nil)

		p := NewExchangeProviderFacade(m1, m2)
		rate, timestamp, err := p.GetRate(context.Background(), "EUR", "USD")

		assert.NoError(t, err)
		assert.Equal(t, "1.2", rate)
		assert.Equal(t, now, timestamp)
		m1.AssertExpectations(t)
		m2.AssertExpectations(t)
	})

	t.Run("all fail", func(t *testing.T) {
		m1 := new(MockProvider)
		m2 := new(MockProvider)

		m1.On("GetRate", mock.Anything, "EUR", "USD").Return("", time.Time{}, errors.New("m1 failed"))
		m2.On("GetRate", mock.Anything, "EUR", "USD").Return("", time.Time{}, errors.New("m2 failed"))

		p := NewExchangeProviderFacade(m1, m2)
		_, _, err := p.GetRate(context.Background(), "EUR", "USD")

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "all providers failed")
		assert.Contains(t, err.Error(), "m1 failed")
		assert.Contains(t, err.Error(), "m2 failed")
		m1.AssertExpectations(t)
		m2.AssertExpectations(t)
	})
}
