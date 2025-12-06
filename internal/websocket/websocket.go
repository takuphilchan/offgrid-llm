package websocket

import (
	"bufio"
	"context"
	"crypto/sha1"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"
)

// MessageType represents the type of WebSocket message
type MessageType int

const (
	TextMessage   MessageType = 1
	BinaryMessage MessageType = 2
	CloseMessage  MessageType = 8
	PingMessage   MessageType = 9
	PongMessage   MessageType = 10
)

// Connection represents a WebSocket connection
type Connection struct {
	conn       net.Conn
	mu         sync.Mutex
	closed     bool
	onMessage  func(MessageType, []byte)
	onClose    func()
	onError    func(error)
	pingTicker *time.Ticker
}

// Message represents a WebSocket message
type Message struct {
	Type MessageType
	Data []byte
}

// Upgrade upgrades an HTTP connection to WebSocket
func Upgrade(w http.ResponseWriter, r *http.Request) (*Connection, error) {
	// Verify WebSocket upgrade request
	if r.Header.Get("Upgrade") != "websocket" {
		return nil, fmt.Errorf("missing websocket upgrade header")
	}

	key := r.Header.Get("Sec-WebSocket-Key")
	if key == "" {
		return nil, fmt.Errorf("missing Sec-WebSocket-Key")
	}

	// Generate accept key
	acceptKey := generateAcceptKey(key)

	// Hijack the connection
	hijacker, ok := w.(http.Hijacker)
	if !ok {
		return nil, fmt.Errorf("hijacking not supported")
	}

	conn, _, err := hijacker.Hijack()
	if err != nil {
		return nil, fmt.Errorf("failed to hijack connection: %w", err)
	}

	// Send upgrade response
	response := "HTTP/1.1 101 Switching Protocols\r\n" +
		"Upgrade: websocket\r\n" +
		"Connection: Upgrade\r\n" +
		"Sec-WebSocket-Accept: " + acceptKey + "\r\n\r\n"

	if _, err := conn.Write([]byte(response)); err != nil {
		conn.Close()
		return nil, fmt.Errorf("failed to send upgrade response: %w", err)
	}

	wsConn := &Connection{
		conn: conn,
	}

	return wsConn, nil
}

// generateAcceptKey generates the Sec-WebSocket-Accept key
func generateAcceptKey(key string) string {
	const guid = "258EAFA5-E914-47DA-95CA-C5AB0DC85B11"
	h := sha1.New()
	h.Write([]byte(key + guid))
	return base64.StdEncoding.EncodeToString(h.Sum(nil))
}

// SetMessageHandler sets the message handler
func (c *Connection) SetMessageHandler(handler func(MessageType, []byte)) {
	c.onMessage = handler
}

// SetCloseHandler sets the close handler
func (c *Connection) SetCloseHandler(handler func()) {
	c.onClose = handler
}

// SetErrorHandler sets the error handler
func (c *Connection) SetErrorHandler(handler func(error)) {
	c.onError = handler
}

// ReadLoop reads messages from the connection
func (c *Connection) ReadLoop() {
	defer c.Close()

	reader := bufio.NewReader(c.conn)

	for {
		msgType, data, err := c.readFrame(reader)
		if err != nil {
			if err != io.EOF && !c.closed {
				if c.onError != nil {
					c.onError(err)
				}
			}
			return
		}

		switch msgType {
		case PingMessage:
			c.WriteMessage(PongMessage, data)
		case PongMessage:
			// Pong received, connection is alive
		case CloseMessage:
			return
		default:
			if c.onMessage != nil {
				c.onMessage(msgType, data)
			}
		}
	}
}

