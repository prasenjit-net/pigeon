package hub

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"regexp"
	"sync"
	"time"

	"github.com/prasenjit-net/pigeon/internal/ca"
	"github.com/prasenjit-net/pigeon/internal/connections"
	"github.com/prasenjit-net/pigeon/internal/groups"
	"github.com/prasenjit-net/pigeon/internal/queue"
	"github.com/prasenjit-net/pigeon/internal/registry"
)

var groupHandleRe = regexp.MustCompile(`^[a-z][a-z0-9_]{2,31}$`)

// Hub maintains the set of active WebSocket clients and routes messages
// between them. A single goroutine owns the clients map; all mutations go
// through channels to avoid mutexes on the hot path.
type Hub struct {
	mu      sync.RWMutex
	clients map[string]*Client // keyed by subject.id

	register   chan *Client
	unregister chan *Client
	route      chan routeReq

	authority  *ca.CA
	reg        registry.Registry
	connStore  connections.Store
	groupStore groups.Store
	queue      *queue.Queue
	logger     *slog.Logger
}

type routeReq struct {
	to      string
	payload []byte
}

func New(authority *ca.CA, reg registry.Registry, connStore connections.Store, groupStore groups.Store, logger *slog.Logger) *Hub {
	return &Hub{
		clients:    make(map[string]*Client),
		register:   make(chan *Client, 16),
		unregister: make(chan *Client, 16),
		route:      make(chan routeReq, 256),
		authority:  authority,
		reg:        reg,
		connStore:  connStore,
		groupStore: groupStore,
		queue:      queue.New(),
		logger:     logger,
	}
}

// Run starts the hub event loop. Call it in a goroutine.
func (h *Hub) Run() {
	for {
		select {
		case c := <-h.register:
			h.mu.Lock()
			h.clients[c.userID] = c
			h.mu.Unlock()

			h.logger.Info("hub: user connected", "id", c.userID, "name", c.name)
			h.deliverPending(c)
			h.deliverPendingConnects(c)
			h.deliverPendingGroupConnects(c)
			h.broadcastUserJoined(c)
			h.broadcastGroupJoined(c)
			h.sendRoster(c)
			h.sendGroupRoster(c)

		case c := <-h.unregister:
			h.mu.Lock()
			if existing, ok := h.clients[c.userID]; ok && existing == c {
				delete(h.clients, c.userID)
			}
			h.mu.Unlock()

			h.logger.Info("hub: user disconnected", "id", c.userID, "name", c.name)
			h.broadcastUserLeft(c.userID)
			h.broadcastGroupLeft(c.userID)

		case req := <-h.route:
			h.mu.RLock()
			target, ok := h.clients[req.to]
			h.mu.RUnlock()
			if ok {
				select {
				case target.send <- req.payload:
				default:
					h.logger.Warn("hub: recipient send buffer full, dropping", "to", req.to)
				}
			}
		}
	}
}

// Register queues a client for addition to the hub.
func (h *Hub) Register(c *Client) { h.register <- c }

// Unregister queues a client for removal from the hub.
func (h *Hub) Unregister(c *Client) { h.unregister <- c }

// Route delivers a raw JSON payload to an online recipient.
func (h *Hub) Route(to string, payload []byte) bool {
	h.mu.RLock()
	target, ok := h.clients[to]
	h.mu.RUnlock()
	if !ok {
		return false
	}
	select {
	case target.send <- payload:
		return true
	default:
		h.logger.Warn("hub: recipient send buffer full, dropping", "to", to)
		return false
	}
}

// Queue returns the message queue so client.go can push offline messages.
func (h *Hub) Queue() *queue.Queue { return h.queue }

// CA returns the hub's CA so clients can verify certificates.
func (h *Hub) CA() *ca.CA { return h.authority }

// deliverPending sends any queued messages to a newly connected client.
func (h *Hub) deliverPending(c *Client) {
	msgs := h.queue.Drain(c.userID)
	if len(msgs) == 0 {
		return
	}
	payload := mustMarshal(PendingMessagesMsg{Type: TypePendingMessages, Messages: msgs})
	select {
	case c.send <- payload:
		h.logger.Info("hub: delivered pending messages", "id", c.userID, "count", len(msgs))
	default:
		h.logger.Warn("hub: could not deliver pending messages, buffer full", "id", c.userID)
	}
}

