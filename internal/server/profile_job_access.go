package server

import (
	"net/http"
	"strings"

	"github.com/takuphilchan/offgrid-llm/internal/agents"
	"github.com/takuphilchan/offgrid-llm/internal/capabilities"
	"github.com/takuphilchan/offgrid-llm/internal/execution"
	"github.com/takuphilchan/offgrid-llm/internal/users"
)

// Chat permission is not service-filesystem or shared-credential authority.
// Until a provider has an actor-specific use grant, its administrator-owned
// resources remain administrator-only. Pure built-ins need no external grant.
func profileToolAllowed(admin bool, d capabilities.Descriptor) bool {
	if admin {
		return true
	}
	return d.Source == "builtin" && d.Namespace == "tools" && d.Kind == capabilities.Read &&
		(d.Name == "calculator" || d.Name == "current_time")
}

func (s *Server) profileActorCapabilityAllowed(actor string, d capabilities.Descriptor) bool {
	if s.config == nil || !s.config.RequireAuth {
		return true
	}
	if s.userStore == nil || !s.userStore.ActorHasPermission(actor, users.PermissionChat) {
		return false
	}
	if s.userStore.ActorHasPermission(actor, users.PermissionAdmin) {
		return true
	}
	if d == execution.AccessRequestDescriptor() {
		return true
	}
	if expected, ok := executionDescriptor(d.Name); ok && expected == d {
		return true
	}
	if expected, ok := contextDescriptor(d.Name); ok && expected == d {
		return true
	}
	// Artifacts are scoped by the canonical owner/job, not an arbitrary path.
	return d == taskArtifactDescriptor() || profileToolAllowed(false, d)
}

// Authentication/Chat permission is enforced by the surrounding route. Keep
// legacy unrestricted tasks administrative while exposing owned profile work.
func (s *Server) profileJobAccess(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if s.profileAdmin(r) {
			next(w, r)
			return
		}
		id := strings.Split(strings.TrimPrefix(r.URL.Path, "/api/v2/jobs/"), "/")[0]
		if s.agentManager == nil {
			writeJobReadError(w, agents.ErrRunStorage)
			return
		}
		task, ok := s.agentManager.GetTask(id)
		if !ok || task.Actor != s.agentActor(r) || task.Config.Profile == nil {
			writeJobReadError(w, agents.ErrTaskNotFound)
			return
		}
		workspaceInput := strings.HasSuffix(r.URL.Path, "/input") && (task.PendingInput != nil && task.PendingInput.Kind == "workspace" || task.Config.ExecutionRequestVersion == execution.AccessRequestVersion && task.Config.Execution != nil)
		if r.Method != http.MethodGet && (strings.HasSuffix(r.URL.Path, "/input") && !workspaceInput || strings.HasSuffix(r.URL.Path, "/reconnect")) {
			writeJobError(w, 403, "computer_administration_required", "Computer access administration requires an administrator.", false)
			return
		}
		next(w, r)
	}
}
