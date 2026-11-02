package rag

import (
	"context"
	"fmt"
)

// EnhancePromptForDocuments never broadens a requested source scope. Recheck
// identities after retrieval as deletion/reindexing can race a query.
func (e *Engine) EnhancePromptForDocuments(ctx context.Context, query string, ids []string) (string, *RAGContext, error) {
	if len(ids) == 0 || len(ids) > 16 {
		return query, nil, fmt.Errorf("invalid document selection")
	}
	before := map[string]string{}
	for _, id := range ids {
		doc := e.viewForContext(ctx).GetDocument(id)
		if doc == nil || doc.IndexStatus != "ready" || doc.ChunkCount == 0 {
			return query, nil, fmt.Errorf("selected document unavailable")
		}
		before[id] = doc.ContentHash
	}
	opts := DefaultSearchOptions()
	opts.TopK, opts.MinScore, opts.DocumentFilter = 5, 0.20, append([]string(nil), ids...)
	result, err := e.Search(ctx, query, opts)
	if err != nil {
		return query, nil, err
	}
	for id, hash := range before {
		doc := e.viewForContext(ctx).GetDocument(id)
		if doc == nil || doc.IndexStatus != "ready" || doc.ContentHash != hash {
			return query, nil, fmt.Errorf("selected document changed during retrieval")
		}
	}
	for _, item := range result.Results {
		if _, allowed := before[item.DocumentID]; !allowed {
			return query, nil, fmt.Errorf("retrieval exceeded selected documents")
		}
	}
	result.TruncateContext(e.maxContextLen)
	return result.Context + "User question: " + query, result, nil
}
