package hub

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"time"

	"github.com/gorilla/websocket"
	"github.com/prasenjit-net/pigeon/internal/ca"
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
	// Allow all origins in development; tighten in production.
	CheckOrigin: func(r *http.Request) bool { return true },
}

// Client is a single WebSocket connection. It has two goroutines: readPump
// and writePump, communicating with the hub via the send channel.
type Client struct {
	hub    *Hub
	conn   *websocket.Conn
	send   chan []byte
	logger *slog.Logger

	// set after successful hello handshake
	userID        string
	name          string
	cert          ca.SignedCertificate
	signingKey    map[string]any
	encryptionKey map[string]any
}

// ServeWS upgrades an HTTP connection to WebSocket and starts the client pumps.
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

// readPump processes inbound messages from the WebSocket.
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

// writePump drains the send channel and writes to the WebSocket.
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

// dispatch routes a raw inbound message to the correct handler.
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

	delivery := mustMarshal(DeliveryMsg{
		Type:             TypeMessage,
		From:             c.userID,
		EncryptedPayload: msg.EncryptedPayload,
		SenderCert:       c.cert,
	})

	if !c.hub.Route(msg.To, delivery) {
		c.sendError("offline", "recipient is not online")
	}
}

func (c *Client) sendError(code, message string) {
	payload := mustMarshal(ErrorMsg{Type: TypeError, Code: code, Message: message})
	select {
	case c.send <- payload:
	default:
	}
}

func (c *Client) onlineUser() OnlineUser {
	return OnlineUser{
		ID:            c.userID,
		Name:          c.name,
		SigningKey:     c.signingKey,
		EncryptionKey: c.encryptionKey,
	}
}

func mustMarshal(v any) []byte {
	b, err := json.Marshal(v)
	if err != nil {
		panic("hub: marshal failed: " + err.Error())
	}
	return b
}
