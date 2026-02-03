package main

import (
	"context"
	"log"
	"os/signal"
	"syscall"

	"go.uber.org/zap"

	_ "quoteservice/internal/api/docs"
	"quoteservice/internal/config"
)

func main() {
	cfg, err := config.LoadConfig()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	zapLogger, err := zap.NewProduction()
	if err != nil {
		log.Fatalf("Failed to init logger: %v", err)
	}
	defer func() { _ = zapLogger.Sync() }()
	sugar := zapLogger.Sugar()

	sugar.Infow("Starting Currency Quotes Service", "port", cfg.Server.Port)

	app, err := NewApp(cfg, sugar)
	if err != nil {
		sugar.Fatalw("Failed to initialize app", "error", err)
	}

	ctx, stop := signal.NotifyContext(context.Background(),
		syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	if err := app.Run(ctx); err != nil {
		sugar.Fatalw("Application error", "error", err)
	}
}
