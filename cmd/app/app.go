// Package main is the entry point for the exchange rate quote service.
package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/hibiken/asynq"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"

	"quoteservice/internal/config"
	"quoteservice/internal/provider"
	"quoteservice/internal/repository"
	"quoteservice/internal/service"
	"quoteservice/internal/worker"
)

// App holds all application dependencies and manages their lifecycle.
type App struct {
	cfg         *config.Config
	logger      *zap.SugaredLogger
	db          *sql.DB
	rdb         *redis.Client
	asynqClient *asynq.Client
	asynqServer *asynq.Server
	asynqMux    *asynq.ServeMux
	httpServer  *http.Server
}

// NewApp initializes all dependencies and returns a ready-to-run App.
func NewApp(cfg *config.Config, logger *zap.SugaredLogger) (*App, error) {
	app := &App{
		cfg:    cfg,
		logger: logger,
	}

	if err := app.initStorage(); err != nil {
		_ = app.close()
		return nil, err
	}

	if err := app.initServices(); err != nil {
		_ = app.close()
		return nil, err
	}

	return app, nil
}

// close releases database and Redis connections
func (app *App) close() error {
	var errs []error
	if app.rdb != nil {
		if err := app.rdb.Close(); err != nil {
			errs = append(errs, fmt.Errorf("redis close: %w", err))
		}
	}
	if app.db != nil {
		if err := app.db.Close(); err != nil {
			errs = append(errs, fmt.Errorf("db close: %w", err))
		}
	}
	return errors.Join(errs...)
}

func (app *App) initStorage() error {
	db, err := repository.NewPostgresDB(&app.cfg.Database)
	if err != nil {
		return fmt.Errorf("connect to Postgres: %w", err)
	}
	app.db = db

	if err := repository.RunMigrations(app.db); err != nil {
		return fmt.Errorf("run DB migrations: %w", err)
	}

	app.rdb = redis.NewClient(&redis.Options{
		Addr:     app.cfg.Redis.Addr,
		Password: app.cfg.Redis.Password,
	})
	if err := app.rdb.Ping(context.Background()).Err(); err != nil {
		return fmt.Errorf("connect to Redis: %w", err)
	}

	return nil
}

func (app *App) initServices() error {
	redisOpt := asynq.RedisClientOpt{Addr: app.cfg.Redis.Addr, Password: app.cfg.Redis.Password}
	app.asynqClient = asynq.NewClient(redisOpt)
	app.asynqServer = asynq.NewServer(
		redisOpt,
		asynq.Config{
			Concurrency: app.cfg.Worker.Concurrency,
		},
	)

	rateProvider, err := newRateProvider(app.cfg.External)
	if err != nil {
		return err
	}
	quoteRepo := repository.NewPostgresQuoteRepository(app.db)
	currencyValidator := service.NewValidator()
	quoteService := service.NewQuoteService(quoteRepo, rateProvider, currencyValidator, app.asynqClient, app.rdb, app.logger, app.cfg.Cache)

	app.asynqMux = asynq.NewServeMux()
	app.asynqMux.HandleFunc(service.TaskTypeUpdateQuote, worker.NewQuoteUpdateHandler(quoteService, app.logger))

	app.initHTTP(quoteService)
	return nil
}

func newRateProvider(cfg config.ExternalConfig) (provider.RatesProvider, error) {
	switch cfg.Provider {
	case "exchangerate_host":
		return provider.NewExchangeRateHostProvider(cfg.BaseURL, cfg.APIKey, cfg.Timeout), nil
	default:
		return nil, fmt.Errorf("unknown rate provider: %s", cfg.Provider)
	}
}

// Run starts the HTTP server and Asynq worker, blocking until the context is canceled.
func (app *App) Run(ctx context.Context) error {
	g, ctx := errgroup.WithContext(ctx)

	g.Go(func() error {
		app.logger.Infow("Starting Asynq worker server")
		if err := app.asynqServer.Start(app.asynqMux); err != nil {
			return fmt.Errorf("asynq worker failed to start: %w", err)
		}

		<-ctx.Done()
		return nil
	})

	g.Go(func() error {
		app.logger.Infow("HTTP server listening", "port", app.cfg.Server.Port)
		if err := app.httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			return fmt.Errorf("HTTP server error: %w", err)
		}
		return nil
	})

	// Graceful shutdown: triggered by context cancellation (signal or component failure).
	g.Go(func() error {
		<-ctx.Done()
		return app.shutdown()
	})

	return g.Wait()
}

// shutdown performs ordered teardown: HTTP server -> Asynq worker -> connections.
// This ensures in-flight tasks finish before the DB and Redis connections close.
func (app *App) shutdown() error {
	app.logger.Infow("Shutting down server...")

	var errs []error

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// 1. Stop accepting new HTTP requests, drain in-flight
	if err := app.httpServer.Shutdown(shutdownCtx); err != nil {
		app.logger.Errorw("HTTP server shutdown error", "error", err)
		errs = append(errs, fmt.Errorf("http shutdown: %w", err))
	}

	// 2. Drain in-flight Asynq tasks
	app.asynqServer.Shutdown()

	// 3. Close Asynq client
	if err := app.asynqClient.Close(); err != nil {
		app.logger.Errorw("Asynq client close error", "error", err)
		errs = append(errs, fmt.Errorf("asynq client close: %w", err))
	}

	// 4. Close Redis and database (after worker is done using them)
	if err := app.close(); err != nil {
		app.logger.Errorw("Connection cleanup errors", "error", err)
		errs = append(errs, err)
	}

	app.logger.Infow("Shutdown complete")
	return errors.Join(errs...)
}
