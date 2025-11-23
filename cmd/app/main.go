package main

import (
	"context"
	"log"

	"github.com/bubelovv/avito-internship-autumn-2025/internal/app"
	"github.com/bubelovv/avito-internship-autumn-2025/internal/config"
	"github.com/bubelovv/avito-internship-autumn-2025/internal/logger"
	"go.uber.org/zap"
)

func main() {
	ctx := context.Background()

	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("config: %v", err)
	}

	zapLogger, err := logger.New(cfg.LogLevel)
	if err != nil {
		log.Fatalf("logger: %v", err)
	}
	defer zapLogger.Sync()

	application, err := app.New(ctx, cfg, zapLogger)
	if err != nil {
		zapLogger.Fatal("init app failed", zap.Error(err))
	}

	if err := application.Run(ctx); err != nil {
		zapLogger.Fatal("app stopped", zap.Error(err))
	}
}
