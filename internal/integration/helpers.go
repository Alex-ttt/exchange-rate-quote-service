//go:build integration

package integration

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
)

var (
	testDB  *sql.DB
	testRDB *redis.Client
)

// resetTestData truncates the quotes table and flushes the current Redis database.
func resetTestData(t *testing.T) {
	t.Helper()

	_, err := testDB.ExecContext(context.Background(), "TRUNCATE TABLE quotes CASCADE")
	if err != nil {
		t.Fatalf("failed to truncate quotes table: %v", err)
	}

	if err := testRDB.FlushDB(context.Background()).Err(); err != nil {
		t.Fatalf("failed to flush redis: %v", err)
	}
}

// testContext returns a context with a 30-second deadline tied to the test's cleanup.
func testContext(t *testing.T) context.Context {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	t.Cleanup(cancel)
	return ctx
}
