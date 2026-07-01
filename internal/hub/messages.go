package hub

import (
	"github.com/prasenjit-net/pigeon/internal/ca"
	"github.com/prasenjit-net/pigeon/internal/queue"
)

// Inbound message types (client → server).
const (
	TypeHello          = "hello"
	TypeMessage        = "message"
	TypeConnectRequest = "connect_request"
	TypeConnectRespond = "connect_respond"
)

// Outbound message types (server → client).
const (
	TypeRoster             = "roster"
	TypeUserJoined         = "user_joined"
	TypeUserLeft           = "user_left"
	TypeError              = "error"
	TypeMessageAck         = "message_ack"
	TypePendingMessages    = "pending_messages"
	TypeConnectRequestAck  = "connect_request_ack"
	TypePendingConnects    = "pending_connects"
)

// InboundEnvelope is the top-level wrapper for all client→server messages.
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
	ClientMsgID      string               `json:"clientMsgId"`
	To               string               `json:"to"`
	EncryptedPayload string               `json:"encryptedPayload"`
	SenderCert       ca.SignedCertificate `json:"senderCert"`
}

// ConnectRequestMsg is sent by the client to initiate a connection.
type ConnectRequestMsg struct {
	Type     string `json:"type"`
	TargetID string `json:"targetId"`
}

// ConnectRespondMsg is sent by the client to accept or reject a connection.
type ConnectRespondMsg struct {
	Type         string `json:"type"`
	ConnectionID string `json:"connectionId"`
	Accept       bool   `json:"accept"`
}

// ContactUser is a peer summary included in connection-related messages.
// SigningKey and EncryptionKey are only populated after a connection is accepted.
type ContactUser struct {
	ID            string         `json:"id"`
	Handle        string         `json:"handle"`
	Name          string         `json:"name"`
	SigningKey     map[string]any `json:"signingKey,omitempty"`
	EncryptionKey map[string]any `json:"encryptionKey,omitempty"`
}

// RosterUser is a contact in the roster — includes both online and offline
// accepted connections with their public keys.
type RosterUser struct {
	ID            string         `json:"id"`
	Handle        string         `json:"handle"`
	Name          string         `json:"name"`
	SigningKey     map[string]any `json:"signingKey"`
	EncryptionKey map[string]any `json:"encryptionKey"`
	Online        bool           `json:"online"`
}

// RosterMsg is sent to a client on connect with all accepted contacts.
type RosterMsg struct {
	Type  string       `json:"type"`
	Users []RosterUser `json:"users"`
}

// UserJoinedMsg is sent to connections when an accepted peer comes online.
type UserJoinedMsg struct {
	Type string     `json:"type"`
	User RosterUser `json:"user"`
}

// UserLeftMsg is sent to connections when a peer disconnects.
type UserLeftMsg struct {
	Type string `json:"type"`
	ID   string `json:"id"`
}

// DeliveryMsg is forwarded to the recipient for a live message delivery.
type DeliveryMsg struct {
	Type             string               `json:"type"`
	ID               string               `json:"id"`
	From             string               `json:"from"`
	EncryptedPayload string               `json:"encryptedPayload"`
	SenderCert       ca.SignedCertificate `json:"senderCert"`
	Timestamp        int64                `json:"timestamp"`
}

// MessageAckMsg is sent back to the sender after every message.
type MessageAckMsg struct {
	Type        string `json:"type"`
	ClientMsgID string `json:"clientMsgId"`
	ServerMsgID string `json:"serverMsgId"`
	Status      string `json:"status"` // "delivered" | "queued"
	Timestamp   int64  `json:"timestamp"`
}

// PendingMessagesMsg is sent to a client on connect if they have queued messages.
type PendingMessagesMsg struct {
	Type     string                   `json:"type"`
	Messages []queue.PersistedMessage `json:"messages"`
}

// ConnectRequestAckMsg acknowledges a sent connection request.
type ConnectRequestAckMsg struct {
	Type         string `json:"type"`
	ConnectionID string `json:"connectionId"`
	Status       string `json:"status"` // "pending"
}

// IncomingConnectReqMsg is forwarded to the recipient of a connection request.
type IncomingConnectReqMsg struct {
	Type         string      `json:"type"`
	ConnectionID string      `json:"connectionId"`
	Requester    ContactUser `json:"requester"`
}

// IncomingConnectRespMsg is forwarded to the requester when a response arrives.
type IncomingConnectRespMsg struct {
	Type         string      `json:"type"`
	ConnectionID string      `json:"connectionId"`
	Accept       bool        `json:"accept"`
	Peer         ContactUser `json:"peer"`
}

// PendingConnectsMsg is sent on hello to deliver missed connection events.
type PendingConnectsMsg struct {
	Type      string                   `json:"type"`
	Inbound   []IncomingConnectReqMsg  `json:"inbound"`
	Responses []IncomingConnectRespMsg `json:"responses"`
}

// ErrorMsg is sent to the client when an error occurs.
type ErrorMsg struct {
	Type    string `json:"type"`
	Code    string `json:"code"`
	Message string `json:"message"`
}

// ── Group message types ──────────────────────────────────────────────────────

// Group inbound types (client → server).
const (
	TypeGroupCreate  = "group_create"
	TypeGroupInvite  = "group_invite"
	TypeGroupRespond = "group_respond"
	TypeGroupMessage = "group_message"
)

