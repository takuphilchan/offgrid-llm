package rag

import (
	"context"
	"database/sql"
	"errors"

	"github.com/takuphilchan/offgrid-llm/internal/document"
)

// Bound simultaneous full-graph integrity checks, independently of ingestion.
// Waiting readers are cancellable and do not hold the engine/store locks.
var structureReaders = make(chan struct{}, 2)

func (v *KnowledgeView) Structure(ctx context.Context, id, revision, unit string, offset, limit, relationshipOffset int, navigationOffset ...int) (*document.StructurePage, error) {
	if offset < 0 || offset > 200000 || limit < 1 || limit > 200 || len(unit) > 128 || relationshipOffset < 0 || relationshipOffset > 400000 {
		return nil, ErrExtractionInvalid
	}
	select {
	case structureReaders <- struct{}{}:
	case <-ctx.Done():
		return nil, ctx.Err()
	}
	defer func() { <-structureReaders }()
	e := v.engine
	e.mu.RLock()
	defer e.mu.RUnlock()
	if err := e.documentAccess(v.principal, id, false); err != nil {
		return nil, err
	}
	store, ok := e.store.(*SQLiteStore)
	if !ok {
		return nil, ErrExtractionUnavailable
	}
	doc, err := store.GetDocument(id)
	if err != nil {
		return nil, err
	}
	if doc == nil {
		return nil, ErrCollectionUnavailable
	}
	if revision == "" {
		revision = doc.ExtractionRevision
	}
	if revision == "" {
		return nil, ErrExtractionUnavailable
	}
	graph, err := store.extraction(id, revision)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrExtractionUnavailable
	}
	if err != nil {
		return nil, err
	}
	if err = ctx.Err(); err != nil {
		return nil, err
	}
	canCorrect := revision == doc.ExtractionRevision && e.enabled && e.documentAccess(v.principal, id, true) == nil
	identity, found, identityErr := store.GetIndexMetadata()
	canCorrect = canCorrect && identityErr == nil && found && identity.SchemaVersion == IndexSchemaVersion && identity.EmbeddingModel == e.embeddingModel && identity.EmbeddingIdentity == e.embeddingIdentity
	page, err := document.ProjectStructure(graph, id, unit, offset, limit, relationshipOffset, canCorrect, navigationOffset...)
	if err != nil {
		return nil, ErrExtractionInvalid
	}
	if err = e.documentAccess(v.principal, id, false); err != nil {
		return nil, err
	}
	return page, nil
}
