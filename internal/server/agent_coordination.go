package server

import (
	"github.com/google/uuid"
	"net/http"
)

// The old in-memory orchestrator does not carry actor-scoped durable approvals
// or recoverable child jobs. Never expose it as a second execution authority.
// Bounded delegation uses /api/v2/jobs and delegate_tasks, not this legacy engine.
func handleLegacyCoordination(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Request-ID", uuid.NewString())
	switch r.Method {
	case http.MethodGet:
		writeJSON(w, 200, map[string]any{"available": false, "workflows": []any{}, "modes": []string{}, "code": "durable_coordination_unavailable", "replacement": "/api/v2/jobs", "message": "This legacy in-memory workflow engine is retired. Task-first jobs support bounded read-only delegation, linked child tasks and explicit recovery through /api/v2/jobs. Arbitrary workflow registration is not supported."})
	case http.MethodPost:
		writeJobError(w, 501, "durable_coordination_unavailable", "This legacy workflow route is retired. Use /api/v2/jobs for durable tasks and bounded read-only delegation. Nothing was registered or executed.", false)
	default:
		writeJobError(w, 405, "method_not_allowed", "Use GET to inspect coordination availability.", false)
	}
}