// Group outbound types (server → client).
const (
	TypeGroupCreateAck      = "group_create_ack"
	TypeGroupInviteAck      = "group_invite_ack"
	TypeGroupRespondNotify  = "group_respond"       // reuses "group_respond" wire string
	TypeGroupRoster         = "group_roster"
	TypeGroupMessageAck     = "group_message_ack"
	TypeGroupUserJoined     = "group_user_joined"
	TypeGroupUserLeft       = "group_user_left"
	TypePendingGroupConnects = "pending_group_connects"
)

// GroupCreateMsg is sent by a client to create a new group.
type GroupCreateMsg struct {
	Type   string `json:"type"`
	Handle string `json:"handle"` // without #
	Name   string `json:"name"`
}

// GroupInviteMsg is sent by the group owner to invite a user by ID.
type GroupInviteMsg struct {
	Type     string `json:"type"`
	GroupID  string `json:"groupId"`
	TargetID string `json:"targetId"`
}

// GroupRespondMsg is sent by an invitee to accept or reject an invite.
type GroupRespondMsg struct {
	Type     string `json:"type"`
	MemberID string `json:"memberId"` // GroupMember.ID
	Accept   bool   `json:"accept"`
}

// GroupSendMsg is sent by a member to broadcast an E2E-encrypted group message.
type GroupSendMsg struct {
	Type             string               `json:"type"`
	ClientMsgID      string               `json:"clientMsgId"`
	GroupID          string               `json:"groupId"`
	EncryptedPayload string               `json:"encryptedPayload"`
	SenderCert       ca.SignedCertificate `json:"senderCert"`
}

// GroupSummary is a compact group descriptor used in acks and roster entries.
type GroupSummary struct {
	ID      string `json:"id"`
	Handle  string `json:"handle"` // without #
	Name    string `json:"name"`
	OwnerID string `json:"ownerId"`
}

// GroupRosterMember is a member entry in the group_roster, including the
// encryption public key needed for E2E group message encryption.
type GroupRosterMember struct {
	ID            string         `json:"id"`
	Handle        string         `json:"handle"`
	Name          string         `json:"name"`
	EncryptionKey map[string]any `json:"encryptionKey"`
	Online        bool           `json:"online"`
}

// GroupRosterEntry is one group in the group_roster batch.
type GroupRosterEntry struct {
	Group   GroupSummary        `json:"group"`
	Members []GroupRosterMember `json:"members"`
}

// GroupRosterMsg is sent to a client on hello — all accepted groups with members.
type GroupRosterMsg struct {
	Type   string             `json:"type"` // "group_roster"
	Groups []GroupRosterEntry `json:"groups"`
}

// GroupCreateAckMsg is sent back to the creator after a successful group_create.
type GroupCreateAckMsg struct {
	Type  string       `json:"type"` // "group_create_ack"
	Group GroupSummary `json:"group"`
}

// OutboundGroupInviteMsg is forwarded to the invitee (wire type "group_invite").
type OutboundGroupInviteMsg struct {
	Type     string       `json:"type"` // "group_invite"
	MemberID string       `json:"memberId"`
	Group    GroupSummary `json:"group"`
	Inviter  ContactUser  `json:"inviter"`
}

// GroupInviteAckMsg is sent back to the inviter after the invite is persisted.
type GroupInviteAckMsg struct {
	Type     string      `json:"type"` // "group_invite_ack"
	MemberID string      `json:"memberId"`
	GroupID  string      `json:"groupId"`
	Invitee  ContactUser `json:"invitee"`
}

// GroupRespondNotifyMsg is sent to the inviter when the invitee responds.
type GroupRespondNotifyMsg struct {
	Type     string      `json:"type"` // "group_respond"
	MemberID string      `json:"memberId"`
	GroupID  string      `json:"groupId"`
	Accept   bool        `json:"accept"`
	Member   ContactUser `json:"member"` // the invitee who responded
}

// GroupDeliveryMsg is the live-delivery and pending-delivery shape for a group message.
type GroupDeliveryMsg struct {
	Type             string               `json:"type"` // "group_message"
	ID               string               `json:"id"`
	GroupID          string               `json:"groupId"`
	From             string               `json:"from"`
	EncryptedPayload string               `json:"encryptedPayload"`
	SenderCert       ca.SignedCertificate `json:"senderCert"`
	Timestamp        int64                `json:"timestamp"`
}

// GroupMessageAckMsg is sent back to the sender after a group message.
type GroupMessageAckMsg struct {
	Type        string `json:"type"` // "group_message_ack"
	ClientMsgID string `json:"clientMsgId"`
	ServerMsgID string `json:"serverMsgId"`
	GroupID     string `json:"groupId"`
	Status      string `json:"status"` // "delivered" | "partial" | "queued"
	Timestamp   int64  `json:"timestamp"`
}

// GroupUserJoinedMsg notifies group members when a member comes online.
type GroupUserJoinedMsg struct {
	Type    string            `json:"type"` // "group_user_joined"
	GroupID string            `json:"groupId"`
	Member  GroupRosterMember `json:"member"`
}

// GroupUserLeftMsg notifies group members when a member goes offline.
type GroupUserLeftMsg struct {
	Type    string `json:"type"` // "group_user_left"
	GroupID string `json:"groupId"`
	UserID  string `json:"userId"`
}

// PendingGroupConnectsMsg is sent on hello for undelivered group invite events.
type PendingGroupConnectsMsg struct {
	Type      string                   `json:"type"` // "pending_group_connects"
	Invites   []OutboundGroupInviteMsg `json:"invites"`
	Responses []GroupRespondNotifyMsg  `json:"responses"`
}
