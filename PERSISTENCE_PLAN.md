# Pigeon — Message Persistence Plan

## Overview

Messages gain two persistence layers:

| Layer | Scope | Lifetime |
|---|---|---|
| **Server memory queue** | Messages to offline recipients | Until recipient next connects, or server restart |
| **Browser localStorage** | All messages, both directions | Permanent (until user resets) |

The server still never decrypts anything. Queued messages are stored as opaque encrypted payloads and delivered as-is on reconnect.

---

## Core Design Decisions

### Message IDs

The server assigns a **UUIDv4** to every message at the moment it is received. This ID:
- Is included in live deliveries (`DeliveryMsg.id`)
- Is included in queued deliveries (`PendingMessagesMsg.messages[].id`)
- Is echoed back to the sender in an ack (`MessageAckMsg.serverMsgId`)
- Is used by the client to **deduplicate** messages (prevents double-display if a message arrives both live and in the next pending batch)

The client also generates a short **client-side ID** (`clientMsgId`) for optimistic display while waiting for the server ack.

### Acks to the sender

Every sent message now receives an explicit ack from the server telling the sender whether it was delivered live or queued. The ack carries the server ID so the client can promote the optimistically-displayed message to a confirmed one.

```
Sender           Server           Recipient
  |  --message-->   |                |
  |                 |  (online) ------message+id-->
  |  <--ack(delivered,id)--          |
  |                 |
  |  --message-->   |
  |                 |  (offline) → queue(id)
  |  <--ack(queued,id)--
```

### Pending message delivery on connect

When a client sends `hello`, the server:
1. Verifies the certificate.
2. Checks the queue for any messages addressed to that user ID.
3. Sends a single `pending_messages` frame with all queued messages (in chronological order).
4. Clears their queue entry.

The client receives this before the roster, so unread badges are populated before any UI is shown.

### Client deduplication

A message is considered a duplicate if its `serverMsgId` already exists in the conversation store. Duplicates are silently dropped. This protects against edge cases where a recipient is connected, goes offline mid-delivery, and reconnects before the server's write pump times out.

---

## Server-Side Changes

### New package: `internal/queue/`

```go
// PersistedMessage is stored server-side for offline recipients.
type PersistedMessage struct {
    ID               string               `json:"id"`
    From             string               `json:"from"`
    To               string               `json:"to"`
    EncryptedPayload string               `json:"encryptedPayload"`
    SenderCert       ca.SignedCertificate `json:"senderCert"`
    Timestamp        int64                `json:"timestamp"` // Unix milliseconds
}

// Queue is an in-memory store of pending messages keyed by recipient ID.
// It is goroutine-safe. The Hub holds one instance.
type Queue struct {
    mu         sync.Mutex
    pending    map[string][]PersistedMessage
    maxPerUser int // default 200; oldest messages are dropped when exceeded
}

func (q *Queue) Push(recipientID string, msg PersistedMessage)
func (q *Queue) Drain(recipientID string) []PersistedMessage // returns and clears
func (q *Queue) Len(recipientID string) int
```

`Hub` gains a `queue *queue.Queue` field, created in `hub.New()`.

### Updated WebSocket message types (`internal/hub/messages.go`)

**New outbound type constants:**
```go
const (
    TypeMessageAck      = "message_ack"
    TypePendingMessages = "pending_messages"
)
```

**Updated `SendMsg`** — adds `ClientMsgID` for ack matching:
```go
type SendMsg struct {
    Type             string               `json:"type"`
    ClientMsgID      string               `json:"clientMsgId"`  // NEW
    To               string               `json:"to"`
    EncryptedPayload string               `json:"encryptedPayload"`
    SenderCert       ca.SignedCertificate `json:"senderCert"`
}
```

**Updated `DeliveryMsg`** — adds server ID and timestamp:
```go
type DeliveryMsg struct {
    Type             string               `json:"type"`
    ID               string               `json:"id"`        // NEW: server-assigned UUID
    From             string               `json:"from"`
    EncryptedPayload string               `json:"encryptedPayload"`
    SenderCert       ca.SignedCertificate `json:"senderCert"`
    Timestamp        int64                `json:"timestamp"` // NEW: Unix ms
}
```

**New `MessageAckMsg`** — sent back to the sender:
```go
type MessageAckMsg struct {
    Type        string `json:"type"`        // "message_ack"
    ClientMsgID string `json:"clientMsgId"` // echoed from SendMsg
    ServerMsgID string `json:"serverMsgId"` // server-assigned UUID
    Status      string `json:"status"`      // "delivered" | "queued"
    Timestamp   int64  `json:"timestamp"`
}
```