// deliverPendingConnects sends undelivered connection events on hello.
func (h *Hub) deliverPendingConnects(c *Client) {
	inboundReqs, err := h.connStore.ListUndeliveredInbound(c.userID)
	if err != nil {
		h.logger.Error("hub: list undelivered inbound connections", "id", c.userID, "error", err)
		return
	}
	responses, err := h.connStore.ListUndeliveredResponses(c.userID)
	if err != nil {
		h.logger.Error("hub: list undelivered connection responses", "id", c.userID, "error", err)
		return
	}

	inbound := make([]IncomingConnectReqMsg, 0, len(inboundReqs))
	for _, req := range inboundReqs {
		cert, lookupErr := h.reg.Get(req.RequesterID)
		if lookupErr != nil {
			continue
		}
		inbound = append(inbound, IncomingConnectReqMsg{
			Type:         TypeConnectRequest,
			ConnectionID: req.ID,
			Requester: ContactUser{
				ID:     cert.Cert.Subject.ID,
				Handle: cert.Cert.Subject.Handle,
				Name:   cert.Cert.Subject.Name,
			},
		})
		if markErr := h.connStore.MarkRecipientNotified(req.ID); markErr != nil {
			h.logger.Warn("hub: mark recipient notified", "conn_id", req.ID, "error", markErr)
		}
	}

	resps := make([]IncomingConnectRespMsg, 0, len(responses))
	for _, req := range responses {
		cert, lookupErr := h.reg.Get(req.RecipientID)
		if lookupErr != nil {
			continue
		}
		peer := ContactUser{ID: cert.Cert.Subject.ID, Handle: cert.Cert.Subject.Handle, Name: cert.Cert.Subject.Name}
		if req.Status == connections.Accepted {
			peer.SigningKey = cert.Cert.Subject.SigningKey
			peer.EncryptionKey = cert.Cert.Subject.EncryptionKey
		}
		resps = append(resps, IncomingConnectRespMsg{
			Type:         TypeConnectRespond,
			ConnectionID: req.ID,
			Accept:       req.Status == connections.Accepted,
			Peer:         peer,
		})
		if markErr := h.connStore.MarkRequesterNotified(req.ID); markErr != nil {
			h.logger.Warn("hub: mark requester notified", "conn_id", req.ID, "error", markErr)
		}
	}

	if len(inbound) == 0 && len(resps) == 0 {
		return
	}
	payload := mustMarshal(PendingConnectsMsg{
		Type:      TypePendingConnects,
		Inbound:   inbound,
		Responses: resps,
	})
	select {
	case c.send <- payload:
	default:
		h.logger.Warn("hub: could not deliver pending connects, buffer full", "id", c.userID)
	}
}

// sendRoster sends all accepted connections (online + offline) to the client.
func (h *Hub) sendRoster(c *Client) {
	accepted, err := h.connStore.ListAccepted(c.userID)
	if err != nil {
		h.logger.Error("hub: list accepted connections for roster", "id", c.userID, "error", err)
		return
	}

	users := make([]RosterUser, 0, len(accepted))
	h.mu.RLock()
	onlineClients := h.clients
	h.mu.RUnlock()

	for _, conn := range accepted {
		peerID := conn.RequesterID
		if peerID == c.userID {
			peerID = conn.RecipientID
		}
		cert, lookupErr := h.reg.Get(peerID)
		if lookupErr != nil {
			continue
		}
		_, online := onlineClients[peerID]
		users = append(users, RosterUser{
			ID:            cert.Cert.Subject.ID,
			Handle:        cert.Cert.Subject.Handle,
			Name:          cert.Cert.Subject.Name,
			SigningKey:     cert.Cert.Subject.SigningKey,
			EncryptionKey: cert.Cert.Subject.EncryptionKey,
			Online:        online,
		})
	}

	msg := mustMarshal(RosterMsg{Type: TypeRoster, Users: users})
	select {
	case c.send <- msg:
	default:
	}
}

