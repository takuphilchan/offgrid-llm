package rag

import "context"

// Identity is supplied by the authenticated service/job, never document metadata
// or model output. Old engine callers can see only the legacy shared workspace.
type knowledgePrincipal struct {
	actor      string
	sharedOnly bool
}
type knowledgePrincipalKey struct{}

func (p knowledgePrincipal) valid() bool { return p.sharedOnly || validKnowledgeActor(p.actor) }
func (p knowledgePrincipal) allows(policy *CollectionPolicy, write bool) bool {
	if policy == nil || !policy.valid() {
		return false
	}
	if p.sharedOnly {
		return policy.ID == WorkspaceCollection
	}
	return policy.Allows(p.actor, write)
}
func principalFromContext(ctx context.Context) knowledgePrincipal {
	if p, ok := ctx.Value(knowledgePrincipalKey{}).(knowledgePrincipal); ok {
		return p
	}
	return knowledgePrincipal{sharedOnly: true}
}

type KnowledgeView struct {
	engine    *Engine
	principal knowledgePrincipal
}

// ForActor binds all reads to current collection policy. It does not replace
// the service's Knowledge permissions or grant access to a referenced source.
func (e *Engine) ForActor(actor string) *KnowledgeView {
	return &KnowledgeView{engine: e, principal: knowledgePrincipal{actor: actor}}
}
func (e *Engine) viewForContext(ctx context.Context) *KnowledgeView {
	return &KnowledgeView{engine: e, principal: principalFromContext(ctx)}
}
func (v *KnowledgeView) context(ctx context.Context) context.Context {
	return context.WithValue(ctx, knowledgePrincipalKey{}, v.principal)
}
func (v *KnowledgeView) IsEnabled() bool { return v.principal.valid() && v.engine.IsEnabled() }
func (v *KnowledgeView) Search(ctx context.Context, query string, opts SearchOptions) (*RAGContext, error) {
	return v.engine.Search(v.context(ctx), query, opts)
}
func (v *KnowledgeView) EnhancePrompt(ctx context.Context, query string) (string, *RAGContext, error) {
	return v.engine.EnhancePrompt(v.context(ctx), query)
}
func (v *KnowledgeView) EnhancePromptForDocuments(ctx context.Context, query string, ids []string) (string, *RAGContext, error) {
	return v.engine.EnhancePromptForDocuments(v.context(ctx), query, ids)
}
func (v *KnowledgeView) ReindexDocument(ctx context.Context, id string) (*Document, error) {
	return v.engine.ReindexDocument(v.context(ctx), id)
}
func (v *KnowledgeView) Evaluate(ctx context.Context, cases []EvaluationCase) (EvaluationReport, error) {
	return v.engine.Evaluate(v.context(ctx), cases)
}

func (v *KnowledgeView) CanReadDocument(id string) bool {
	v.engine.mu.RLock()
	defer v.engine.mu.RUnlock()
	return v.engine.documentAccess(v.principal, id, false) == nil
}

func (v *KnowledgeView) Stats() map[string]interface{} {
	stats := v.engine.Stats()
	// Maintenance health is shared, but another collection's deletion count is not.
	status := v.engine.RetentionStatus()
	stats["retention"] = map[string]any{"checked_at": status.CheckedAt, "error_code": status.ErrorCode}
	docs := v.ListDocuments()
	chunks, retained := 0, 0
	for _, doc := range docs {
		chunks += doc.ChunkCount
		if doc.SourceRetained {
			retained++
		}
	}
	stats["document_count"], stats["chunk_count"], stats["reindexable_document_count"] = len(docs), chunks, retained
	return stats
}

func (e *Engine) documentAccess(p knowledgePrincipal, id string, write bool) error {
	if !p.valid() || e.store == nil {
		return ErrCollectionUnavailable
	}
	if s, ok := e.store.(*SQLiteStore); ok {
		s.mu.RLock()
		defer s.mu.RUnlock()
		policy, _, err := documentPolicy(s.db, id)
		if err != nil || !p.allows(policy, write) {
			return ErrCollectionUnavailable
		}
	}
	return nil // Legacy stores have no private-collection capability.
}

func (v *KnowledgeView) GetDocument(id string) *Document {
	e := v.engine
	e.mu.RLock()
	defer e.mu.RUnlock()
	if e.documentAccess(v.principal, id, false) != nil {
		return nil
	}
	doc, err := e.store.GetDocument(id)
	if err != nil || e.documentAccess(v.principal, id, false) != nil {
		return nil
	}
	return doc
}

func (v *KnowledgeView) ListDocuments() []*Document {
	e := v.engine
	e.mu.RLock()
	defer e.mu.RUnlock()
	if e.store == nil || !v.principal.valid() {
		return []*Document{}
	}
	docs, err := e.store.ListDocuments()
	if err != nil {
		return []*Document{}
	}
	if s, ok := e.store.(*SQLiteStore); ok {
		ids, err := s.authorizedDocumentIDs(v.principal, nil)
		if err != nil {
			return []*Document{}
		}
		allowed := make(map[string]bool, len(ids))
		for _, id := range ids {
			allowed[id] = true
		}
		filtered := make([]*Document, 0, len(ids))
		for _, doc := range docs {
			if allowed[doc.ID] {
				filtered = append(filtered, doc)
			}
		}
		return filtered
	}
	return docs
}

func (v *KnowledgeView) Original(id string) (*OriginalSource, error) {
	e := v.engine
	e.mu.RLock()
	defer e.mu.RUnlock()
	if err := e.documentAccess(v.principal, id, false); err != nil {
		return nil, err
	}
	s, ok := e.store.(*SQLiteStore)
	if !ok {
		return nil, ErrOriginalUnavailable
	}
	original, err := s.original(id)
	if err != nil {
		return nil, err
	}
	if err := e.documentAccess(v.principal, id, false); err != nil {
		return nil, err
	}
	return original, nil
}

func (v *KnowledgeView) DeleteDocument(id string) bool {
	return v.RemoveDocument(id) == nil
}

// RemoveDocument preserves typed hold/storage failures for authenticated clients.
func (v *KnowledgeView) RemoveDocument(id string) error {
	e := v.engine
	e.mu.Lock()
	defer e.mu.Unlock()
	if !v.principal.valid() || e.store == nil {
		return ErrCollectionUnavailable
	}
	if s, ok := e.store.(*SQLiteStore); ok {
		s.mu.Lock()
		defer s.mu.Unlock()
		policy, _, err := documentPolicy(s.db, id)
		if err != nil || !v.principal.allows(policy, true) {
			return ErrCollectionUnavailable
		}
		return s.deleteDocument(id)
	}
	return e.store.DeleteDocument(id)
}