**New `PendingMessagesMsg`** — sent to a client on connect:
```go
type PendingMessagesMsg struct {
    Type     string                    `json:"type"` // "pending_messages"
    Messages []queue.PersistedMessage  `json:"messages"`
}
```

### Updated `client.go` — `handleSend`

Current: if recipient offline → send `error(offline)`.  
New: if recipient offline → queue the message, send ack with `status: "queued"`.

```
handleSend:
  1. Parse SendMsg (now includes clientMsgId)
  2. Generate serverMsgId = uuid.New()
  3. timestamp = time.Now().UnixMilli()
  4. Build DeliveryMsg{ID: serverMsgId, Timestamp: timestamp, ...}
  5. if hub.Route(msg.To, deliveryBytes):
       send MessageAckMsg{status: "delivered", ...} to sender
     else:
       hub.Queue().Push(msg.To, PersistedMessage{...})
       send MessageAckMsg{status: "queued", ...} to sender
```

### Updated `client.go` — `handleHello`

After registering with the hub:
```
handleHello (after c.hub.Register(c)):
  pending := c.hub.Queue().Drain(c.userID)
  if len(pending) > 0:
    send PendingMessagesMsg{Messages: pending}
```

The `pending_messages` frame is sent before the roster so the client can populate unread counts before rendering the sidebar.

### UUID dependency

Add `github.com/google/uuid` for `uuid.NewString()`. Alternatively generate a random hex string using `crypto/rand` with no new dependency.

---

## Client-Side Changes

### New store: `ui/src/store/messages.ts`

Persists conversation history in localStorage. Each conversation is stored under a separate key to avoid deserialising everything on open.

```typescript
// localStorage key: pigeon.conv.<otherUserId>
interface ConversationStore {
  messages: PersistedMsg[]
  lastReadAt: number  // Unix ms; messages after this timestamp are "unread"
}

interface PersistedMsg {
  id: string           // server-assigned UUID (used for dedup)
  clientId?: string    // temp client ID, present until ack received
  direction: 'sent' | 'received'
  text: string         // decrypted plaintext
  timestamp: number    // Unix ms
  status: 'delivered' | 'queued' | 'failed'
}

// API
function loadConversation(otherUserId: string): ConversationStore
function saveConversation(otherUserId: string, store: ConversationStore): void
function appendMessage(otherUserId: string, msg: PersistedMsg): void
function updateMessageStatus(otherUserId: string, clientId: string, update: Partial<PersistedMsg>): void
function markRead(otherUserId: string): void
function unreadCount(otherUserId: string): number  // messages.filter(m => m.timestamp > lastReadAt && m.direction === 'received').length
function clearConversation(otherUserId: string): void
```

Max messages per conversation: **500** (oldest dropped when exceeded on write).

### Updated `useChat`

**On mount** (when `recipientId` becomes non-empty): load messages from localStorage and set as initial state.

**On send**:
1. Generate `clientId = nanoid()`.
2. Append optimistic message `{ clientId, direction: 'sent', status: 'pending', text, timestamp: Date.now() }` to state and localStorage.
3. Encrypt and send `{ type: "message", clientMsgId: clientId, ... }`.

**On `message_ack`** (new case in `receiveMessage`):
```typescript
if msg.type === 'message_ack' && msg.clientMsgId matches a pending sent message:
  update that message: { id: msg.serverMsgId, status: msg.status, timestamp: msg.timestamp }
  persist update to localStorage
```

**On `message` (live delivery)**:
1. Check localStorage for dedup by `msg.id`.
2. Decrypt.
3. Append to state and localStorage with `status: 'delivered'`.

**On conversation open** (when `recipientId` is selected in ChatPage): call `markRead(recipientId)`.

### New hook: `usePendingMessages`

Handles the `pending_messages` event which arrives outside of any single conversation context (the messages may span multiple senders).

```typescript
interface PendingHandler {
  handlePendingMessages: (msg: WsMessage) => Promise<void>
  unreadCounts: Map<string, number>  // senderId → count of new messages
}

function usePendingMessages(ownEncPrivKeyJWK: JsonWebKey): PendingHandler
```

**On `pending_messages`**:
```
For each message in msg.messages:
  1. Check dedup: if id already in localStorage for that conversation, skip
  2. Decrypt encryptedPayload with own private key
  3. appendMessage(message.from, { id, text, direction: 'received', status: 'delivered', timestamp })
  4. Track new message count per sender

Rebuild unreadCounts from all conversations' unreadCount()
```

