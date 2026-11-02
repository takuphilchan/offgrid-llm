package server

import (
	"github.com/takuphilchan/offgrid-llm/internal/agents"
	"github.com/takuphilchan/offgrid-llm/internal/sessions"
	"github.com/takuphilchan/offgrid-llm/internal/users"
	"net/http"
)

func (s *Server) rejectUnavailableTask(w http.ResponseWriter, r *http.Request, task *agents.Task) bool {
	if s.taskKnowledgeAllowed(s.agentActor(r), task) {
		return false
	}
	writeJobError(w, 403, "knowledge_unavailable", "A source used by this task is no longer available.", false)
	return true
}

func (s *Server) taskKnowledgeAllowed(actor string, task *agents.Task) bool {
	if task == nil {
		return false
	}
	c := task.Config
	ids := append([]string(nil), c.KnowledgeDependencies...)
	for _, doc := range c.KnowledgeDocuments {
		ids = append(ids, doc.ID)
	}
	for _, source := range c.KnowledgeSources {
		ids = append(ids, source.DocumentID)
	}
	if c.KnowledgeContext != "" && len(ids) == 0 {
		return false
	}
	return s.savedKnowledgeAllowed(actor, ids)
}

// Session ownership and sessions:all do not grant access to retained source
// content. Recheck the current actor and collection, including after restart.
func (s *Server) savedKnowledgeAllowed(actor string, ids []string) bool {
	if len(ids) == 0 {
		return true
	}
	if s.config != nil && s.config.RequireAuth {
		if s.userStore == nil || !s.userStore.ActorHasPermission(actor, users.PermissionRAG) {
			return false
		}
	} else if actor == "local-admin" {
		actor = "local"
	}
	if s.ragEngine == nil {
		return false
	}
	view := s.ragEngine.ForActor(actor)
	seen := map[string]bool{}
	for _, id := range ids {
		if !seen[id] && !view.CanReadDocument(id) {
			return false
		}
		seen[id] = true
	}
	return true
}

func checkPassageAccess(scope *sessions.ScopedManager, passages []sessions.KnowledgePassage) error {
	ids := make([]string, 0, len(passages))
	for _, source := range passages {
		ids = append(ids, source.DocumentID)
	}
	return scope.CheckKnowledge(ids)
}