// broadcastUserJoined notifies only accepted connections of the joining user.
func (h *Hub) broadcastUserJoined(c *Client) {
	accepted, err := h.connStore.ListAccepted(c.userID)
	if err != nil {
		h.logger.Error("hub: list accepted connections for join broadcast", "id", c.userID, "error", err)
		return
	}
	peerIDs := acceptedPeerSet(c.userID, accepted)
	if len(peerIDs) == 0 {
		return
	}

	ru := RosterUser{
		ID:            c.userID,
		Handle:        c.cert.Cert.Subject.Handle,
		Name:          c.name,
		SigningKey:     c.signingKey,
		EncryptionKey: c.encryptionKey,
		Online:        true,
	}
	msg := mustMarshal(UserJoinedMsg{Type: TypeUserJoined, User: ru})

	h.mu.RLock()
	defer h.mu.RUnlock()
	for id, cl := range h.clients {
		if id == c.userID || !peerIDs[id] {
			continue
		}
		select {
		case cl.send <- msg:
		default:
		}
	}
}

// broadcastUserLeft notifies only accepted connections of the leaving user.
func (h *Hub) broadcastUserLeft(userID string) {
	accepted, err := h.connStore.ListAccepted(userID)
	if err != nil {
		h.logger.Error("hub: list accepted connections for leave broadcast", "id", userID, "error", err)
		return
	}
	peerIDs := acceptedPeerSet(userID, accepted)

	msg := mustMarshal(UserLeftMsg{Type: TypeUserLeft, ID: userID})
	h.mu.RLock()
	defer h.mu.RUnlock()
	for id, cl := range h.clients {
		if !peerIDs[id] {
			continue
		}
		select {
		case cl.send <- msg:
		default:
		}
	}
}

// handleConnectRequest processes an incoming connection request from a client.
func (h *Hub) handleConnectRequest(from *Client, raw []byte) {
	var msg ConnectRequestMsg
	if err := json.Unmarshal(raw, &msg); err != nil || msg.TargetID == "" {
		from.sendError("bad_request", "invalid connect_request payload")
		return
	}

	// Validate target exists.
	targetCert, err := h.reg.Get(msg.TargetID)
	if err != nil {
		from.sendError("user_not_found", fmt.Sprintf("user %q not found", msg.TargetID))
		return
	}

	// Persist the connection request.
	conn, err := h.connStore.Request(from.userID, msg.TargetID)
	if err != nil {
		if err == connections.ErrAlreadyExists {
			from.sendError("already_requested", "connection request already exists")
			return
		}
		h.logger.Error("hub: create connection request", "error", err)
		from.sendError("server_error", "failed to create connection request")
		return
	}

	// Ack the sender.
	from.sendMsg(mustMarshal(ConnectRequestAckMsg{
		Type:         TypeConnectRequestAck,
		ConnectionID: conn.ID,
		Status:       connections.Pending,
	}))

	// Deliver to recipient if online; otherwise it's stored as unnotified.
	incomingMsg := mustMarshal(IncomingConnectReqMsg{
		Type:         TypeConnectRequest,
		ConnectionID: conn.ID,
		Requester: ContactUser{
			ID:     from.userID,
			Handle: from.cert.Cert.Subject.Handle,
			Name:   from.name,
		},
	})
	h.mu.RLock()
	target, online := h.clients[msg.TargetID]
	h.mu.RUnlock()
	if online {
		select {
		case target.send <- incomingMsg:
			if err := h.connStore.MarkRecipientNotified(conn.ID); err != nil {
				h.logger.Warn("hub: mark recipient notified", "conn_id", conn.ID, "error", err)
			}
		default:
		}
	}
	_ = targetCert
}