**On any `message` (live, received)**:
- Increment unread count for sender if that conversation is not currently open.

### Updated `ChatPage`

```typescript
const { handlePendingMessages, unreadCounts } = usePendingMessages(identity.encryptionPrivateKey)

// Wire into WebSocket message handler:
const handleMessage = (msg) => {
  rosterHandler(msg)
  if (msg.type === 'pending_messages') handlePendingMessages(msg)
  else chat.receiveMessage(msg)

  // Live message from someone other than selected — increment unread
  if (msg.type === 'message' && msg.from !== selected?.id) {
    incrementUnread(msg.from as string)
  }
}

// When user selects a conversation:
function handleSelect(user: OnlineUser) {
  setSelected(user)
  markRead(user.id)         // localStorage
  clearUnread(user.id)      // in-memory badge state
}
```

Pass `unreadCounts` to `UserList`.

### Updated `UserList`

Add unread badge on each user entry:

```tsx
// If unreadCounts.get(u.id) > 0, show a count bubble:
<span className="ml-auto flex h-5 w-5 items-center justify-center rounded-full bg-indigo-600 text-white text-xs font-bold">
  {count}
</span>
```

---

## localStorage Key Schema (additions)

```
pigeon.conv.<userId>    — JSON ConversationStore (messages + lastReadAt)
```

The existing `pigeon.identity.*` and `pigeon.ca.publicKey` keys are unchanged.

---

## Message Lifecycle State Machine

```
[sender]
  clientId generated → status: 'pending'
  ack received (delivered) → status: 'delivered', id: serverMsgId
  ack received (queued)    → status: 'queued',    id: serverMsgId
  no ack / WS error        → status: 'failed'

[recipient, live]
  message arrives → status: 'delivered'

[recipient, pending batch on connect]
  pending_messages → status: 'delivered' (already server-confirmed)
```

---

## Implementation Phases

### Phase 1 — Server queue and message IDs

- Add `crypto/rand` UUID generator in `internal/queue/uuid.go` (avoid new dep)
- Create `internal/queue/queue.go` with `PersistedMessage` and `Queue`
- Update `internal/hub/messages.go`: new types/constants, add `ID`/`Timestamp`/`ClientMsgID` fields
- Update `internal/hub/hub.go`: add `queue` field, expose `Queue()` accessor
- Update `internal/hub/client.go`:
  - `handleSend`: generate ID, route-or-queue, send ack either way
  - `handleHello`: drain queue after registration, send `pending_messages` if non-empty

Deliverable: `curl /ws` (via websocat) — send a message to an offline user; reconnect as that user; receive `pending_messages`.

---

### Phase 2 — Client message store

- Create `ui/src/store/messages.ts` with full CRUD API
- Cap at 500 messages per conversation (trim oldest on `appendMessage`)

Deliverable: Unit-testable store functions, no UI changes yet.

---

### Phase 3 — Update `useChat` for persistence

- Load from `messages.ts` on mount (when `recipientId` changes)
- Send path: optimistic append with `clientId`, then `message_ack` updates status/id
- Receive path: dedup by `id`, save to store
- Call `markRead` when conversation is opened

Deliverable: Refreshing the page restores conversation history.

---

### Phase 4 — Pending messages + unread counts

- Create `usePendingMessages` hook
- Wire into `ChatPage` message handler
- `ChatPage` manages `unreadCounts: Map<string, number>` in state
- `clearUnread` on conversation select

Deliverable: Open two tabs; go offline; receive messages; reconnect — badge shows on the sender in sidebar.

---

### Phase 5 — Unread badge UI

- Update `UserList` to accept `unreadCounts: Map<string, number>`
- Render count bubble (capped display at `99+`)
- Clear badge when conversation is selected

---

## Capacity and Limits

| Resource | Limit | Behaviour when exceeded |
|---|---|---|
| Server queue per recipient | 200 messages | Oldest messages dropped (FIFO eviction) |
| localStorage per conversation | 500 messages | Oldest messages trimmed on write |
| `pending_messages` batch size | ≤ 200 (queue max) | Sent in a single frame |

---

## What Stays Out of Scope

- **Server-side persistence across restarts** — queue is in-memory; restart loses pending messages
- **Read receipts** — no "seen" signal sent to sender
- **Message ordering across senders** — ordering within a single conversation is preserved; cross-conversation ordering is not a concern here
- **localStorage size limits** — browser quota (~5 MB) not actively managed; 500-message cap per conversation is the practical guard
