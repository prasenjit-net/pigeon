package queue

import (
	"sync"
	"time"

	"github.com/prasenjit-net/pigeon/internal/ca"
)

const defaultMaxPerUser = 200

// PersistedMessage is a message held server-side for an offline recipient.
// The EncryptedPayload is opaque to the server — it is stored and forwarded
// as-is without decryption.
// GroupID is set for group messages; empty for 1:1 messages.
type PersistedMessage struct {
	ID               string               `json:"id"`
	From             string               `json:"from"`
	To               string               `json:"to"`
	GroupID          string               `json:"groupId,omitempty"`
	EncryptedPayload string               `json:"encryptedPayload"`
	SenderCert       ca.SignedCertificate `json:"senderCert"`
	Timestamp        int64                `json:"timestamp"` // Unix milliseconds
}

// NewMessage builds a PersistedMessage with a server-assigned ID and
// current timestamp.
func NewMessage(from, to, encryptedPayload string, senderCert ca.SignedCertificate) PersistedMessage {
	return PersistedMessage{
		ID:               NewID(),
		From:             from,
		To:               to,
		EncryptedPayload: encryptedPayload,
		SenderCert:       senderCert,
		Timestamp:        time.Now().UnixMilli(),
	}
}

// Queue is a goroutine-safe in-memory store of pending messages keyed by
// recipient ID. Oldest messages are dropped when a recipient's queue exceeds
// maxPerUser.
type Queue struct {
	mu         sync.Mutex
	pending    map[string][]PersistedMessage
	maxPerUser int
}

// New returns a Queue with the default capacity per user.
func New() *Queue {
	return &Queue{
		pending:    make(map[string][]PersistedMessage),
		maxPerUser: defaultMaxPerUser,
	}
}

// Push adds a message to the recipient's queue. If the queue is at capacity
// the oldest message is evicted to make room.
func (q *Queue) Push(msg PersistedMessage) {
	q.mu.Lock()
	defer q.mu.Unlock()

	msgs := q.pending[msg.To]
	if len(msgs) >= q.maxPerUser {
		msgs = msgs[1:] // evict oldest
	}
	q.pending[msg.To] = append(msgs, msg)
}

// Drain returns all queued messages for recipientID and removes them from the
// queue. Returns nil if there are no pending messages.
func (q *Queue) Drain(recipientID string) []PersistedMessage {
	q.mu.Lock()
	defer q.mu.Unlock()

	msgs := q.pending[recipientID]
	if len(msgs) == 0 {
		return nil
	}
	delete(q.pending, recipientID)
	return msgs
}

// Len returns the number of pending messages for recipientID.
func (q *Queue) Len(recipientID string) int {
	q.mu.Lock()
	defer q.mu.Unlock()
	return len(q.pending[recipientID])
}
