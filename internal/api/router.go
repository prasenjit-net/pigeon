package api

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"github.com/prasenjit-net/pigeon/internal/ca"
	"github.com/prasenjit-net/pigeon/internal/config"
	"github.com/prasenjit-net/pigeon/internal/registry"
	"github.com/prasenjit-net/pigeon/internal/version"
)

func NewRouter(cfg config.Config, logger *slog.Logger, build version.Info, authority *ca.CA, reg registry.Registry) http.Handler {
	r := chi.NewRouter()
	r.Use(middleware.Timeout(30 * time.Second))

	h := NewHandler(cfg, build, authority, reg)

	r.Get("/health", h.Health)
	r.Get("/meta", h.Meta)
	r.Get("/ca/public-key", h.CAPublicKey)
	r.Post("/register", h.Register)
	r.Get("/users/search", h.SearchUser)

	r.Get("/", func(w http.ResponseWriter, r *http.Request) {
		respondJSON(w, http.StatusOK, map[string]any{
			"service": cfg.App.Name,
			"message": "API ready",
			"routes":  []string{"/api/health", "/api/meta", "/api/ca/public-key", "/api/register", "/api/users/search"},
		})
	})

	logger.Debug("api router initialized")
	return r
}
