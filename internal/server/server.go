package server

import (
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"gorm.io/gorm"

	"github.com/prasenjit-net/pigeon/internal/api"
	"github.com/prasenjit-net/pigeon/internal/ca"
	"github.com/prasenjit-net/pigeon/internal/config"
	"github.com/prasenjit-net/pigeon/internal/db"
	"github.com/prasenjit-net/pigeon/internal/hub"
	"github.com/prasenjit-net/pigeon/internal/registry"
	"github.com/prasenjit-net/pigeon/internal/version"
)

type Options struct {
	DevMode bool
	UIFS    fs.FS
}

type App struct {
	cfg     config.Config
	logger  *slog.Logger
	build   version.Info
	options Options
	hub     *hub.Hub
	ca      *ca.CA
	reg     registry.Registry
	gdb     *gorm.DB
}

func New(cfg config.Config, logger *slog.Logger, build version.Info, options Options) (*App, error) {
	var (
		authority *ca.CA
		reg       registry.Registry
		gdb       *gorm.DB
		err       error
	)

	if cfg.Database.DSN != "" {
		logger.Info("db: connecting to PostgreSQL")
		gdb, err = db.Open(cfg.Database.DSN)
		if err != nil {
			return nil, fmt.Errorf("server: open db: %w", err)
		}
	} else {
		if err = os.MkdirAll(cfg.DataDir, 0o700); err != nil {
			return nil, fmt.Errorf("server: create data dir: %w", err)
		}
		sqlitePath := filepath.Join(cfg.DataDir, "pigeon.db")
		logger.Info("db: using SQLite", "path", sqlitePath)
		gdb, err = db.OpenSQLite(sqlitePath)
		if err != nil {
			return nil, fmt.Errorf("server: open sqlite: %w", err)
		}
	}

	caKeyStore := db.NewGORMKeyStore(gdb)
	migrateFileCAKey(cfg.DataDir, caKeyStore, logger)
	authority, err = ca.NewWithStore(caKeyStore, logger)
	if err != nil {
		return nil, fmt.Errorf("server: init CA: %w", err)
	}

	reg = db.NewGORMRegistry(gdb)
	connStore := db.NewGORMConnectionStore(gdb)
	groupStore := db.NewGORMGroupStore(gdb)

	h := hub.New(authority, reg, connStore, groupStore, logger)
	go h.Run()

	return &App{
		cfg:     cfg,
		logger:  logger,
		build:   build,
		options: options,
		hub:     h,
		ca:      authority,
		reg:     reg,
		gdb:     gdb,
	}, nil
}

// Close releases the database connection pool. Call this during graceful shutdown.
func (a *App) Close() {
	if a.gdb != nil {
		if err := db.Close(a.gdb); err != nil {
			a.logger.Warn("db: close error", "error", err)
		}
	}
}

func (a *App) Handler() http.Handler {
	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Recoverer)
	r.Use(middleware.Heartbeat("/livez"))
	r.Use(requestLogger(a.logger))

	r.Mount("/api", api.NewRouter(a.cfg, a.logger, a.build, a.ca, a.reg))

	// WebSocket endpoint — no timeout middleware wrapping it.
	r.Get("/ws", func(w http.ResponseWriter, r *http.Request) {
		hub.ServeWS(a.hub, w, r, a.logger)
	})

	if a.options.DevMode && strings.TrimSpace(a.cfg.UI.DevProxyURL) != "" {
		r.Handle("/*", newDevProxy(a.cfg.UI.DevProxyURL, a.logger))
		return r
	}

	distFS, err := fs.Sub(a.options.UIFS, "ui/dist")
	if err != nil {
		a.logger.Error("embedded ui not available", "error", err)
		r.Get("/*", func(w http.ResponseWriter, r *http.Request) {
			http.Error(w, "embedded UI missing; run `make build-ui` before building the binary", http.StatusServiceUnavailable)
		})
		return r
	}

	r.Handle("/*", newSPAHandler(distFS))
	return r
}

func newDevProxy(rawURL string, logger *slog.Logger) http.Handler {
	target, err := url.Parse(rawURL)
	if err != nil {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			http.Error(w, fmt.Sprintf("invalid UI dev proxy URL: %v", err), http.StatusInternalServerError)
		})
	}

	proxy := httputil.NewSingleHostReverseProxy(target)
	proxy.ErrorHandler = func(w http.ResponseWriter, r *http.Request, proxyErr error) {
		logger.Error("vite proxy error", "error", proxyErr)
		http.Error(w, "Vite dev server is unavailable. Start it with `make dev-ui` or `make dev-all`.", http.StatusBadGateway)
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		proxy.ServeHTTP(w, r)
	})
}

type spaHandler struct {
	fsys       fs.FS
	fileServer http.Handler
}

func newSPAHandler(fsys fs.FS) http.Handler {
	return &spaHandler{
		fsys:       fsys,
		fileServer: http.FileServer(http.FS(fsys)),
	}
}

func (h *spaHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	cleanPath := strings.TrimPrefix(path.Clean(r.URL.Path), "/")
	if cleanPath == "." || cleanPath == "" {
		cleanPath = "index.html"
	}

	if fileExists(h.fsys, cleanPath) {
		h.fileServer.ServeHTTP(w, r)
		return
	}

	indexReq := r.Clone(r.Context())
	indexReq.URL.Path = "/index.html"
	h.fileServer.ServeHTTP(w, indexReq)
}

func fileExists(fsys fs.FS, name string) bool {
	file, err := fsys.Open(name)
	if err != nil {
		return false
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		return false
	}
	return !info.IsDir()
}

// migrateFileCAKey moves an existing data/ca.key.pem into the DB store the
// first time the server starts after the GORM migration. It is a no-op when
// the DB already has a key or the file does not exist.
func migrateFileCAKey(dataDir string, store ca.KeyStore, logger *slog.Logger) {
	if _, err := store.Load(); !errors.Is(err, ca.ErrKeyNotFound) {
		return // DB already has a key (or unexpected error — let NewWithStore handle it)
	}
	oldPath := filepath.Join(dataDir, "ca.key.pem")
	pem, err := os.ReadFile(oldPath)
	if err != nil {
		return // file doesn't exist — fresh install, nothing to migrate
	}
	if err := store.Save(pem); err != nil {
		logger.Warn("ca: failed to migrate key file to DB", "error", err)
		return
	}
	_ = os.Remove(oldPath)
	logger.Info("ca: migrated ca.key.pem to database")
}

func requestLogger(logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			ww := middleware.NewWrapResponseWriter(w, r.ProtoMajor)
			next.ServeHTTP(ww, r)
			logger.Info("request",
				"method", r.Method,
				"path", r.URL.Path,
				"status", ww.Status(),
				"duration", time.Since(start).String(),
				"request_id", middleware.GetReqID(r.Context()),
			)
		})
	}
}
