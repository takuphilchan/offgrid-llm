package server

import (
	"github.com/takuphilchan/offgrid-llm/internal/execution"
	"net/http"
)

// This catalog is observation-only. Folder authority is supplied by an
// independently approved host, never by POSTing a folder path to this route.
func (s *Server) handleExecutionWorkspaces(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	if r.Method != http.MethodGet {
		writeJobError(w, 405, "method_not_allowed", "Inspect execution workspaces with GET; select folders on the trusted local host.", false)
		return
	}
	resources := []execution.Resource{}
	if s.executionHosts != nil {
		resources = s.executionHosts.List(r.Context(), s.agentActor(r), s.workspaceID)
	}
	writeJSON(w, 200, map[string]any{"workspaces": resources, "protocol": execution.ProtocolVersion})
}
