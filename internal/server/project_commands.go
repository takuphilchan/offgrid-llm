package server

import (
	"net/http"
	"strings"

	"github.com/takuphilchan/offgrid-llm/internal/agents"
	"github.com/takuphilchan/offgrid-llm/pkg/api"
)

func (s *Server) handleProjectCommand(w http.ResponseWriter, r *http.Request) {
	parts := strings.Split(strings.Trim(strings.TrimPrefix(r.URL.Path, "/api/v2/projects"), "/"), "/")
	id := parts[0]
	if id != "" && (len(parts) != 2 || parts[1] != "commands") {
		writeControlError(w, agents.ErrControlCommand)
		return
	}
	var command api.ProjectCommand
	if decodeControlEnvelope(w, r, &command) != nil || command.TargetID != id {
		writeControlError(w, agents.ErrControlCommand)
		return
	}
	receipt, err := s.agentManager.ApplyProjectCommand(s.controlScope(r), command)
	if err != nil {
		writeControlError(w, err)
		return
	}
	w.Header().Set("Location", "/api/v2/projects/"+receipt.TargetID)
	writeJSON(w, http.StatusAccepted, receipt)
}