// readFrame reads a WebSocket frame
func (c *Connection) readFrame(reader *bufio.Reader) (MessageType, []byte, error) {
	// Read first two bytes
	header := make([]byte, 2)
	if _, err := io.ReadFull(reader, header); err != nil {
		return 0, nil, err
	}

	opcode := MessageType(header[0] & 0x0F)
	masked := header[1]&0x80 != 0
	payloadLen := int64(header[1] & 0x7F)

	// Extended payload length
	if payloadLen == 126 {
		ext := make([]byte, 2)
		if _, err := io.ReadFull(reader, ext); err != nil {
			return 0, nil, err
		}
		payloadLen = int64(ext[0])<<8 | int64(ext[1])
	} else if payloadLen == 127 {
		ext := make([]byte, 8)
		if _, err := io.ReadFull(reader, ext); err != nil {
			return 0, nil, err
		}
		payloadLen = int64(ext[0])<<56 | int64(ext[1])<<48 | int64(ext[2])<<40 | int64(ext[3])<<32 |
			int64(ext[4])<<24 | int64(ext[5])<<16 | int64(ext[6])<<8 | int64(ext[7])
	}

	// Read mask key if present
	var maskKey []byte
	if masked {
		maskKey = make([]byte, 4)
		if _, err := io.ReadFull(reader, maskKey); err != nil {
			return 0, nil, err
		}
	}

	// Read payload
	payload := make([]byte, payloadLen)
	if _, err := io.ReadFull(reader, payload); err != nil {
		return 0, nil, err
	}

	// Unmask payload
	if masked {
		for i := range payload {
			payload[i] ^= maskKey[i%4]
		}
	}

	return opcode, payload, nil
}

// WriteMessage writes a message to the connection
func (c *Connection) WriteMessage(msgType MessageType, data []byte) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.closed {
		return fmt.Errorf("connection closed")
	}

	frame := c.buildFrame(msgType, data)
	_, err := c.conn.Write(frame)
	return err
}

// WriteJSON writes a JSON message
func (c *Connection) WriteJSON(v interface{}) error {
	data, err := json.Marshal(v)
	if err != nil {
		return err
	}
	return c.WriteMessage(TextMessage, data)
}

// buildFrame builds a WebSocket frame
func (c *Connection) buildFrame(msgType MessageType, data []byte) []byte {
	length := len(data)
	frame := make([]byte, 0, length+10)

	// First byte: FIN + opcode
	frame = append(frame, 0x80|byte(msgType))

	// Second byte: payload length
	if length < 126 {
		frame = append(frame, byte(length))
	} else if length < 65536 {
		frame = append(frame, 126, byte(length>>8), byte(length))
	} else {
		frame = append(frame, 127)
		for i := 7; i >= 0; i-- {
			frame = append(frame, byte(length>>(i*8)))
		}
	}

	// Payload
	frame = append(frame, data...)
	return frame
}

// Close closes the connection
func (c *Connection) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.closed {
		return nil
	}

	c.closed = true

	if c.pingTicker != nil {
		c.pingTicker.Stop()
	}

	if c.onClose != nil {
		c.onClose()
	}

	// Send close frame
	closeFrame := c.buildFrame(CloseMessage, []byte{0x03, 0xE8}) // Normal closure
	c.conn.Write(closeFrame)

	return c.conn.Close()
}

// StartPing starts sending ping messages
func (c *Connection) StartPing(interval time.Duration) {
	c.pingTicker = time.NewTicker(interval)
	go func() {
		for range c.pingTicker.C {
			if err := c.WriteMessage(PingMessage, []byte("ping")); err != nil {
				return
			}
		}
	}()
}

// Hub manages multiple WebSocket connections
type Hub struct {
	mu          sync.RWMutex
	connections map[*Connection]bool
	channels    map[string]map[*Connection]bool
	broadcast   chan Message
	register    chan *Connection
	unregister  chan *Connection
	logger      *log.Logger
}

// NewHub creates a new WebSocket hub
func NewHub() *Hub {
	return &Hub{
		connections: make(map[*Connection]bool),
		channels:    make(map[string]map[*Connection]bool),
		broadcast:   make(chan Message, 256),
		register:    make(chan *Connection),
		unregister:  make(chan *Connection),
		logger:      log.New(log.Writer(), "[WS Hub] ", log.LstdFlags),
	}
}

