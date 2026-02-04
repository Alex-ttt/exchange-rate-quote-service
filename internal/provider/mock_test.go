package provider

import (
	"context"
	"time"

	"github.com/stretchr/testify/mock"
)

type MockProvider struct {
	mock.Mock
}

func (m *MockProvider) GetRate(ctx context.Context, base, quote string) (string, time.Time, error) {
	args := m.Called(ctx, base, quote)
	return args.String(0), args.Get(1).(time.Time), args.Error(2)
}
