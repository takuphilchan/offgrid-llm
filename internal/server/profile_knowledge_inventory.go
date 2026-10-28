package server

import (
	"net/http"

	"github.com/takuphilchan/offgrid-llm/internal/agents"
)

// Calendar review records resource identity without doing the scheduled search
// early. Ordinary task admission keeps its existing bounded retrieval path.
func (s *Server) attachJobKnowledge(w http.ResponseWriter, r *http.Request, request *jobSubmitRequest, config *agents.AgentConfig) bool {
	if !request.DeferKnowledge {
		return s.attachProfileKnowledge(w, r, request.Prompt, config)
	}
	config.KnowledgeContext = ""
	config.KnowledgeDocuments = nil
	ids := selectedProfileKnowledge(*config)
	if len(ids) == 0 {
		return true
	}
	if s.ragEngine == nil || !s.ragEngine.IsEnabled() || s.ragEngine.StorageError() != nil {
		writeJobError(w, 503, "knowledge_unavailable", "Enable Knowledge before reviewing this schedule.", true)
		return false
	}
	for _, id := range ids {
		doc := s.ragEngine.GetDocument(id)
		if doc == nil {
			writeJobError(w, 422, "knowledge_unavailable", "An attached Knowledge document is unavailable. No schedule was saved.", false)
			return false
		}
		config.KnowledgeDocuments = append(config.KnowledgeDocuments, agents.KnowledgeDocument{ID: id, Name: doc.Name})
	}
	return true
}
