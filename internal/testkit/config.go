// Package testkit provides test infrastructure for integration tests using testcontainers.
package testkit

import (
	"fmt"
	"os"
	"strconv"
	"time"
)

// Config holds environment-driven configuration for integration test infrastructure.
type Config struct {
	PGImage        string
	RedisImage     string
	PGDSN          string        // If set, skip Postgres container.
	RedisAddr      string        // If set, skip Redis container.
	StartupTimeout time.Duration // Max time to wait for containers to become ready.
	KeepContainers bool          // If true, do not terminate containers on shutdown.
}

// LoadConfig reads test infrastructure settings from environment variables.
func LoadConfig() Config {
	cfg := Config{
		PGImage:        envOrDefault("TEST_PG_IMAGE", "postgres:18.1-alpine"),
		RedisImage:     envOrDefault("TEST_REDIS_IMAGE", "redis:8.4.0-alpine"),
		PGDSN:          os.Getenv("TEST_PG_DSN"),
		RedisAddr:      os.Getenv("TEST_REDIS_ADDR"),
		StartupTimeout: envDurationOrDefault("TEST_STARTUP_TIMEOUT", 90*time.Second),
		KeepContainers: envBoolOrDefault("KEEP_CONTAINERS", false),
	}
	return cfg
}

func envOrDefault(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func envDurationOrDefault(key string, def time.Duration) time.Duration {
	v := os.Getenv(key)
	if v == "" {
		return def
	}
	d, err := time.ParseDuration(v)
	if err != nil {
		// Try parsing as plain seconds.
		secs, err2 := strconv.Atoi(v)
		if err2 != nil {
			fmt.Fprintf(os.Stderr, "testkit: invalid value %q for %s (expected duration or seconds), using default %v\n", v, key, def)
			return def
		}
		return time.Duration(secs) * time.Second
	}
	return d
}

func envBoolOrDefault(key string, def bool) bool {
	v := os.Getenv(key)
	if v == "" {
		return def
	}
	b, err := strconv.ParseBool(v)
	if err != nil {
		fmt.Fprintf(os.Stderr, "testkit: invalid value %q for %s (expected bool), using default %v\n", v, key, def)
		return def
	}
	return b
}
