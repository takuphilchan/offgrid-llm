package server

import (
	"net/http"

	"github.com/takuphilchan/offgrid-llm/internal/computer"
)

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
	if s.browserHub != nil && len(s.browserHub.List(s.agentActor(r))) > 0 {
		capability.Available = true
		capability.ReasonCode = "browser_preview"
		capability.Drivers[0].Available = true
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
