# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Commands

```bash
# Initial setup
make install-deps       # go mod download + npm install in ui/

# Development
make dev                # Go server only (proxy non-/api requests to Vite)
make dev-ui             # Vite dev server on :5173
make dev-all            # backend + frontend together

# Build
make build              # build UI then compile Go binary → build/pigeon
make build-ui           # npm run build in ui/
make build-go           # compile Go binary only

# Test & lint
make test               # go test ./...
make lint               # go vet
make lint-ui            # eslint in ui/
make fmt                # go fmt

# Run a single Go test
go test ./internal/api/... -run TestHealthEndpoint -v
```

## Architecture

### Request flow

```
HTTP request
  → chi router (server.go)
    → /api/*  → api.NewRouter  → Handler methods
    → /*       → SPA handler (embeds ui/dist) or Vite reverse proxy (--dev mode)
```

### Go backend

- **`main.go`** — entry point; passes the embedded `ui/dist` FS to `app.Execute`.
- **`ui_embed.go`** — `//go:embed all:ui/dist` directive; `make build-ui` must run before `make build-go`.
- **`cmd/app/`** — Cobra commands (`serve`, `init`, `version`). `root.go` wires Viper config; `serve.go` starts the HTTP server with graceful shutdown.
- **`internal/config/`** — `Config` struct loaded from `config.yaml` → `.env`/`.env.local` → `APP_*` env vars → CLI flags (in that priority order). `config.Default()` is the source of truth for defaults.
- **`internal/server/`** — builds the chi router, mounts `/api`, and serves the SPA or proxies to Vite when `--dev` is passed. `spaHandler` falls back to `index.html` for unknown paths (client-side routing).
- **`internal/api/`** — `NewRouter` + `Handler`. Add new API endpoints here.
- **`internal/version/`** — version string injected at build time via `-ldflags` in the Makefile.
- **`internal/logging/`** — `slog`-based logger configured from `LoggingConfig`.

### Dev vs production UI

In `--dev` mode the server reverse-proxies all non-`/api` requests to the Vite dev server at `APP_UI_DEV_PROXY_URL` (default `:5173`). In production mode the binary serves the embedded `ui/dist` build directly — no separate Node process.

### React frontend (`ui/`)

- Vite + React 18 + TypeScript + Tailwind CSS.
- `ui/src/services/api.ts` — all API calls via React Query.
- `ui/src/pages/` — page-level components (`DashboardPage`, `ExamplesPage`, `SettingsPage`).
- `ui/src/components/Layout.tsx` — fixed left sidebar shell shared by all pages.

### Configuration precedence

`config.Default()` < `config.yaml` < `.env` / `.env.local` < `APP_*` env vars < CLI flags.

All env vars use the `APP_` prefix with `_` replacing `.` (e.g. `APP_SERVER_PORT=9090`).
