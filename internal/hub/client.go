package hub

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"time"

	"github.com/gorilla/websocket"

	"github.com/prasenjit-net/pigeon/internal/ca"
	"github.com/prasenjit-net/pigeon/internal/queue"
)

const (
	writeWait      = 10 * time.Second
	pongWait       = 60 * time.Second
	pingPeriod     = 50 * time.Second
	maxMessageSize = 64 * 1024 // 64 KB
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin:     func(r *http.Request) bool { return true },
}

// Client is a single WebSocket connection with read and write pump goroutines.
type Client struct {
	hub    *Hub
	conn   *websocket.Conn
	send   chan []byte
	logger *slog.Logger

	// populated after a valid hello handshake
	userID        string
	name          string
	cert          ca.SignedCertificate
	signingKey    map[string]any
	encryptionKey map[string]any
}

// ServeWS upgrades the HTTP connection to WebSocket and starts the pumps.
func ServeWS(h *Hub, w http.ResponseWriter, r *http.Request, logger *slog.Logger) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		logger.Error("ws: upgrade failed", "error", err)
		return
	}
	c := &Client{
		hub:    h,
		conn:   conn,
		send:   make(chan []byte, 64),
		logger: logger,
	}
	go c.writePump()
	go c.readPump()
}

func (c *Client) readPump() {
	defer func() {
		if c.userID != "" {
			c.hub.Unregister(c)
		}
		c.conn.Close()
	}()

	c.conn.SetReadLimit(maxMessageSize)
	c.conn.SetReadDeadline(time.Now().Add(pongWait))
	c.conn.SetPongHandler(func(string) error {
		c.conn.SetReadDeadline(time.Now().Add(pongWait))
		return nil
	})

	for {
		_, raw, err := c.conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseNormalClosure) {
				c.logger.Warn("ws: read error", "error", err)
			}
			return
		}
		c.dispatch(raw)
	}
}

func (c *Client) writePump() {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		c.conn.Close()
	}()

	for {
		select {
		case msg, ok := <-c.send:
			c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if !ok {
				c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}
			if err := c.conn.WriteMessage(websocket.TextMessage, msg); err != nil {
				return
			}
		case <-ticker.C:
			c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

func (c *Client) dispatch(raw []byte) {
	var env InboundEnvelope
	if err := json.Unmarshal(raw, &env); err != nil {
		c.sendError("bad_json", "invalid JSON")
		return
	}
	switch env.Type {
	case TypeHello:
		c.handleHello(raw)
	case TypeMessage:
		c.handleSend(raw)
	case TypeConnectRequest:
		c.hub.handleConnectRequest(c, raw)
	case TypeConnectRespond:
		c.hub.handleConnectRespond(c, raw)
	case TypeGroupCreate:
		c.hub.handleGroupCreate(c, raw)
	case TypeGroupInvite:
		c.hub.handleGroupInvite(c, raw)
	case TypeGroupRespond:
		c.hub.handleGroupRespond(c, raw)
	case TypeGroupMessage:
		c.hub.handleGroupMessage(c, raw)
	default:
		c.sendError("unknown_type", "unknown message type: "+env.Type)
	}
}

func (c *Client) handleHello(raw []byte) {
	var msg HelloMsg
	if err := json.Unmarshal(raw, &msg); err != nil {
		c.sendError("bad_hello", "invalid hello message")
		return
	}
	// Certificates issued before the handle feature was added have an empty
	// Handle field. Reject them so the client re-registers with a handle.
	if msg.Certificate.Cert.Subject.Handle == "" {
		c.logger.Warn("ws: rejected hello with outdated cert (no handle)")
		c.sendError("cert_outdated", "certificate is outdated — please re-register")
		return
	}
	if err := c.hub.CA().VerifyCertificate(msg.Certificate); err != nil {
		c.logger.Warn("ws: rejected hello with invalid cert", "error", err)
		c.sendError("invalid_cert", "certificate verification failed: "+err.Error())
		return
	}

	c.userID = msg.Certificate.Cert.Subject.ID
	c.name = msg.Certificate.Cert.Subject.Name
	c.cert = msg.Certificate
	c.signingKey = msg.Certificate.Cert.Subject.SigningKey
	c.encryptionKey = msg.Certificate.Cert.Subject.EncryptionKey

	c.hub.Register(c)
	// pending_messages and roster are sent by the hub's Run loop after Register.
}

func (c *Client) handleSend(raw []byte) {
	if c.userID == "" {
		c.sendError("not_identified", "send hello first")
		return
	}

	var msg SendMsg
	if err := json.Unmarshal(raw, &msg); err != nil {
		c.sendError("bad_message", "invalid message payload")
		return
	}
	if msg.To == "" || msg.EncryptedPayload == "" {
		c.sendError("bad_message", "to and encryptedPayload are required")
		return
	}

	// Build the persisted message (carries the server-assigned ID and timestamp).
	pm := queue.NewMessage(c.userID, msg.To, msg.EncryptedPayload, c.cert)

	// Attempt live delivery first.
	delivery := mustMarshal(DeliveryMsg{
		Type:             TypeMessage,
		ID:               pm.ID,
		From:             pm.From,
		EncryptedPayload: pm.EncryptedPayload,
		SenderCert:       pm.SenderCert,
		Timestamp:        pm.Timestamp,
	})

	var status string
	if c.hub.Route(msg.To, delivery) {
		status = "delivered"
	} else {
		// Recipient offline — store for later delivery.
		c.hub.Queue().Push(pm)
		status = "queued"
	}

	// Ack the sender regardless of delivery outcome.
	c.sendMsg(mustMarshal(MessageAckMsg{
		Type:        TypeMessageAck,
		ClientMsgID: msg.ClientMsgID,
		ServerMsgID: pm.ID,
		Status:      status,
		Timestamp:   pm.Timestamp,
	}))
}

func (c *Client) sendError(code, message string) {
	c.sendMsg(mustMarshal(ErrorMsg{Type: TypeError, Code: code, Message: message}))
}

func (c *Client) sendMsg(payload []byte) {
	select {
	case c.send <- payload:
	default:
	}
}

