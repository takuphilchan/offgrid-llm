package server

import (
	"context"
	"net/http"

	"github.com/takuphilchan/offgrid-llm/internal/inference"
	"github.com/takuphilchan/offgrid-llm/internal/sessions"
	"github.com/takuphilchan/offgrid-llm/internal/users"
	"github.com/takuphilchan/offgrid-llm/pkg/api"
)

// withChatKnowledgeGuard preserves the session's selected documents and passage
// capture while also protecting direct API requests that have no saved session.
// Retrieval fills the capture before any generation guard is evaluated.
func (s *Server) withChatKnowledgeGuard(ctx context.Context, request *api.ChatCompletionRequest) context.Context {
	if request.UseKnowledgeBase == nil || !*request.UseKnowledgeBase {
		return ctx
	}
	scope, _ := ctx.Value(knowledgeScopeKey{}).(knowledgeScope)
	if scope.passages == nil {
		scope.passages = new([]sessions.KnowledgePassage)
	}
	ctx = withKnowledgeScope(ctx, scope.documents, scope.passages)
	actor := "local"
	if user := users.UserFromContext(ctx); user != nil {
		actor = user.ID
	}
	return inference.WithGenerationGuard(ctx, func() error {
		ids := make([]string, 0, len(*scope.passages))
		for _, passage := range *scope.passages {
			ids = append(ids, passage.DocumentID)
		}
		if len(ids) == 0 || !s.savedKnowledgeAllowed(actor, ids) {
			return newCodedServiceError(http.StatusForbidden, "knowledge_unavailable", "A source used by this request is no longer available. No further answer was released.", nil)
		}
		return nil
	})
}
