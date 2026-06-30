package hub

import (
	"github.com/prasenjit-net/pigeon/internal/ca"
	"github.com/prasenjit-net/pigeon/internal/queue"
)

// Inbound message types (client → server).
const (
	TypeHello   = "hello"
	TypeMessage = "message"
)

// Outbound message types (server → client).
const (
	TypeRoster          = "roster"
	TypeUserJoined      = "user_joined"
	TypeUserLeft        = "user_left"
	TypeError           = "error"
	TypeMessageAck      = "message_ack"
	TypePendingMessages = "pending_messages"
)

// InboundEnvelope is the top-level wrapper for all client→server messages.
// The Type field is read first; the full payload is re-decoded per type.
type InboundEnvelope struct {
	Type string `json:"type"`
}

// HelloMsg is sent by the client immediately after connecting.
type HelloMsg struct {
	Type        string               `json:"type"`
	Certificate ca.SignedCertificate `json:"certificate"`
}

// SendMsg is sent by the client to route an encrypted message to another user.
type SendMsg struct {
	Type             string               `json:"type"`
	ClientMsgID      string               `json:"clientMsgId"`      // client-assigned temp ID for ack matching
	To               string               `json:"to"`               // recipient subject.id
	EncryptedPayload string               `json:"encryptedPayload"` // base64url ciphertext
	SenderCert       ca.SignedCertificate `json:"senderCert"`
}

// OnlineUser is the user representation included in roster messages.
type OnlineUser struct {
	ID            string         `json:"id"`
	Name          string         `json:"name"`
	SigningKey     map[string]any `json:"signingKey"`
	EncryptionKey map[string]any `json:"encryptionKey"`
}

// RosterMsg is sent to a client when the set of online users changes.
type RosterMsg struct {
	Type  string       `json:"type"`
	Users []OnlineUser `json:"users"`
}

// UserJoinedMsg is sent when a new user connects.
type UserJoinedMsg struct {
	Type string     `json:"type"`
	User OnlineUser `json:"user"`
}

// UserLeftMsg is sent when a user disconnects.
type UserLeftMsg struct {
	Type string `json:"type"`
	ID   string `json:"id"`
}

// DeliveryMsg is forwarded to the recipient for a live (online) delivery.
type DeliveryMsg struct {
	Type             string               `json:"type"`
	ID               string               `json:"id"`               // server-assigned UUID
	From             string               `json:"from"`
	EncryptedPayload string               `json:"encryptedPayload"`
	SenderCert       ca.SignedCertificate `json:"senderCert"`
	Timestamp        int64                `json:"timestamp"` // Unix milliseconds
}

// MessageAckMsg is sent back to the sender after every message.
// Status is "delivered" if the recipient was online, "queued" if offline.
type MessageAckMsg struct {
	Type        string `json:"type"`
	ClientMsgID string `json:"clientMsgId"` // echoed from SendMsg
	ServerMsgID string `json:"serverMsgId"` // server-assigned UUID
	Status      string `json:"status"`      // "delivered" | "queued"
	Timestamp   int64  `json:"timestamp"`
}

// PendingMessagesMsg is sent to a client on connect if they have queued messages.
type PendingMessagesMsg struct {
	Type     string                    `json:"type"`
	Messages []queue.PersistedMessage  `json:"messages"`
}

// ErrorMsg is sent to the client when an error occurs.
type ErrorMsg struct {
	Type    string `json:"type"`
	Code    string `json:"code"`
	Message string `json:"message"`
}