// handleConnectRespond processes an accept/reject from the recipient.
func (h *Hub) handleConnectRespond(from *Client, raw []byte) {
	var msg ConnectRespondMsg
	if err := json.Unmarshal(raw, &msg); err != nil || msg.ConnectionID == "" {
		from.sendError("bad_request", "invalid connect_respond payload")
		return
	}

	conn, err := h.connStore.GetByID(msg.ConnectionID)
	if err != nil {
		from.sendError("not_found", "connection not found")
		return
	}
	if conn.RecipientID != from.userID {
		from.sendError("forbidden", "only the recipient may respond to a connection request")
		return
	}
	if conn.Status != connections.Pending {
		from.sendError("already_responded", "connection request already responded to")
		return
	}

	newStatus := connections.Rejected
	if msg.Accept {
		newStatus = connections.Accepted
	}
	if err := h.connStore.UpdateStatus(conn.ID, newStatus); err != nil {
		h.logger.Error("hub: update connection status", "error", err)
		from.sendError("server_error", "failed to update connection")
		return
	}

	// Build the response for the requester.
	peer := ContactUser{
		ID:     from.userID,
		Handle: from.cert.Cert.Subject.Handle,
		Name:   from.name,
	}
	if msg.Accept {
		peer.SigningKey = from.signingKey
		peer.EncryptionKey = from.encryptionKey
	}
	respMsg := mustMarshal(IncomingConnectRespMsg{
		Type:         TypeConnectRespond,
		ConnectionID: conn.ID,
		Accept:       msg.Accept,
		Peer:         peer,
	})

	// Deliver to requester if online.
	h.mu.RLock()
	requesterClient, requesterOnline := h.clients[conn.RequesterID]
	h.mu.RUnlock()
	if requesterOnline {
		select {
		case requesterClient.send <- respMsg:
			if err := h.connStore.MarkRequesterNotified(conn.ID); err != nil {
				h.logger.Warn("hub: mark requester notified", "conn_id", conn.ID, "error", err)
			}
		default:
		}
	}

	// If accepted, refresh both rosters.
	if msg.Accept {
		h.sendRoster(from)
		if requesterOnline {
			h.sendRoster(requesterClient)
		}
	}
}

// acceptedPeerSet returns a set of peer IDs from a list of accepted connections.
func acceptedPeerSet(userID string, accepted []connections.Request) map[string]bool {
	peers := make(map[string]bool, len(accepted))
	for _, conn := range accepted {
		if conn.RequesterID == userID {
			peers[conn.RecipientID] = true
		} else {
			peers[conn.RequesterID] = true
		}
	}
	return peers
}

func mustMarshal(v any) []byte {
	b, err := json.Marshal(v)
	if err != nil {
		panic("hub: marshal failed: " + err.Error())
	}
	return b
}

// handleConnectionRemove removes an accepted connection between from and targetId.
func (h *Hub) handleConnectionRemove(from *Client, raw []byte) {
	var msg ConnectionRemoveMsg
	if err := json.Unmarshal(raw, &msg); err != nil || msg.TargetID == "" {
		from.sendError("bad_request", "invalid connection_remove payload")
		return
	}

	accepted, err := h.connStore.ListAccepted(from.userID)
	if err != nil {
		h.logger.Error("hub: list accepted connections for removal", "error", err)
		from.sendError("server_error", "failed to disconnect")
		return
	}

	var connID string
	for _, c := range accepted {
		if c.RequesterID == msg.TargetID || c.RecipientID == msg.TargetID {
			connID = c.ID
			break
		}
	}
	if connID == "" {
		from.sendError("not_found", "no accepted connection with that user")
		return
	}

	if err := h.connStore.Delete(connID); err != nil {
		h.logger.Error("hub: delete connection", "error", err)
		from.sendError("server_error", "failed to disconnect")
		return
	}

	// Ack the initiator and refresh both rosters.
	from.sendMsg(mustMarshal(ConnectionRemoveAckMsg{Type: TypeConnectionRemoveAck, TargetID: msg.TargetID}))
	h.sendRoster(from)
	h.mu.RLock()
	targetClient, online := h.clients[msg.TargetID]
	h.mu.RUnlock()
	if online {
		h.sendRoster(targetClient)
	}
}

