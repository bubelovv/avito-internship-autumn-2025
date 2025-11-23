package httpserver

import (
	"net/http"
	"time"

	"github.com/bubelovv/avito-internship-autumn-2025/internal/service"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"go.uber.org/zap"
)

func newRouter(logger *zap.Logger, svc *service.Service) http.Handler {
	h := &handler{
		svc:    svc,
		logger: logger,
	}

	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Recoverer)
	r.Use(middleware.Timeout(30 * time.Second))
	r.Use(zapRequestLogger(logger))

	r.Get("/health", h.handleHealth)

	r.Route("/team", func(r chi.Router) {
		r.Post("/add", h.handleTeamAdd)
		r.Get("/get", h.handleTeamGet)
	})

	r.Route("/users", func(r chi.Router) {
		r.Post("/setIsActive", h.handleUserSetActive)
		r.Get("/getReview", h.handleUserGetReview)
	})

	r.Route("/pullRequest", func(r chi.Router) {
		r.Post("/create", h.handlePullRequestCreate)
		r.Post("/merge", h.handlePullRequestMerge)
		r.Post("/reassign", h.handlePullRequestReassign)
	})

	return r
}

func zapRequestLogger(logger *zap.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			ww := middleware.NewWrapResponseWriter(w, r.ProtoMajor)

			next.ServeHTTP(ww, r)

			logger.Info(
				"http request",
				zap.String("method", r.Method),
				zap.String("path", r.URL.Path),
				zap.Int("status", ww.Status()),
				zap.Duration("duration", time.Since(start)),
				zap.String("request_id", middleware.GetReqID(r.Context())),
			)
		})
	}
}
