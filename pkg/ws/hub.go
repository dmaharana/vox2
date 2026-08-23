package ws

import (
	"context"
	"encoding/json"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/rs/zerolog/log"
)

const (
	writeWait      = 10 * time.Second
	pongWait       = 60 * time.Second
	pingPeriod     = (pongWait * 9) / 10
	maxMessageSize = 512 * 1024 // 512 KB
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true // Allow all origins for local dev / embedded harness
	},
}

// Client represents a single active WebSocket connection.
type Client struct {
	hub  *Hub
	conn *websocket.Conn
	send chan OutboundMessage

	mu            sync.Mutex
	activeCancels map[string]context.CancelFunc
}

// MessageHandler is the signature for handling incoming client messages.
type MessageHandler func(ctx context.Context, client *Client, msg InboundMessage)

// Hub maintains the set of active clients and handles message routing.
type Hub struct {
	mu         sync.RWMutex
	clients    map[*Client]bool
	broadcast  chan OutboundMessage
	register   chan *Client
	unregister chan *Client
	handler    MessageHandler
	ctx        context.Context
	cancel     context.CancelFunc
}

// NewHub creates a new Hub instance.
func NewHub(handler MessageHandler) *Hub {
	ctx, cancel := context.WithCancel(context.Background())
	return &Hub{
		clients:    make(map[*Client]bool),
		broadcast:  make(chan OutboundMessage, 256),
		register:   make(chan *Client),
		unregister: make(chan *Client),
		handler:    handler,
		ctx:        ctx,
		cancel:     cancel,
	}
}

// SetHandler updates the message handler.
func (h *Hub) SetHandler(handler MessageHandler) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.handler = handler
}

// Run starts the Hub event loop.
func (h *Hub) Run() {
	for {
		select {
		case <-h.ctx.Done():
			h.mu.Lock()
			for client := range h.clients {
				close(client.send)
				delete(h.clients, client)
			}
			h.mu.Unlock()
			return

		case client := <-h.register:
			h.mu.Lock()
			h.clients[client] = true
			h.mu.Unlock()
			log.Debug().Msg("WebSocket client connected")

		case client := <-h.unregister:
			h.mu.Lock()
			if _, ok := h.clients[client]; ok {
				delete(h.clients, client)
				close(client.send)
			}
			h.mu.Unlock()
			log.Debug().Msg("WebSocket client disconnected")

		case message := <-h.broadcast:
			h.mu.RLock()
			for client := range h.clients {
				select {
				case client.send <- message:
				default:
					close(client.send)
					delete(h.clients, client)
				}
			}
			h.mu.RUnlock()
		}
	}
}

// Close stops the Hub.
func (h *Hub) Close() {
	h.cancel()
}

// Broadcast sends a message to all connected clients.
func (h *Hub) Broadcast(msg OutboundMessage) {
	if msg.Timestamp.IsZero() {
		msg.Timestamp = time.Now()
	}
	h.broadcast <- msg
}

// ServeWS upgrades the HTTP request to a WebSocket connection and registers the client.
func (h *Hub) ServeWS(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Error().Err(err).Msg("Failed to upgrade WebSocket connection")
		return
	}

	client := &Client{
		hub:           h,
		conn:          conn,
		send:          make(chan OutboundMessage, 256),
		activeCancels: make(map[string]context.CancelFunc),
	}

	h.register <- client

	go client.writePump()
	go client.readPump()
}

// Send sends an outbound message to this specific client.
func (c *Client) Send(msg OutboundMessage) {
	if msg.Timestamp.IsZero() {
		msg.Timestamp = time.Now()
	}
	select {
	case c.send <- msg:
	default:
		log.Warn().Msg("Client send buffer full, dropping message")
	}
}

// RegisterCancel registers a cancel function for a given conversation/request ID.
func (c *Client) RegisterCancel(id string, cancel context.CancelFunc) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.activeCancels[id] = cancel
}

// Cancel cancels the active context for an ID.
func (c *Client) Cancel(id string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if cancel, exists := c.activeCancels[id]; exists {
		cancel()
		delete(c.activeCancels, id)
	}
}

// RemoveCancel removes the cancel function once done.
func (c *Client) RemoveCancel(id string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.activeCancels, id)
}

func (c *Client) readPump() {
	defer func() {
		c.mu.Lock()
		for _, cancel := range c.activeCancels {
			cancel()
		}
		c.activeCancels = make(map[string]context.CancelFunc)
		c.mu.Unlock()

		c.hub.unregister <- c
		_ = c.conn.Close()
	}()

	c.conn.SetReadLimit(maxMessageSize)
	_ = c.conn.SetReadDeadline(time.Now().Add(pongWait))
	c.conn.SetPongHandler(func(string) error {
		_ = c.conn.SetReadDeadline(time.Now().Add(pongWait))
		return nil
	})

	for {
		_, messageBytes, err := c.conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Warn().Err(err).Msg("WebSocket read error")
			}
			break
		}

		var msg InboundMessage
		if err := json.Unmarshal(messageBytes, &msg); err != nil {
			c.Send(OutboundMessage{
				Type:  TypeError,
				Error: "Invalid JSON message format",
			})
			continue
		}

		if msg.Type == TypePing {
			c.Send(OutboundMessage{
				Type:    TypePong,
				Payload: "pong",
			})
			continue
		}

		if msg.Type == TypeCancel {
			if msg.ConversationID != "" {
				c.Cancel(msg.ConversationID)
			}
			continue
		}

		c.hub.mu.RLock()
		handler := c.hub.handler
		c.hub.mu.RUnlock()

		if handler != nil {
			go handler(context.Background(), c, msg)
		}
	}
}

func (c *Client) writePump() {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		_ = c.conn.Close()
	}()

	for {
		select {
		case message, ok := <-c.send:
			_ = c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if !ok {
				_ = c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			w, err := c.conn.NextWriter(websocket.TextMessage)
			if err != nil {
				return
			}
			if err := json.NewEncoder(w).Encode(message); err != nil {
				_ = w.Close()
				return
			}

			if err := w.Close(); err != nil {
				return
			}

		case <-ticker.C:
			_ = c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}