// handleGroupLeave removes the requesting user from a group.
func (h *Hub) handleGroupLeave(from *Client, raw []byte) {
	var msg GroupLeaveMsg
	if err := json.Unmarshal(raw, &msg); err != nil || msg.GroupID == "" {
		from.sendError("bad_request", "invalid group_leave payload")
		return
	}

	g, err := h.groupStore.GetByID(msg.GroupID)
	if err != nil {
		from.sendError("not_found", "group not found")
		return
	}
	if g.OwnerID == from.userID {
		from.sendError("forbidden", "the group owner cannot leave")
		return
	}

	// Collect current members before removal so we can notify them.
	remaining, listErr := h.groupStore.ListAcceptedMembers(msg.GroupID)
	if listErr != nil {
		h.logger.Error("hub: list members before group leave", "error", listErr)
	}

	if err := h.groupStore.RemoveMember(msg.GroupID, from.userID); err != nil {
		if err == groups.ErrNotFound {
			from.sendError("not_found", "not a member of this group")
			return
		}
		h.logger.Error("hub: remove group member", "error", err)
		from.sendError("server_error", "failed to leave group")
		return
	}

	// Ack the leaving user and give them a fresh group roster (group is now gone).
	from.sendMsg(mustMarshal(GroupLeaveAckMsg{Type: TypeGroupLeaveAck, GroupID: msg.GroupID}))
	h.sendGroupRoster(from)

	// Notify remaining online members that this user permanently left.
	if listErr == nil {
		memberLeft := mustMarshal(GroupMemberLeftMsg{Type: TypeGroupMemberLeft, GroupID: msg.GroupID, UserID: from.userID})
		h.mu.RLock()
		onlineClients := h.clients
		h.mu.RUnlock()
		for _, m := range remaining {
			if m.UserID == from.userID {
				continue
			}
			if cl, ok := onlineClients[m.UserID]; ok {
				select {
				case cl.send <- memberLeft:
				default:
				}
			}
		}
	}
}

// ── Group handlers ────────────────────────────────────────────────────────────

// sendGroupRoster sends all accepted groups (with members + encryption keys) to c.
func (h *Hub) sendGroupRoster(c *Client) {
	userGroups, err := h.groupStore.ListUserGroups(c.userID)
	if err != nil {
		h.logger.Error("hub: list user groups for roster", "id", c.userID, "error", err)
		return
	}

	h.mu.RLock()
	onlineClients := h.clients
	h.mu.RUnlock()

	entries := make([]GroupRosterEntry, 0, len(userGroups))
	for _, g := range userGroups {
		members, listErr := h.groupStore.ListAcceptedMembers(g.ID)
		if listErr != nil {
			h.logger.Error("hub: list group members for roster", "group_id", g.ID, "error", listErr)
			continue
		}
		rosterMembers := make([]GroupRosterMember, 0, len(members))
		for _, m := range members {
			cert, lookupErr := h.reg.Get(m.UserID)
			if lookupErr != nil {
				continue
			}
			_, online := onlineClients[m.UserID]
			rosterMembers = append(rosterMembers, GroupRosterMember{
				ID:            cert.Cert.Subject.ID,
				Handle:        cert.Cert.Subject.Handle,
				Name:          cert.Cert.Subject.Name,
				EncryptionKey: cert.Cert.Subject.EncryptionKey,
				Online:        online,
			})
		}
		entries = append(entries, GroupRosterEntry{
			Group:   GroupSummary{ID: g.ID, Handle: g.Handle, Name: g.Name, OwnerID: g.OwnerID},
			Members: rosterMembers,
		})
	}

	msg := mustMarshal(GroupRosterMsg{Type: TypeGroupRoster, Groups: entries})
	select {
	case c.send <- msg:
	default:
	}
}

