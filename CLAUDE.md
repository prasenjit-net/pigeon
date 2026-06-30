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

## What This Is

Pigeon is an **end-to-end encrypted browser chat application**. The server acts as a Certificate Authority and a blind message router — it signs user identities but never reads message content. All cryptography runs in the browser via the Web Crypto API.

## Architecture

### Request flow

```
HTTP request
  → chi router (server.go)
    → /api/*  → api.NewRouter  → Handler methods
    → /ws     → hub.ServeWS   → WebSocket hub
    → /*      → SPA handler (embeds ui/dist) or Vite reverse proxy (--dev mode)
```

### Go backend packages

- **`internal/ca/`** — RSA-PSS CA key pair. Generated once, persisted to `data/ca.key.pem`. `ca.Issue()` builds and signs user certificates. `ca.VerifyCertificate()` checks signature and expiry.
- **`internal/registry/`** — Thread-safe in-memory map of `userId → SignedCertificate`. Populated on `/api/register`.
- **`internal/hub/`** — WebSocket hub and per-connection client. Single hub goroutine owns the `clients` map; mutations go through channels. `client.go` has read/write pumps per connection. Handles `hello` (verify cert, add to roster), `message` (route encrypted payload to recipient), and `disconnect` (broadcast user_left).
- **`internal/api/`** — HTTP handlers: `GET /api/ca/public-key`, `POST /api/register`, `GET /api/health`, `GET /api/meta`.
- **`internal/config/`** — `Config` struct. `DataDir` (default `"data"`) is where the CA key is stored.
- **`internal/server/`** — Assembles the chi router, creates CA, registry, and hub, mounts `/api` and `/ws`.

### Certificate format

```json
{
  "cert": {
    "version": 1,
    "subject": { "name": "Alice", "id": "<hex SHA-256 of signing key>",
                 "signingKey": {JWK}, "encryptionKey": {JWK} },
    "issuedAt": 1751234567, "expiresAt": 1782770567, "issuer": "pigeon"
  },
  "signature": "<base64url RSA-PSS over JSON of cert>"
}
```

### WebSocket protocol

Client→server: `{ type: "hello", certificate }` and `{ type: "message", to, encryptedPayload, senderCert }`.  
Server→client: `{ type: "roster", users[] }`, `{ type: "user_joined", user }`, `{ type: "user_left", id }`, `{ type: "message", from, encryptedPayload, senderCert }`, `{ type: "error", code, message }`.

### Frontend packages

- **`ui/src/crypto/`** — Web Crypto API wrappers. `keys.ts`: generate/import/export RSA-PSS and RSA-OAEP key pairs. `encrypt.ts`: `encryptMessage`/`decryptMessage` (RSA-OAEP direct for ≤190 bytes; hybrid AES-256-GCM for longer). `certificate.ts`: `verifyCertificate`. `fingerprint.ts`: `fingerprintJWK` → hex SHA-256 of key material.
- **`ui/src/store/identity.ts`** — Read/write identity (both key pairs + certificate) in `localStorage` under `pigeon.*` keys.
- **`ui/src/hooks/`** — `useIdentity`: loads identity from localStorage. `useWebSocket`: opens `/ws`, sends `hello`, reconnects with backoff. `useRoster`: maintains online user map from WebSocket events. `useChat`: per-conversation encrypted message state; calls `encryptMessage` on send and `decryptMessage` on receive.
- **`ui/src/pages/OnboardingPage.tsx`** — Three-step flow: name → keygen → register. On completion redirects to `/chat`.
- **`ui/src/pages/ChatPage.tsx`** — Wires `useWebSocket`, `useRoster`, `useChat`. Left sidebar (UserList), right pane (ConversationPane or empty state).

### User identity flow

1. `OnboardingPage` generates two RSA key pairs via `SubtleCrypto.generateKey`.
2. `POST /api/register` sends both public keys + name; server returns a `SignedCertificate`.
3. Identity (private keys + certificate) saved to `localStorage`.
4. On every page load `useIdentity` checks localStorage; redirects to `/onboarding` if absent.
5. On `/chat`, `useWebSocket` opens `/ws` and sends `hello` with the stored certificate.

### Encrypted message path

```
Sender types text
  → encryptMessage(text, recipient.encryptionKey)   [RSA-OAEP or hybrid AES-GCM]
  → WS send { type:"message", to, encryptedPayload }
  → server routes raw bytes to recipient's WS connection (never decrypts)
  → recipient receives { type:"message", from, encryptedPayload }
  → decryptMessage(encryptedPayload, ownEncryptionPrivateKey)
  → display plaintext
```

### Dev vs production UI

`make dev` / `go run . serve --dev` proxies non-`/api` non-`/ws` requests to the Vite dev server (`APP_UI_DEV_PROXY_URL`, default `:5173`). Production binary serves the embedded `ui/dist` build directly.

### Configuration precedence

`config.Default()` < `config.yaml` < `.env`/`.env.local` < `APP_*` env vars < CLI flags. Key env vars: `APP_SERVER_PORT`, `APP_LOGGING_LEVEL`, `APP_DATA_DIR`, `APP_UI_DEV_PROXY_URL`.