// Run starts the hub
func (h *Hub) Run(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			h.closeAll()
			return
		case conn := <-h.register:
			h.mu.Lock()
			h.connections[conn] = true
			h.mu.Unlock()
			h.logger.Printf("Connection registered, total: %d", len(h.connections))

		case conn := <-h.unregister:
			h.mu.Lock()
			if _, ok := h.connections[conn]; ok {
				delete(h.connections, conn)
				// Remove from all channels
				for channel := range h.channels {
					delete(h.channels[channel], conn)
				}
			}
			h.mu.Unlock()
			h.logger.Printf("Connection unregistered, total: %d", len(h.connections))

		case message := <-h.broadcast:
			h.mu.RLock()
			for conn := range h.connections {
				go func(c *Connection) {
					if err := c.WriteMessage(message.Type, message.Data); err != nil {
						h.unregister <- c
					}
				}(conn)
			}
			h.mu.RUnlock()
		}
	}
}

// Register registers a connection
func (h *Hub) Register(conn *Connection) {
	h.register <- conn
}

// Unregister unregisters a connection
func (h *Hub) Unregister(conn *Connection) {
	h.unregister <- conn
}

// Broadcast sends a message to all connections
func (h *Hub) Broadcast(msgType MessageType, data []byte) {
	h.broadcast <- Message{Type: msgType, Data: data}
}

// BroadcastJSON broadcasts a JSON message
func (h *Hub) BroadcastJSON(v interface{}) error {
	data, err := json.Marshal(v)
	if err != nil {
		return err
	}
	h.Broadcast(TextMessage, data)
	return nil
}

// Subscribe adds a connection to a channel
func (h *Hub) Subscribe(conn *Connection, channel string) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if h.channels[channel] == nil {
		h.channels[channel] = make(map[*Connection]bool)
	}
	h.channels[channel][conn] = true
}

// Unsubscribe removes a connection from a channel
func (h *Hub) Unsubscribe(conn *Connection, channel string) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if h.channels[channel] != nil {
		delete(h.channels[channel], conn)
	}
}

// PublishToChannel sends a message to all connections in a channel
func (h *Hub) PublishToChannel(channel string, msgType MessageType, data []byte) {
	h.mu.RLock()
	conns, ok := h.channels[channel]
	h.mu.RUnlock()

	if !ok {
		return
	}

	for conn := range conns {
		go func(c *Connection) {
			if err := c.WriteMessage(msgType, data); err != nil {
				h.Unsubscribe(c, channel)
			}
		}(conn)
	}
}

// PublishJSONToChannel publishes JSON to a channel
func (h *Hub) PublishJSONToChannel(channel string, v interface{}) error {
	data, err := json.Marshal(v)
	if err != nil {
		return err
	}
	h.PublishToChannel(channel, TextMessage, data)
	return nil
}

// closeAll closes all connections
func (h *Hub) closeAll() {
	h.mu.Lock()
	defer h.mu.Unlock()

	for conn := range h.connections {
		conn.Close()
	}
	h.connections = make(map[*Connection]bool)
	h.channels = make(map[string]map[*Connection]bool)
}

// ConnectionCount returns the number of active connections
func (h *Hub) ConnectionCount() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.connections)
}

// StreamEvent represents an event for streaming
type StreamEvent struct {
	Type      string          `json:"type"` // "start", "token", "done", "error"
	RequestID string          `json:"request_id,omitempty"`
	Token     string          `json:"token,omitempty"`
	Content   string          `json:"content,omitempty"`
	Model     string          `json:"model,omitempty"`
	Error     string          `json:"error,omitempty"`
	Usage     *StreamingUsage `json:"usage,omitempty"`
	Metadata  map[string]any  `json:"metadata,omitempty"`
}

// StreamingUsage contains token usage info
type StreamingUsage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