// deliverPendingGroupConnects sends undelivered group invites and responses on hello.
func (h *Hub) deliverPendingGroupConnects(c *Client) {
	undeliveredInvites, err := h.groupStore.ListUndeliveredInvites(c.userID)
	if err != nil {
		h.logger.Error("hub: list undelivered group invites", "id", c.userID, "error", err)
		return
	}
	undeliveredResponses, err := h.groupStore.ListUndeliveredResponses(c.userID)
	if err != nil {
		h.logger.Error("hub: list undelivered group responses", "id", c.userID, "error", err)
		return
	}

	invites := make([]OutboundGroupInviteMsg, 0, len(undeliveredInvites))
	for _, m := range undeliveredInvites {
		g, groupErr := h.groupStore.GetByID(m.GroupID)
		if groupErr != nil {
			continue
		}
		inviterCert, certErr := h.reg.Get(m.InviterID)
		if certErr != nil {
			continue
		}
		invites = append(invites, OutboundGroupInviteMsg{
			Type:     TypeGroupInvite,
			MemberID: m.ID,
			Group:    GroupSummary{ID: g.ID, Handle: g.Handle, Name: g.Name, OwnerID: g.OwnerID},
			Inviter: ContactUser{
				ID:     inviterCert.Cert.Subject.ID,
				Handle: inviterCert.Cert.Subject.Handle,
				Name:   inviterCert.Cert.Subject.Name,
			},
		})
		if markErr := h.groupStore.MarkInviteeNotified(m.ID); markErr != nil {
			h.logger.Warn("hub: mark invitee notified", "member_id", m.ID, "error", markErr)
		}
	}

	responses := make([]GroupRespondNotifyMsg, 0, len(undeliveredResponses))
	for _, m := range undeliveredResponses {
		respondentCert, certErr := h.reg.Get(m.UserID)
		if certErr != nil {
			continue
		}
		responses = append(responses, GroupRespondNotifyMsg{
			Type:     TypeGroupRespondNotify,
			MemberID: m.ID,
			GroupID:  m.GroupID,
			Accept:   m.Status == groups.Accepted,
			Member: ContactUser{
				ID:     respondentCert.Cert.Subject.ID,
				Handle: respondentCert.Cert.Subject.Handle,
				Name:   respondentCert.Cert.Subject.Name,
			},
		})
		if markErr := h.groupStore.MarkInviterNotified(m.ID); markErr != nil {
			h.logger.Warn("hub: mark inviter notified", "member_id", m.ID, "error", markErr)
		}
	}

	if len(invites) == 0 && len(responses) == 0 {
		return
	}
	payload := mustMarshal(PendingGroupConnectsMsg{
		Type:      TypePendingGroupConnects,
		Invites:   invites,
		Responses: responses,
	})
	select {
	case c.send <- payload:
	default:
		h.logger.Warn("hub: could not deliver pending group connects, buffer full", "id", c.userID)
	}
}

// broadcastGroupJoined notifies online group members that c has come online.
func (h *Hub) broadcastGroupJoined(c *Client) {
	userGroups, err := h.groupStore.ListUserGroups(c.userID)
	if err != nil {
		h.logger.Error("hub: list user groups for join broadcast", "id", c.userID, "error", err)
		return
	}
	if len(userGroups) == 0 {
		return
	}

	member := GroupRosterMember{
		ID:            c.userID,
		Handle:        c.cert.Cert.Subject.Handle,
		Name:          c.name,
		EncryptionKey: c.encryptionKey,
		Online:        true,
	}

	h.mu.RLock()
	onlineClients := h.clients
	h.mu.RUnlock()

	for _, g := range userGroups {
		allMembers, listErr := h.groupStore.ListAcceptedMembers(g.ID)
		if listErr != nil {
			continue
		}
		payload := mustMarshal(GroupUserJoinedMsg{Type: TypeGroupUserJoined, GroupID: g.ID, Member: member})
		for _, m := range allMembers {
			if m.UserID == c.userID {
				continue
			}
			if cl, ok := onlineClients[m.UserID]; ok {
				select {
				case cl.send <- payload:
				default:
				}
			}
		}
	}
}

// broadcastGroupLeft notifies online group members that userID has gone offline.
func (h *Hub) broadcastGroupLeft(userID string) {
	userGroups, err := h.groupStore.ListUserGroups(userID)
	if err != nil {
		h.logger.Error("hub: list user groups for leave broadcast", "id", userID, "error", err)
		return
	}
	if len(userGroups) == 0 {
		return
	}

	h.mu.RLock()
	onlineClients := h.clients
	h.mu.RUnlock()

	for _, g := range userGroups {
		allMembers, listErr := h.groupStore.ListAcceptedMembers(g.ID)
		if listErr != nil {
			continue
		}
		payload := mustMarshal(GroupUserLeftMsg{Type: TypeGroupUserLeft, GroupID: g.ID, UserID: userID})
		for _, m := range allMembers {
			if m.UserID == userID {
				continue
			}
			if cl, ok := onlineClients[m.UserID]; ok {
				select {
				case cl.send <- payload:
				default:
				}
			}
		}
	}
}

