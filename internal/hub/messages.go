package hub

import "github.com/prasenjit-net/pigeon/internal/ca"

// Inbound message types (client → server).
const (
	TypeHello   = "hello"
	TypeMessage = "message"
)

// Outbound message types (server → client).
const (
	TypeRoster     = "roster"
	TypeUserJoined = "user_joined"
	TypeUserLeft   = "user_left"
	TypeError      = "error"
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

// DeliveryMsg is what the server forwards to the recipient.
type DeliveryMsg struct {
	Type             string               `json:"type"`
	From             string               `json:"from"`
	EncryptedPayload string               `json:"encryptedPayload"`
	SenderCert       ca.SignedCertificate `json:"senderCert"`
}

// ErrorMsg is sent to the client when an error occurs.
type ErrorMsg struct {
	Type    string `json:"type"`
	Code    string `json:"code"`
	Message string `json:"message"`
}
