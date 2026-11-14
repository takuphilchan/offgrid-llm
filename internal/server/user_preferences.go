package server

import (
	"net/http"

	"github.com/takuphilchan/offgrid-llm/internal/agents"
	"github.com/takuphilchan/offgrid-llm/pkg/api"
)

func (s *Server) handleUserPreferences(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	if s.agentManager == nil {
		writeControlError(w, agents.ErrRunStorage)
		return
	}
	scope := s.controlScope(r)
	if r.Method == http.MethodGet && r.URL.Path == "/api/v2/user-preferences" {
		preferences, err := s.agentManager.UserPreferences(scope)
		if err != nil {
			writeControlError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, preferences)
		return
	}
	if r.Method == http.MethodPost && r.URL.Path == "/api/v2/user-preferences/commands" {
		var command api.UserPreferencesCommand
		if decodeControlEnvelope(w, r, &command) != nil {
			writeControlError(w, agents.ErrControlCommand)
			return
		}
		receipt, err := s.agentManager.ConfigureUserPreferences(scope, command)
		if err != nil {
			writeControlError(w, err)
			return
		}
		writeJSON(w, http.StatusAccepted, receipt)
		return
	}
	writeJobError(w, http.StatusMethodNotAllowed, "method_not_allowed", "Use GET to inspect preferences or POST commands to change them.", false)
}
