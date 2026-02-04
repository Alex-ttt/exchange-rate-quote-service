//go:build integration

package integration

import (
	"context"
	"database/sql"
	"testing"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"

	"quoteservice/internal/repository"
	"quoteservice/internal/testkit"
)

func TestMain(m *testing.M) {
	testkit.Run(m, func() error {
		var err error
		testDB, err = sql.Open("pgx", testkit.Global().PostgresDSN())
		if err != nil {
			return err
		}
		if err := testDB.Ping(); err != nil {
			return err
		}
		if err := repository.RunMigrations(testDB, zap.NewNop().Sugar()); err != nil {
			return err
		}

		testRDB = redis.NewClient(&redis.Options{
			Addr: testkit.Global().RedisAddr(),
		})
		return testRDB.Ping(context.Background()).Err()
	})
}