// ChatRequest represents a WebSocket chat request
type ChatRequest struct {
	Type      string         `json:"type"` // "chat", "cancel", "subscribe"
	RequestID string         `json:"request_id"`
	Model     string         `json:"model,omitempty"`
	Messages  []ChatMessage  `json:"messages,omitempty"`
	Options   map[string]any `json:"options,omitempty"`
	Channel   string         `json:"channel,omitempty"`
}

// ChatMessage for WebSocket
type ChatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// StreamHandler handles streaming chat over WebSocket
type StreamHandler struct {
	hub           *Hub
	chatFunc      func(ctx context.Context, req ChatRequest) (<-chan StreamEvent, error)
	activeStreams map[string]context.CancelFunc
	mu            sync.Mutex
}

// NewStreamHandler creates a new stream handler
func NewStreamHandler(hub *Hub) *StreamHandler {
	return &StreamHandler{
		hub:           hub,
		activeStreams: make(map[string]context.CancelFunc),
	}
}

// SetChatFunction sets the function to handle chat requests
func (h *StreamHandler) SetChatFunction(fn func(ctx context.Context, req ChatRequest) (<-chan StreamEvent, error)) {
	h.chatFunc = fn
}

// HandleConnection handles a WebSocket connection for streaming
func (h *StreamHandler) HandleConnection(conn *Connection) {
	h.hub.Register(conn)

	conn.SetCloseHandler(func() {
		h.hub.Unregister(conn)
	})

	conn.SetMessageHandler(func(msgType MessageType, data []byte) {
		if msgType != TextMessage {
			return
		}

		var req ChatRequest
		if err := json.Unmarshal(data, &req); err != nil {
			conn.WriteJSON(StreamEvent{
				Type:  "error",
				Error: "Invalid JSON: " + err.Error(),
			})
			return
		}

		switch req.Type {
		case "chat":
			go h.handleChat(conn, req)
		case "cancel":
			h.cancelStream(req.RequestID)
		case "subscribe":
			h.hub.Subscribe(conn, req.Channel)
		case "unsubscribe":
			h.hub.Unsubscribe(conn, req.Channel)
		}
	})

	conn.ReadLoop()
}

// handleChat handles a chat request
func (h *StreamHandler) handleChat(conn *Connection, req ChatRequest) {
	if h.chatFunc == nil {
		conn.WriteJSON(StreamEvent{
			Type:      "error",
			RequestID: req.RequestID,
			Error:     "Chat function not configured",
		})
		return
	}

	ctx, cancel := context.WithCancel(context.Background())

	h.mu.Lock()
	h.activeStreams[req.RequestID] = cancel
	h.mu.Unlock()

	defer func() {
		h.mu.Lock()
		delete(h.activeStreams, req.RequestID)
		h.mu.Unlock()
	}()

	// Send start event
	conn.WriteJSON(StreamEvent{
		Type:      "start",
		RequestID: req.RequestID,
		Model:     req.Model,
	})

	// Get stream channel
	eventChan, err := h.chatFunc(ctx, req)
	if err != nil {
		conn.WriteJSON(StreamEvent{
			Type:      "error",
			RequestID: req.RequestID,
			Error:     err.Error(),
		})
		return
	}

	// Stream events
	for event := range eventChan {
		event.RequestID = req.RequestID
		if err := conn.WriteJSON(event); err != nil {
			cancel()
			return
		}
	}
}

// cancelStream cancels an active stream
func (h *StreamHandler) cancelStream(requestID string) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if cancel, ok := h.activeStreams[requestID]; ok {
		cancel()
		delete(h.activeStreams, requestID)
	}
}

// Handler returns an HTTP handler for WebSocket upgrades
func Handler(hub *Hub, handler *StreamHandler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Check for WebSocket upgrade
		if !strings.EqualFold(r.Header.Get("Upgrade"), "websocket") {
			http.Error(w, "Expected WebSocket upgrade", http.StatusBadRequest)
			return
		}

		conn, err := Upgrade(w, r)
		if err != nil {
			log.Printf("WebSocket upgrade failed: %v", err)
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		handler.HandleConnection(conn)
	}
}
