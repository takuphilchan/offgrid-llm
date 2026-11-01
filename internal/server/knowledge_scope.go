package server

import (
	"context"
	"net/http"
	"strings"

	"github.com/takuphilchan/offgrid-llm/internal/rag"
	"github.com/takuphilchan/offgrid-llm/internal/sessions"
)

type knowledgeScopeKey struct{}
type knowledgeScope struct {
	documents []string
	passages  *[]sessions.KnowledgePassage
}

func withKnowledgeScope(ctx context.Context, ids []string, passages *[]sessions.KnowledgePassage) context.Context {
	return context.WithValue(ctx, knowledgeScopeKey{}, knowledgeScope{documents: ids, passages: passages})
}

func validateKnowledgeSelection(req *sessionGenerateRequest) error {
	if req.KnowledgeDocuments == nil {
		return nil
	}
	if !req.UseKnowledgeBase || len(req.KnowledgeDocuments) == 0 || len(req.KnowledgeDocuments) > 16 {
		return newServiceError(http.StatusBadRequest, "Select between one and sixteen documents with Knowledge enabled", nil)
	}
	seen := map[string]bool{}
	for _, id := range req.KnowledgeDocuments {
		if id == "" || strings.TrimSpace(id) != id || len(id) > 256 || strings.ContainsAny(id, "\x00\r\n") || seen[id] {
			return newServiceError(http.StatusBadRequest, "Invalid or duplicate selected document", nil)
		}
		seen[id] = true
	}
	if req.Agent != nil {
		restrictions := agentResourceRestrictions{}
		if req.Restrictions != nil {
			restrictions = *req.Restrictions
		}
		if restrictions.Knowledge != nil {
			allowed := map[string]bool{}
			for _, id := range *restrictions.Knowledge {
				allowed[id] = true
			}
			for id := range seen {
				if !allowed[id] {
					return newServiceError(http.StatusForbidden, "Selected documents exceed this turn's Knowledge restrictions", nil)
				}
			}
		}
		ids := append([]string(nil), req.KnowledgeDocuments...)
		restrictions.Knowledge = &ids
		req.Restrictions = &restrictions
	}
	return nil
}

type scopedKnowledgeRetriever interface {
	EnhancePromptForDocuments(context.Context, string, []string) (string, *rag.RAGContext, error)
}

func retrieveKnowledge(ctx context.Context, source knowledgeRetriever, query string) (*rag.RAGContext, error) {
	scope, _ := ctx.Value(knowledgeScopeKey{}).(knowledgeScope)
	if scope.documents != nil {
		filtered, ok := source.(scopedKnowledgeRetriever)
		if !ok {
			return nil, newServiceError(http.StatusServiceUnavailable, "Selected-source retrieval is unavailable on this runtime", nil)
		}
		_, result, err := filtered.EnhancePromptForDocuments(ctx, query, scope.documents)
		if err == nil && result != nil {
			allowed := map[string]bool{}
			for _, id := range scope.documents {
				allowed[id] = true
			}
			for _, item := range result.Results {
				if !allowed[item.DocumentID] {
					return nil, newServiceError(http.StatusServiceUnavailable, "Retrieved source exceeded the requested scope", nil)
				}
			}
		}
		return result, err
	}
	_, result, err := source.EnhancePrompt(ctx, query)
	return result, err
}

func captureKnowledge(ctx context.Context, result *rag.RAGContext) {
	scope, _ := ctx.Value(knowledgeScopeKey{}).(knowledgeScope)
	if scope.passages == nil {
		return
	}
	passages := make([]sessions.KnowledgePassage, 0, len(result.Results))
	for _, item := range result.Results {
		if item.Chunk == nil {
			continue
		}
		passages = append(passages, sessions.KnowledgePassage{DocumentID: item.DocumentID, Name: item.DocName, ChunkID: item.Chunk.ID, Content: item.Chunk.Content, ContentHash: rag.GenerateContentHash(item.Chunk.Content)})
	}
	*scope.passages = passages
}
