package rag

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/takuphilchan/offgrid-llm/internal/document"
)

// SelectEvidence resolves explicit immutable source scope without an embedding
// model. It never substitutes the current revision or drops a requested block.
func (v *KnowledgeView) SelectEvidence(ctx context.Context, id, revision string, page int, blocks []string, budget int) (*RAGContext, error) {
	if id == "" || revision == "" || len(revision) > 128 || page < 0 || page > 100000 || len(blocks) > 32 || (page > 0 && len(blocks) > 0) || (page == 0 && len(blocks) == 0) || budget < 1 {
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
		return nil, ErrExtractionUnavailable
	}
	graph, err := store.extraction(id, revision)
	if err != nil {
		return nil, ErrExtractionUnavailable
	}
	var raw string
	if err = store.db.QueryRowContext(ctx, `SELECT source_ranges FROM document_extractions WHERE document_id=? AND revision=? AND LENGTH(CAST(source_ranges AS BLOB))<=?`, id, revision, 16<<20).Scan(&raw); err != nil {
		return nil, err
	}
	var ranges []SourceRange
	if json.Unmarshal([]byte(raw), &ranges) != nil || validateSourceRanges(graph.Text, ranges) != nil {
		return nil, ErrExtractionInvalid
	}
	spans := []document.Span{}
	if page > 0 {
		for _, u := range graph.Units {
			if u.Kind == "page" && u.Index == page {
				spans = append(spans, u.Span)
				break
			}
		}
	} else {
		available := map[string]document.Block{}
		for _, b := range graph.Blocks {
			available[b.ID] = b
		}
		seen := map[string]bool{}
		for _, id := range blocks {
			b, found := available[id]
			if !found || seen[id] {
				return nil, ErrExtractionInvalid
			}
			seen[id] = true
			spans = append(spans, b.Span)
		}
	}
	if len(spans) == 0 {
		return nil, ErrExtractionUnavailable
	}
	metadata := map[string]string{}
	for k, v := range doc.Metadata {
		metadata[k] = v
	}
	if graph.Coverage != nil {
		b, _ := json.Marshal(graph.Coverage)
		metadata["extraction_coverage"] = string(b)
	}
	result := &RAGContext{RetrievalMode: "selected-evidence"}
	index := indexEvidence(graph, ranges)
	seen := map[string]bool{}
	for _, span := range spans {
		if err = ctx.Err(); err != nil {
			return nil, err
		}
		if span.End <= span.Start || span.Start < 0 || span.End > len(graph.Text) {
			return nil, ErrExtractionUnavailable
		}
		var source *SourceRange
		for _, r := range ranges {
			if r.Start <= span.Start && r.End >= span.End {
				source = sourceForSpan(r, span.Start, span.End)
				break
			}
		}
		if source == nil {
			return nil, ErrExtractionUnavailable
		}
		text := graph.Text[span.Start:span.End]
		chunk := &Chunk{ID: "selected-" + GenerateContentHash(fmt.Sprintf("%s:%d:%d", revision, span.Start, span.End)), DocumentID: id, Content: text, StartChar: span.Start, EndChar: span.End, ContentHash: GenerateContentHash(text), Source: source, Section: source.Name}
		if source.Kind == "page" {
			chunk.Page = source.Index
		}
		group, err := index.expand(SearchResult{Chunk: chunk, DocumentID: id, DocName: doc.Name, Metadata: metadata, ExtractionRevision: revision}, budget)
		if err != nil {
			return nil, err
		}
		for _, r := range group {
			if !r.EvidenceReceipt.LinksComplete {
				return nil, ErrExtractionUnavailable
			}
			if !seen[r.Chunk.ID] {
				result.Results = append(result.Results, r)
				seen[r.Chunk.ID] = true
			}
		}
	}
	result.FormatContext()
	if len(result.Context) > budget {
		return nil, ErrExtractionInvalid
	}
	if err = e.documentAccess(v.principal, id, false); err != nil {
		return nil, err
	}
	return result, nil
}
