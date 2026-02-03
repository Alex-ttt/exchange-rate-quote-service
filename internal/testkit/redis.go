package testkit

import (
	"context"
	"fmt"
	"net/url"

	"github.com/testcontainers/testcontainers-go"
	tcredis "github.com/testcontainers/testcontainers-go/modules/redis"
)

// RedisModule wraps a Redis testcontainer and the addr (host:port) for the test instance.
type RedisModule struct {
	container testcontainers.Container
	addr      string
}

// Addr returns the host:port string for the Redis instance.
func (r *RedisModule) Addr() string { return r.addr }

// Terminate stops the container.
func (r *RedisModule) Terminate(ctx context.Context) error {
	if r.container == nil {
		return nil
	}
	return r.container.Terminate(ctx)
}

// StartRedis starts a Redis container and returns a RedisModule.
// If cfg.RedisAddr is set, no container is started and that addr is returned directly.
func StartRedis(ctx context.Context, cfg *Config) (*RedisModule, error) {
	if cfg.RedisAddr != "" {
		return &RedisModule{addr: cfg.RedisAddr}, nil
	}

	ctr, err := tcredis.Run(ctx, cfg.RedisImage)
	if err != nil {
		return nil, fmt.Errorf("start redis container: %w", err)
	}

	connStr, err := ctr.ConnectionString(ctx)
	if err != nil {
		_ = ctr.Terminate(ctx)
		return nil, fmt.Errorf("get redis connection string: %w", err)
	}

	// The project uses host:port addr format (not redis:// URLs), so extract it.
	addr, err := extractAddr(connStr)
	if err != nil {
		_ = ctr.Terminate(ctx)
		return nil, fmt.Errorf("parse redis connection string %q: %w", connStr, err)
	}

	return &RedisModule{
		container: ctr,
		addr:      addr,
	}, nil
}

// extractAddr parses a redis:// URL and returns host:port.
func extractAddr(connStr string) (string, error) {
	u, err := url.Parse(connStr)
	if err != nil {
		return "", err
	}
	return u.Host, nil
}
