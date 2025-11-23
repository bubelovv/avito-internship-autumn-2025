package app

import (
	"context"
	"os/signal"
	"syscall"

	"github.com/bubelovv/avito-internship-autumn-2025/internal/config"
	"github.com/bubelovv/avito-internship-autumn-2025/internal/httpserver"
	"github.com/bubelovv/avito-internship-autumn-2025/internal/migrations"
	"github.com/bubelovv/avito-internship-autumn-2025/internal/repository"
	"github.com/bubelovv/avito-internship-autumn-2025/internal/service"
	"github.com/bubelovv/avito-internship-autumn-2025/internal/storage/postgres"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"
)

type App struct {
	cfg        config.Config
	logger     *zap.Logger
	httpServer *httpserver.Server
	db         *pgxpool.Pool
	repo       *repository.Repository
	svc        *service.Service
}

func New(ctx context.Context, cfg config.Config, logger *zap.Logger) (*App, error) {
	db, err := postgres.New(ctx, cfg.DatabaseURL, logger)
	if err != nil {
		return nil, err
	}

	if err := migrations.Run(ctx, cfg.DatabaseURL, logger); err != nil {
		db.Close()
		return nil, err
	}

	repo := repository.New(db)
	svc := service.New(repo)
	server := httpserver.New(cfg.HTTPPort, logger, svc)

	return &App{
		cfg:        cfg,
		logger:     logger,
		httpServer: server,
		db:         db,
		repo:       repo,
		svc:        svc,
	}, nil
}

func (a *App) Run(ctx context.Context) error {
	defer a.db.Close()
	ctx, stop := signal.NotifyContext(ctx, syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	errCh := make(chan error, 1)
	go func() {
		errCh <- a.httpServer.Start()
	}()

	select {
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), a.cfg.ShutdownTimeout)
		defer cancel()

		if err := a.httpServer.Stop(shutdownCtx); err != nil {
			return err
		}

		return <-errCh
	case err := <-errCh:
		return err
	}
}
