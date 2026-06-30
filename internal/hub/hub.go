package hub

import (
	"log/slog"
	"sync"

	"github.com/prasenjit-net/pigeon/internal/ca"
)

// Hub maintains the set of active WebSocket clients and routes messages
// between them. A single goroutine owns the clients map; all mutations go
// through channels to avoid mutexes on the hot path.
type Hub struct {
	mu      sync.RWMutex
	clients map[string]*Client // keyed by subject.id

	register   chan *Client
	unregister chan *Client
	route      chan routeReq

	authority *ca.CA
	logger    *slog.Logger
}

type routeReq struct {
	to      string
	payload []byte
}

func New(authority *ca.CA, logger *slog.Logger) *Hub {
	return &Hub{
		clients:    make(map[string]*Client),
		register:   make(chan *Client, 16),
		unregister: make(chan *Client, 16),
		route:      make(chan routeReq, 256),
		authority:  authority,
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
			h.broadcastUserJoined(c)
			h.sendRoster(c)

		case c := <-h.unregister:
			h.mu.Lock()
			if existing, ok := h.clients[c.userID]; ok && existing == c {
				delete(h.clients, c.userID)
			}
			h.mu.Unlock()

			h.logger.Info("hub: user disconnected", "id", c.userID, "name", c.name)
			h.broadcastUserLeft(c.userID)

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

// Route queues a raw JSON payload to be delivered to the recipient.
// Returns false immediately if the recipient is not online (non-blocking).
func (h *Hub) Route(to string, payload []byte) bool {
	h.mu.RLock()
	_, ok := h.clients[to]
	h.mu.RUnlock()
	if !ok {
		return false
	}
	h.route <- routeReq{to: to, payload: payload}
	return true
}

// CA returns the hub's CA so clients can verify certificates.
func (h *Hub) CA() *ca.CA { return h.authority }

// roster builds the current OnlineUser list (caller holds no lock — takes
// its own read lock).
func (h *Hub) roster() []OnlineUser {
	h.mu.RLock()
	defer h.mu.RUnlock()
	users := make([]OnlineUser, 0, len(h.clients))
	for _, c := range h.clients {
		users = append(users, c.onlineUser())
	}
	return users
}

func (h *Hub) sendRoster(c *Client) {
	msg := mustMarshal(RosterMsg{Type: TypeRoster, Users: h.roster()})
	select {
	case c.send <- msg:
	default:
	}
}

func (h *Hub) broadcastUserJoined(c *Client) {
	msg := mustMarshal(UserJoinedMsg{Type: TypeUserJoined, User: c.onlineUser()})
	h.mu.RLock()
	defer h.mu.RUnlock()
	for id, cl := range h.clients {
		if id == c.userID {
			continue // don't send to the joining user — they get a full roster instead
		}
		select {
		case cl.send <- msg:
		default:
		}
	}
}

func (h *Hub) broadcastUserLeft(userID string) {
	msg := mustMarshal(UserLeftMsg{Type: TypeUserLeft, ID: userID})
	h.mu.RLock()
	defer h.mu.RUnlock()
	for _, cl := range h.clients {
		select {
		case cl.send <- msg:
		default:
		}
	}
}
