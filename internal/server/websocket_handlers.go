package server

import (
	"encoding/json"
	"log"
	"net/http"
	"net/url"
	"strings"

	"github.com/takuphilchan/offgrid-llm/internal/websocket"
)

func (s *Server) handleWebSocket(w http.ResponseWriter, r *http.Request) {
	if !s.isAllowedWebSocketOrigin(r) {
		http.Error(w, "Forbidden WebSocket origin", http.StatusForbidden)
		return
	}
	conn, err := websocket.Upgrade(w, r)
	if err != nil {
		log.Printf("WebSocket upgrade error: %v", err)
		return
	}

	// Register with hub
	s.wsHub.Register(conn)

	// Set up message handler
	conn.SetMessageHandler(func(msgType websocket.MessageType, data []byte) {
		// Process message
		var request map[string]any
		if err := json.Unmarshal(data, &request); err != nil {
			return
		}

		// Handle different message types
		reqType, _ := request["type"].(string)
		switch reqType {
		case "subscribe":
			channel, _ := request["channel"].(string)
			s.wsHub.Subscribe(conn, channel)
		case "unsubscribe":
			channel, _ := request["channel"].(string)
			s.wsHub.Unsubscribe(conn, channel)
		case "ping":
			conn.WriteJSON(map[string]string{"type": "pong"})
		}
	})

	// Set close handler to unregister from hub
	conn.SetCloseHandler(func() {
		s.wsHub.Unregister(conn)
	})

	// Start the read loop (blocking)
	conn.ReadLoop()
}

func (s *Server) isAllowedWebSocketOrigin(r *http.Request) bool {
	origin := r.Header.Get("Origin")
	if origin == "" {
		return true // Non-browser clients do not send Origin.
	}
	parsed, err := url.Parse(origin)
	if err != nil || parsed.Host == "" {
		return false
	}
	if strings.EqualFold(parsed.Host, r.Host) {
		return true
	}
	for _, allowed := range strings.Split(s.config.CORSOrigins, ",") {
		if strings.EqualFold(strings.TrimSpace(allowed), origin) {
			return true
		}
	}
	return false
}
