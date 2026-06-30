package api

import (
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/prasenjit-net/pigeon/internal/config"
	"github.com/prasenjit-net/pigeon/internal/registry"
	"github.com/prasenjit-net/pigeon/internal/version"
)

func TestHealthEndpoint(t *testing.T) {
	reg := registry.New()
	// CA is nil — Health doesn't use it.
	router := NewRouter(config.Default(), slog.New(slog.NewTextHandler(io.Discard, nil)), version.Current(), nil, reg)
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	res := httptest.NewRecorder()

	router.ServeHTTP(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", res.Code)
	}
	if !strings.Contains(res.Body.String(), `"status":"ok"`) {
		t.Fatalf("expected ok payload, got %s", res.Body.String())
	}
}
