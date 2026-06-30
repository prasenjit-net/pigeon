# Pigeon — E2E Encrypted Chat: Implementation Plan

## Overview

Pigeon becomes a browser-based, end-to-end encrypted chat application. The server acts as a **Certificate Authority (CA)** and a **blind message router** — it signs user identities but can never read message content. All cryptography runs in the browser via the [Web Crypto API](https://developer.mozilla.org/en-US/docs/Web/API/SubtleCrypto).

---

## Cryptographic Design

### Two RSA Key Pairs Per User

Each user holds two separate key pairs, both generated in the browser and stored in `localStorage`:

| Key Pair | Algorithm | Purpose |
|---|---|---|
| **Signing key** | RSA-PSS 2048-bit | Proves identity; included in certificate |
| **Encryption key** | RSA-OAEP 2048-bit | Encrypts messages; recipient decrypts |

RSA does not allow the same key to be used for both PSS signing and OAEP encryption (they are separate algorithm families), so two key pairs are required. Both public keys are bundled into the user's certificate.

### Server CA Key

On first startup the server generates an RSA-PSS 2048-bit key pair and persists it to disk (`data/ca.key.json`). It is the trust anchor for all user certificates. Clients fetch the CA public key once and cache it.

### Certificate Format

The server issues a signed certificate binding the user's name and both public keys:

```json
{
  "cert": {
    "version": 1,
    "subject": {
      "name": "Alice",
      "id": "<hex of SHA-256(signingKey JWK)>",
      "signingKey": { "<RSA-PSS public key as JWK>" },
      "encryptionKey": { "<RSA-OAEP public key as JWK>" }
    },
    "issuedAt": 1751234567,
    "expiresAt": 1782770567,
    "issuer": "pigeon"
  },
  "signature": "<base64url of RSA-PSS signature over canonical JSON of cert>"
}
```

`id` is the stable user identifier used everywhere (roster, message routing, localStorage).

### Message Encryption

```
plaintext
  → encrypted with recipient's RSA-OAEP public key  (from their certificate)
  → base64url ciphertext sent through server
  → decrypted with recipient's RSA-OAEP private key  (never leaves browser)
```

For the MVP, one RSA-OAEP operation per message is acceptable (max ~245 bytes of plaintext per operation for 2048-bit keys). Longer messages will use hybrid encryption: a random AES-256-GCM key is RSA-OAEP-encrypted; the message body is AES-GCM-encrypted with that key. Both pieces are sent together.

---

## System Architecture

```
Browser A                        Go Server                        Browser B
─────────                        ─────────                        ─────────
KeyPair (localStorage)           CA Key (data/ca.key.json)        KeyPair (localStorage)
Certificate (localStorage)       Registry (in-memory)             Certificate (localStorage)
                                 Hub (WebSocket connections)

1. POST /api/register ──────────────────────────────>
                      <── signed certificate ─────────

2. GET /api/ca/public-key ──────────────────────────>
                      <── CA public key ──────────────

3. WS /ws hello+cert ───────────────────────────────>
                                 broadcast roster ──> (all connected clients)

4. send encrypted msg ──────────────────────────────>
                                 route to B ────────> decrypt + display
```

---

## Data Flow

### Registration (one-time per browser)

1. User enters their display name.
2. Browser generates signing key pair (RSA-PSS) and encryption key pair (RSA-OAEP) via `SubtleCrypto.generateKey`.
3. Both private keys are exported as JWK and saved to `localStorage`.
4. `POST /api/register` sends `{ name, signingPublicKey, encryptionPublicKey }`.
5. Server validates, builds certificate JSON, signs it with CA key, returns `SignedCertificate`.
6. Browser stores certificate in `localStorage`.

### Connection (every page load after registration)

1. Browser loads keys and certificate from `localStorage`.
2. Opens `WebSocket /ws`.
3. Sends `{ type: "hello", certificate: SignedCertificate }`.
4. Server verifies certificate signature, adds user to the hub.
5. Server broadcasts updated roster to all connected clients.

### Sending a Message

1. Sender looks up recipient's `encryptionKey` from the roster.
2. If message ≤ 190 bytes: RSA-OAEP encrypt directly.
3. If message > 190 bytes: generate ephemeral AES-256-GCM key → RSA-OAEP-encrypt the AES key → AES-GCM-encrypt the message body → send both.
4. Sends `{ type: "message", to: recipientId, encryptedPayload: "...", senderCert: SignedCertificate }` over WebSocket.
5. Server routes to recipient's WebSocket connection. Server never decrypts.

### Receiving a Message

1. Browser receives `{ type: "message", from: senderId, encryptedPayload, senderCert }`.
2. Verifies `senderCert` against cached CA public key.
3. Decrypts `encryptedPayload` with own RSA-OAEP private key.
4. Displays plaintext in chat window.

---

## WebSocket Message Protocol

All messages are JSON over a single WebSocket connection per client.

### Client → Server

```typescript
// Announce presence after connection
{ type: "hello", certificate: SignedCertificate }

// Send an encrypted message
{
  type: "message",
  to: string,               // recipient's subject.id
  encryptedPayload: string, // see Payload Format below
  senderCert: SignedCertificate
}
```

### Server → Client

```typescript
// Full roster on connect or any change
{ type: "roster", users: OnlineUser[] }

// Single user joined (optimization to avoid full roster re-send)
{ type: "user_joined", user: OnlineUser }

// Single user left
{ type: "user_left", userId: string }

// Routed incoming message
{
  type: "message",
  from: string,
  encryptedPayload: string,
  senderCert: SignedCertificate
}

// Error from server
{ type: "error", code: string, message: string }
```

### Encrypted Payload Format

```json
{
  "v": 1,
  "mode": "rsa-oaep" | "hybrid",
  "encryptedKey": "<base64url, only present for hybrid>",
  "iv": "<base64url, only present for hybrid>",
  "ciphertext": "<base64url>"
}
```

### OnlineUser Shape

```typescript
interface OnlineUser {
  id: string          // hex SHA-256 of signing key
  name: string
  signingKey: JsonWebKey
  encryptionKey: JsonWebKey
  certificate: SignedCertificate
}
```

---

## localStorage Schema

```
pigeon.identity.signingPrivateKey    — JWK (RSA-PSS private)
pigeon.identity.signingPublicKey     — JWK (RSA-PSS public)
pigeon.identity.encryptionPrivateKey — JWK (RSA-OAEP private)
pigeon.identity.encryptionPublicKey  — JWK (RSA-OAEP public)
pigeon.identity.certificate          — JSON string (SignedCertificate)
pigeon.identity.name                 — display name string
pigeon.ca.publicKey                  — JWK (server CA public key, cached)
```

---

## HTTP API

| Method | Path | Auth | Description |
|---|---|---|---|
| GET | `/api/ca/public-key` | None | Returns server CA public key (JWK) |
| POST | `/api/register` | None | Accepts public keys + name, returns SignedCertificate |
| GET | `/api/health` | None | Existing health check |

`/ws` is the WebSocket upgrade endpoint (no separate HTTP handler needed).

---

## Backend Package Structure

```
internal/
  ca/
    ca.go          — CA key lifecycle: generate, persist to data/ca.key.json, load
    sign.go        — build and sign user certificates
    verify.go      — verify a SignedCertificate (used in WebSocket handler)
  registry/
    registry.go    — in-memory store: userId → SignedCertificate (registered users)
  hub/
    hub.go         — WebSocket hub: connected clients, broadcast, route-to-one
    client.go      — per-connection read/write pump
    messages.go    — message type definitions (Go structs matching WS protocol)
  api/
    handler.go     — existing + new: CAPublicKey, Register handlers
    router.go      — existing + mount /ws
```

The hub runs as a single goroutine owning the `clients` map; client goroutines communicate with it via channels (standard Go WebSocket hub pattern — no mutexes on the map).

---

## Frontend Package Structure

```
ui/src/
  crypto/
    keys.ts          — generateKeyPair, exportJWK, importJWK for both key types
    encrypt.ts       — encryptMessage, decryptMessage (RSA-OAEP + hybrid AES-GCM)
    certificate.ts   — verifyCertificate against CA public key
    fingerprint.ts   — SHA-256 fingerprint of a public key JWK → hex id
  store/
    identity.ts      — read/write identity from localStorage; clear (logout)
  hooks/
    useIdentity.ts   — React state over identity store; triggers onboarding if absent
    useWebSocket.ts  — manages WS connection, reconnect, dispatches inbound messages
    useRoster.ts     — maintains OnlineUser[] from roster/user_joined/user_left events
    useChat.ts       — per-conversation message list; encrypt on send, decrypt on receive
  pages/
    OnboardingPage.tsx — multi-step: name input → key generation → registration → done
    ChatPage.tsx       — split-pane: user list left, active conversation right
  components/
    onboarding/
      NameStep.tsx      — name input form
      KeygenStep.tsx    — progress display during async key generation
      RegisterStep.tsx  — calls /api/register, shows result
    chat/
      UserList.tsx      — online users sidebar
      ConversationPane.tsx
      MessageBubble.tsx
      MessageInput.tsx
```

---

## User-Facing Flow

```
First visit
  └─ OnboardingPage
       1. Enter name
       2. "Generate Keys" button → SubtleCrypto.generateKey × 2 (async, ~1s)
       3. Keys saved to localStorage
       4. POST /api/register → receive certificate
       5. Certificate saved to localStorage
       └─> redirect to ChatPage

Returning visit (keys in localStorage)
  └─ ChatPage
       1. Load identity from localStorage
       2. Open WebSocket, send hello
       3. Receive roster → display online users
       4. Click a user → open conversation pane
       5. Type + send → encrypt → WebSocket send
       6. Inbound WS message → decrypt → append to conversation

Logout / reset
  └─ Clear all pigeon.* keys from localStorage
     Close WebSocket
     Redirect to OnboardingPage
```

---

## Implementation Phases

### Phase 1 — Server CA & Registration API

**Goal:** Server can issue signed certificates; client can register.

- `internal/ca/ca.go` — generate RSA-PSS 2048-bit CA key on first start, persist/load from `data/ca.key.json`
- `internal/ca/sign.go` — `Sign(name, signingPub, encryptionPub) SignedCertificate`
- `internal/registry/registry.go` — thread-safe in-memory map of `userId → SignedCertificate`
- New HTTP handlers: `GET /api/ca/public-key`, `POST /api/register`
- Wire into `internal/api/router.go`

Deliverable: `curl /api/register` returns a valid signed certificate.

---

### Phase 2 — WebSocket Hub & Presence

**Goal:** Clients can connect, announce themselves, and see the live roster.

- Add `github.com/gorilla/websocket` dependency
- `internal/hub/hub.go` — channel-based hub goroutine; `Register`, `Unregister`, `Broadcast`, `SendTo`
- `internal/hub/client.go` — per-connection read pump and write pump goroutines
- `internal/hub/messages.go` — Go structs for all WS message types
- Handle `hello` message: verify certificate signature (using CA public key), add to hub, broadcast roster
- Handle disconnect: remove from hub, broadcast updated roster
- Mount WebSocket at `/ws` in `server.go`

Deliverable: Two browser tabs can connect and each sees the other in the roster.

---

### Phase 3 — Message Routing

**Goal:** Server routes encrypted payloads between clients without decrypting.

- In the hub's read pump: handle `type: "message"` — validate `to` field exists in hub, forward raw payload to recipient's write channel
- If recipient is not connected: return `{ type: "error", code: "offline" }` to sender (no persistence)
- Log routing events (sender id → recipient id, payload size) for debugging; never log payload content

Deliverable: Encrypted payload sent from tab A arrives at tab B unchanged.

---

### Phase 4 — Frontend Identity & Crypto Layer

**Goal:** All browser-side crypto utilities, fully tested in isolation.

- `ui/src/crypto/keys.ts` — `generateSigningKeyPair()`, `generateEncryptionKeyPair()`, `exportJWK()`, `importSigningPublicKey()`, `importEncryptionPublicKey()`
- `ui/src/crypto/fingerprint.ts` — `fingerprintKey(jwk): Promise<string>` (hex SHA-256 of canonical JWK)
- `ui/src/crypto/encrypt.ts` — `encryptMessage(plaintext, encKeyJWK)`, `decryptMessage(payload, encPrivKeyJWK)`; hybrid mode for messages > 190 bytes
- `ui/src/crypto/certificate.ts` — `verifyCertificate(signed, caPublicKeyJWK): Promise<boolean>`
- `ui/src/store/identity.ts` — `saveIdentity()`, `loadIdentity()`, `clearIdentity()`

Deliverable: Vitest unit tests pass for encrypt→decrypt round-trips and certificate verification.

---

### Phase 5 — Onboarding UI

**Goal:** First-time users can generate keys and register.

- `OnboardingPage.tsx` with three steps rendered sequentially:
  1. `NameStep` — validates name is non-empty, ≤ 64 chars
  2. `KeygenStep` — triggers Phase 4 functions; shows spinner; saves to localStorage
  3. `RegisterStep` — calls `POST /api/register`; on success stores certificate; redirects
- `useIdentity` hook — on mount checks localStorage; if complete redirects straight to ChatPage
- `App.tsx` routing: add `<Route path="/onboarding" element={<OnboardingPage />} />`; root `/` redirects based on identity presence

Deliverable: A new user can complete registration end-to-end in the browser.

---

### Phase 6 — Chat UI

**Goal:** Full chat experience.

- `useWebSocket` hook — opens `ws://host/ws`, sends `hello` on open, handles reconnect with exponential backoff (max 30s), dispatches inbound messages to a callback
- `useRoster` hook — listens for `roster` / `user_joined` / `user_left` events; maintains `Map<userId, OnlineUser>`
- `useChat(recipientId)` hook — maintains `Message[]` for one conversation; on send calls `encryptMessage` then sends WS message; on receive calls `decryptMessage` then appends
- `ChatPage` layout: fixed left sidebar (UserList), main area (ConversationPane or empty-state)
- `UserList` — shows name + online indicator; click to open conversation
- `ConversationPane` — scrollable message list + MessageInput
- `MessageBubble` — right-aligned (sent) / left-aligned (received); timestamp; decryption error state
- `MessageInput` — textarea, Enter to send (Shift+Enter for newline)
- Remove template pages (Dashboard, Examples, Settings) and replace Layout nav with chat-specific nav

Deliverable: Two users can exchange encrypted messages in real time.

---

### Phase 7 — Hardening & Polish

- Certificate expiry check on both client and server (`expiresAt` validation)
- Server rejects `hello` with an expired or tampered certificate
- Client re-registers automatically when certificate is within 7 days of expiry
- Reconnection: replay `hello` after WebSocket reconnect
- Unread message badge on UserList entries
- Auto-scroll to bottom on new message (unless user has scrolled up)
- "User is offline" feedback when send returns `error: offline`
- CA public key pinning: once fetched and cached in `localStorage`, changes are rejected with a warning
- `make test` coverage: Go unit tests for CA signing and certificate verification; Vitest for crypto utilities

---

## Dependencies to Add

### Go

```
github.com/gorilla/websocket v1.5.x
```

(Alternative: `nhooyr.io/websocket` — lighter, context-aware; either works with chi)

### Node / Frontend

No new npm dependencies required. The Web Crypto API (`window.crypto.subtle`) is built into all modern browsers. AES-GCM and RSA-OAEP are both available natively.

---

## Things Deliberately Out of Scope (MVP)

- **Message persistence** — messages exist only in memory for the duration of a session. If a recipient is offline the message is dropped.
- **Group chat** — one-to-one only.
- **Message ordering guarantees** — TCP order is sufficient for MVP.
- **Push notifications** — browser tab must be open.
- **Key rotation** — users regenerate by clearing localStorage (which creates a new identity).
- **Forward secrecy** — RSA-OAEP does not provide forward secrecy. A future iteration could layer Signal-style double-ratchet on top.
- **Server-side user persistence** — the registry is in-memory; restart loses all registered users.
