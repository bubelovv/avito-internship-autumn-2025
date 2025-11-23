package httpserver

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/bubelovv/avito-internship-autumn-2025/internal/service"
	"go.uber.org/zap"
)

type Server struct {
	srv    *http.Server
	logger *zap.Logger
}

func New(port string, logger *zap.Logger, svc *service.Service) *Server {
	httpSrv := &http.Server{
		Addr:              ":" + port,
		Handler:           newRouter(logger, svc),
		ReadTimeout:       15 * time.Second,
		ReadHeaderTimeout: 5 * time.Second,
		WriteTimeout:      15 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	return &Server{
		srv:    httpSrv,
		logger: logger,
	}
}

func (s *Server) Start() error {
	s.logger.Info("http server listening", zap.String("addr", s.srv.Addr))
	if err := s.srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}

func (s *Server) Stop(ctx context.Context) error {
	s.logger.Info("http server stopping")
	return s.srv.Shutdown(ctx)
}