// handleGroupCreate processes a request to create a new group.
func (h *Hub) handleGroupCreate(from *Client, raw []byte) {
	var msg GroupCreateMsg
	if err := json.Unmarshal(raw, &msg); err != nil || msg.Handle == "" || msg.Name == "" {
		from.sendError("bad_request", "invalid group_create payload")
		return
	}
	if !groupHandleRe.MatchString(msg.Handle) {
		from.sendError("invalid_handle", "group handle must be 3–32 lowercase alphanumeric/underscore characters starting with a letter")
		return
	}

	g, err := h.groupStore.Create(from.userID, msg.Handle, msg.Name)
	if err != nil {
		if err == groups.ErrHandleExists {
			from.sendError("group_handle_taken", "group handle already taken")
			return
		}
		h.logger.Error("hub: create group", "error", err)
		from.sendError("server_error", "failed to create group")
		return
	}

	from.sendMsg(mustMarshal(GroupCreateAckMsg{
		Type:  TypeGroupCreateAck,
		Group: GroupSummary{ID: g.ID, Handle: g.Handle, Name: g.Name, OwnerID: g.OwnerID},
	}))
}

// handleGroupInvite processes an invite from the group owner to a target user.
func (h *Hub) handleGroupInvite(from *Client, raw []byte) {
	var msg GroupInviteMsg
	if err := json.Unmarshal(raw, &msg); err != nil || msg.GroupID == "" || msg.TargetID == "" {
		from.sendError("bad_request", "invalid group_invite payload")
		return
	}

	g, err := h.groupStore.GetByID(msg.GroupID)
	if err != nil {
		from.sendError("not_found", "group not found")
		return
	}
	if g.OwnerID != from.userID {
		from.sendError("forbidden", "only the group owner can invite members")
		return
	}

	targetCert, err := h.reg.Get(msg.TargetID)
	if err != nil {
		from.sendError("user_not_found", fmt.Sprintf("user %q not found", msg.TargetID))
		return
	}

	member, err := h.groupStore.AddMember(msg.GroupID, msg.TargetID, from.userID)
	if err != nil {
		if err == groups.ErrAlreadyMember {
			from.sendError("already_member", "user is already a member of this group")
			return
		}
		h.logger.Error("hub: add group member", "error", err)
		from.sendError("server_error", "failed to add member")
		return
	}

	// Ack the inviter.
	from.sendMsg(mustMarshal(GroupInviteAckMsg{
		Type:     TypeGroupInviteAck,
		MemberID: member.ID,
		GroupID:  msg.GroupID,
		Invitee: ContactUser{
			ID:     targetCert.Cert.Subject.ID,
			Handle: targetCert.Cert.Subject.Handle,
			Name:   targetCert.Cert.Subject.Name,
		},
	}))

	// Deliver invite to target if online; otherwise stored as unnotified.
	inviteMsg := mustMarshal(OutboundGroupInviteMsg{
		Type:     TypeGroupInvite,
		MemberID: member.ID,
		Group:    GroupSummary{ID: g.ID, Handle: g.Handle, Name: g.Name, OwnerID: g.OwnerID},
		Inviter: ContactUser{
			ID:     from.userID,
			Handle: from.cert.Cert.Subject.Handle,
			Name:   from.name,
		},
	})
	h.mu.RLock()
	targetClient, online := h.clients[msg.TargetID]
	h.mu.RUnlock()
	if online {
		select {
		case targetClient.send <- inviteMsg:
			if markErr := h.groupStore.MarkInviteeNotified(member.ID); markErr != nil {
				h.logger.Warn("hub: mark invitee notified", "member_id", member.ID, "error", markErr)
			}
		default:
		}
	}
	_ = targetCert
}

