package server

import (
	"encoding/json"
	"io"
	"net/http"

	"github.com/takuphilchan/offgrid-llm/internal/computer"
)

// Idempotent, actor-scoped revocation. Unlike emergency stop this cannot affect
// another session, including one acquired while a setup panel was open.
func (s *Server) handleComputerSessionStop(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJobError(w, 405, "method_not_allowed", "Use POST to revoke a computer session.", false)
		return
	}
	var req struct {
		Session string `json:"session_id"`
	}
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1024))
	decoder.DisallowUnknownFields()
	if decoder.Decode(&req) != nil || decoder.Decode(new(any)) != io.EOF || req.Session == "" || len(req.Session) > 128 {
		writeJobError(w, 400, "invalid_computer_session", "Provide the computer session to revoke.", false)
		return
	}
	if s.browserHub == nil {
		writeJobError(w, 503, "computer_unavailable", "Computer session service is unavailable.", true)
		return
	}
	s.browserHub.StopSession(s.agentActor(r), req.Session)
	writeJSON(w, 200, map[string]string{"status": "revoked"})
}

func (s *Server) handleComputerStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	// Compatibility status is a projection of the governed companion, not a
	// second controller with a blanket session-approved execution path.
	status := map[string]any{"available": false, "emergency_stop": true, "active_sessions": 0}
	if s.browserHub != nil {
		sessions := s.browserHub.List(s.agentActor(r))
		status["available"] = len(sessions) > 0
		status["active_sessions"] = len(sessions)
		if len(sessions) > 0 {
			status["emergency_stop"] = false
		}
	}
	writeJSON(w, http.StatusOK, status)
}

func (s *Server) handleComputerCapabilities(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", http.MethodGet)
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	capability := computer.UnavailableCapabilities()
	if s.browserHub != nil {
		for _, session := range s.browserHub.List(s.agentActor(r)) {
			for i := range capability.Drivers {
				if capability.Drivers[i].ID == session.Driver {
					capability.Drivers[i].Available = true
					capability.Available = true
					capability.ReasonCode = "browser_preview"
					if session.Target != nil {
						capability.ProtocolVersion = computer.ControlProtocolVersion
						capability.ReasonCode = "native_development"
					}
				}
			}
		}
	}
	writeJSON(w, http.StatusOK, capability)
}

// A client-supplied boolean must never become host-control authority.
func (s *Server) handleComputerUpgradeRequired(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusUpgradeRequired, map[string]any{"error": map[string]any{
		"code":      "computer_api_upgrade_required",
		"message":   "Legacy computer-control requests are disabled. Use supervised Computer Tasks with a compatible local companion.",
		"retryable": false,
	}})
}

func (s *Server) handleComputerStop(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if s.browserHub != nil {
		active := len(s.browserHub.List(s.agentActor(r))) > 0
		s.browserHub.Stop(s.agentActor(r))
		if active {
			writeJSON(w, http.StatusAccepted, map[string]string{"status": "stopping"})
			return
		}
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "stopped"})
}
