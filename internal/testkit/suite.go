package testkit

import (
	"context"
	"fmt"
	"os"
	"sync"
	"testing"
)

// Suite manages the lifecycle of test infrastructure (Postgres and Redis containers).
type Suite struct {
	mu    sync.Mutex
	cfg   Config
	pg    *PostgresModule
	redis *RedisModule
	ready bool
}

var (
	globalSuite *Suite
	globalOnce  sync.Once
)

// Global returns the singleton Suite instance.
func Global() *Suite {
	globalOnce.Do(func() {
		globalSuite = &Suite{cfg: LoadConfig()}
	})
	return globalSuite
}

// Setup starts all required containers (or uses external overrides).
// Returns an error if called twice without Shutdown in between.
func (s *Suite) Setup(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.ready {
		return fmt.Errorf("suite already set up; call Shutdown first")
	}

	pg, err := StartPostgres(ctx, &s.cfg)
	if err != nil {
		return fmt.Errorf("setup postgres: %w", err)
	}
	s.pg = pg

	rdb, err := StartRedis(ctx, &s.cfg)
	if err != nil {
		// Clean up Postgres if Redis fails.
		if !s.cfg.KeepContainers {
			_ = pg.Terminate(ctx)
		}
		return fmt.Errorf("setup redis: %w", err)
	}
	s.redis = rdb
	s.ready = true

	return nil
}

// Shutdown terminates all containers unless KEEP_CONTAINERS is set.
func (s *Suite) Shutdown(ctx context.Context) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.ready {
		return
	}

	if s.cfg.KeepContainers {
		fmt.Println("KEEP_CONTAINERS=true — skipping container cleanup")
		if s.pg != nil {
			fmt.Println("  Postgres DSN:", s.pg.DSN())
		}
		if s.redis != nil {
			fmt.Println("  Redis Addr:", s.redis.Addr())
		}
		s.ready = false
		return
	}

	if s.redis != nil {
		if err := s.redis.Terminate(ctx); err != nil {
			fmt.Println("warning: failed to terminate redis container:", err)
		}
	}
	if s.pg != nil {
		if err := s.pg.Terminate(ctx); err != nil {
			fmt.Println("warning: failed to terminate postgres container:", err)
		}
	}
	s.ready = false
}

// PostgresDSN returns the connection string for the test Postgres database.
func (s *Suite) PostgresDSN() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.pg == nil {
		return ""
	}
	return s.pg.DSN()
}

// RedisAddr returns the host:port address for the test Redis instance.
func (s *Suite) RedisAddr() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.redis == nil {
		return ""
	}
	return s.redis.Addr()
}

// Run sets up the suite, calls optional afterSetup callbacks (e.g. for running
// migrations), executes tests, then shuts down. Intended for use in TestMain.
func (s *Suite) Run(m *testing.M, afterSetup ...func() error) {
	ctx := context.Background()

	if err := s.Setup(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "integration test setup failed: %v\n", err)
		os.Exit(1)
	}

	for _, fn := range afterSetup {
		if err := fn(); err != nil {
			fmt.Fprintf(os.Stderr, "afterSetup callback failed: %v\n", err)
			s.Shutdown(ctx)
			os.Exit(1)
		}
	}

	code := m.Run()

	s.Shutdown(ctx)
	os.Exit(code)
}

// Run is a package-level convenience that delegates to Global().Run.
// For a custom Suite instance, call the method directly: suite.Run(m, ...).
func Run(m *testing.M, afterSetup ...func() error) {
	Global().Run(m, afterSetup...)
}