// handleGroupRespond processes an accept/reject from an invited user.
func (h *Hub) handleGroupRespond(from *Client, raw []byte) {
	var msg GroupRespondMsg
	if err := json.Unmarshal(raw, &msg); err != nil || msg.MemberID == "" {
		from.sendError("bad_request", "invalid group_respond payload")
		return
	}

	member, err := h.groupStore.GetMemberByID(msg.MemberID)
	if err != nil {
		from.sendError("not_found", "membership not found")
		return
	}
	if member.UserID != from.userID {
		from.sendError("forbidden", "only the invitee can respond to a group invite")
		return
	}
	if member.Status != groups.Invited {
		from.sendError("already_responded", "invite already responded to")
		return
	}

	newStatus := groups.Rejected
	if msg.Accept {
		newStatus = groups.Accepted
	}
	if err := h.groupStore.UpdateMemberStatus(msg.MemberID, newStatus); err != nil {
		h.logger.Error("hub: update group member status", "error", err)
		from.sendError("server_error", "failed to update membership")
		return
	}

	// Notify the inviter.
	respondMsg := mustMarshal(GroupRespondNotifyMsg{
		Type:     TypeGroupRespondNotify,
		MemberID: msg.MemberID,
		GroupID:  member.GroupID,
		Accept:   msg.Accept,
		Member: ContactUser{
			ID:     from.userID,
			Handle: from.cert.Cert.Subject.Handle,
			Name:   from.name,
		},
	})
	h.mu.RLock()
	inviterClient, inviterOnline := h.clients[member.InviterID]
	h.mu.RUnlock()
	if inviterOnline {
		select {
		case inviterClient.send <- respondMsg:
			if markErr := h.groupStore.MarkInviterNotified(msg.MemberID); markErr != nil {
				h.logger.Warn("hub: mark inviter notified", "member_id", msg.MemberID, "error", markErr)
			}
		default:
		}
	}

	// If accepted, refresh group roster for new member and all existing members.
	if msg.Accept {
		h.sendGroupRoster(from)
		allMembers, listErr := h.groupStore.ListAcceptedMembers(member.GroupID)
		if listErr == nil {
			h.mu.RLock()
			onlineClients := h.clients
			h.mu.RUnlock()
			for _, m := range allMembers {
				if m.UserID == from.userID {
					continue
				}
				if cl, ok := onlineClients[m.UserID]; ok {
					h.sendGroupRoster(cl)
				}
			}
		}
	}
}

// handleGroupMessage routes an E2E-encrypted group message to all accepted members.
func (h *Hub) handleGroupMessage(from *Client, raw []byte) {
	var msg GroupSendMsg
	if err := json.Unmarshal(raw, &msg); err != nil || msg.GroupID == "" || msg.EncryptedPayload == "" {
		from.sendError("bad_request", "invalid group_message payload")
		return
	}

	// Verify sender is an accepted member.
	if _, err := h.groupStore.GetMember(msg.GroupID, from.userID); err != nil {
		from.sendError("forbidden", "not a member of this group")
		return
	}

	allMembers, err := h.groupStore.ListAcceptedMembers(msg.GroupID)
	if err != nil {
		h.logger.Error("hub: list accepted members for group message", "group_id", msg.GroupID, "error", err)
		from.sendError("server_error", "failed to send group message")
		return
	}

	msgID := queue.NewID()
	timestamp := time.Now().UnixMilli()

	delivery := mustMarshal(GroupDeliveryMsg{
		Type:             TypeGroupMessage,
		ID:               msgID,
		GroupID:          msg.GroupID,
		From:             from.userID,
		EncryptedPayload: msg.EncryptedPayload,
		SenderCert:       from.cert,
		Timestamp:        timestamp,
	})

	delivered, queued := 0, 0
	for _, m := range allMembers {
		if m.UserID == from.userID {
			continue
		}
		if h.Route(m.UserID, delivery) {
			delivered++
		} else {
			h.queue.Push(queue.PersistedMessage{
				ID:               msgID,
				From:             from.userID,
				To:               m.UserID,
				GroupID:          msg.GroupID,
				EncryptedPayload: msg.EncryptedPayload,
				SenderCert:       from.cert,
				Timestamp:        timestamp,
			})
			queued++
		}
	}

	status := "delivered"
	if delivered == 0 && queued > 0 {
		status = "queued"
	} else if delivered > 0 && queued > 0 {
		status = "partial"
	}

	from.sendMsg(mustMarshal(GroupMessageAckMsg{
		Type:        TypeGroupMessageAck,
		ClientMsgID: msg.ClientMsgID,
		ServerMsgID: msgID,
		GroupID:     msg.GroupID,
		Status:      status,
		Timestamp:   timestamp,
	}))
}
