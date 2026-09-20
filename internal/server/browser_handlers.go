package server

import (
	"github.com/takuphilchan/offgrid-llm/internal/computer"
	"net/http"
	"strings"
)

func (s *Server) handleBrowserPairing(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	if r.Method != http.MethodPost {
		writeError(w, "Method not allowed", 405)
		return
	}
	if r.Header.Get("Sec-Fetch-Site") == "cross-site" || !strings.HasPrefix(r.Header.Get("Content-Type"), "application/json") {
		writeError(w, "JSON same-origin request required", 403)
		return
	}
	if s.browserHub == nil {
		writeError(w, "Companion service unavailable", 503)
		return
	}
	code, err := s.browserHub.PairCode(s.agentActor(r))
	if err != nil {
		writeError(w, "Pairing unavailable", 409)
		return
	}
	writeJSON(w, 200, map[string]any{"code": code, "expires_seconds": 120, "protocol_version": computer.ProtocolVersion})
}
func (s *Server) handleBrowserSessions(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	if r.Method != http.MethodGet {
		writeError(w, "Method not allowed", 405)
		return
	}
	sessions := []computer.BrowserSession{}
	if s.browserHub != nil {
		sessions = s.browserHub.List(s.agentActor(r))
		for i := range sessions {
			if sessions[i].RunID != "" && s.agentManager != nil {
				if task, ok := s.agentManager.GetTask(sessions[i].RunID); ok {
					switch task.Status {
					case "completed", "failed", "cancelled":
						sessions[i].State = "finished"
					}
				}
			}
		}
	}
	writeJSON(w, 200, map[string]any{"sessions": sessions})
}

// Only these exact routes bypass user authentication. They implement their own
// single-use enrollment / companion token authentication and expose no user API.
func (s *Server) browserTransport(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path
		if path != "/api/v2/computer/companion/pair" && path != "/api/v2/computer/companion/poll" && path != "/api/v2/computer/companion/reply" && path != "/api/v2/computer/companion/heartbeat" {
			next.ServeHTTP(w, r)
			return
		}
		w.Header().Set("Cache-Control", "no-store")
		if s.browserHub == nil {
			writeError(w, "Companion unavailable", 503)
			return
		}
		if r.Method != http.MethodPost {
			writeError(w, "Method not allowed", 405)
			return
		}
		if r.Header.Get("Origin") != "" || !strings.HasPrefix(r.Header.Get("Content-Type"), "application/json") {
			writeError(w, "Companion transport only", 403)
			return
		}
		r.Body = http.MaxBytesReader(w, r.Body, 64<<10)
		if path == "/api/v2/computer/companion/pair" {
			var req struct {
				Code     string `json:"code"`
				Origin   string `json:"origin"`
				Protocol int    `json:"protocol_version"`
			}
			if decodeJSON(r, &req) != nil {
				writeError(w, "Invalid pairing request", 400)
				return
			}
			session, token, err := s.browserHub.Pair(req.Code, req.Origin, req.Protocol)
			if err != nil {
				writeError(w, "Pairing expired, invalid or another session is active", 403)
				return
			}
			writeJSON(w, 200, map[string]any{"session": session, "token": token})
			return
		}
		token := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
		if path == "/api/v2/computer/companion/heartbeat" {
			if s.browserHub.Heartbeat(token) != nil {
				writeError(w, "Session revoked", 403)
				return
			}
			writeJSON(w, 200, map[string]bool{"active": true})
			return
		}
		if path == "/api/v2/computer/companion/poll" {
			action, err := s.browserHub.Poll(token)
			if err != nil {
				writeError(w, "Companion session expired or revoked", 403)
				return
			}
			writeJSON(w, 200, map[string]any{"action": action})
			return
		}
		var reply computer.BrowserReply
		if decodeJSON(r, &reply) != nil {
			writeError(w, "Invalid result", 400)
			return
		}
		if err := s.browserHub.Reply(token, reply); err != nil {
			writeError(w, "Result rejected", 409)
			return
		}
		writeJSON(w, 200, map[string]bool{"accepted": true})
	})
}
