package rag

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/takuphilchan/offgrid-llm/internal/document"
)

var ErrExtractionConflict = errors.New("document extraction changed; reload before correcting")
var ErrExtractionUnavailable = errors.New("structured extraction unavailable; rebuild the retained document first")
var ErrExtractionInvalid = errors.New("invalid extraction review request")
var ErrExtractionIndexUnavailable = errors.New("configured embedding identity does not match this index")

func (v *KnowledgeView) CorrectDocument(ctx context.Context, id, expected string, edits []document.Correction) (*Document, error) {
	return v.engine.correctDocument(v.context(ctx), id, expected, edits, v.engine.generateEmbeddings)
}

func (e *Engine) correctDocument(ctx context.Context, id, expected string, edits []document.Correction, embed func(context.Context, []string) ([][]float32, error)) (*Document, error) {
	if err := e.ingestMu.acquire(ctx); err != nil {
		return nil, err
	}
	defer e.ingestMu.release()
	e.mu.RLock()
	defer e.mu.RUnlock()
	p := principalFromContext(ctx)
	if !validKnowledgeActor(p.actor) || e.documentAccess(p, id, true) != nil {
		return nil, ErrCollectionUnavailable
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
	if doc.ExtractionRevision == "" {
		return nil, ErrExtractionUnavailable
	}
	if expected == "" || doc.ExtractionRevision != expected {
		return nil, ErrExtractionConflict
	}
	parent, err := store.extraction(id, expected)
	if err != nil {
		return nil, err
	}
	derived, err := parent.Correct(p.actor, time.Now().UTC(), edits)
	if err != nil {
		return nil, errors.Join(ErrExtractionInvalid, err)
	}
	identity, found, err := store.GetIndexMetadata()
	if err != nil {
		return nil, err
	}
	if !e.enabled || !found || identity.SchemaVersion != IndexSchemaVersion || identity.EmbeddingModel != e.embeddingModel || identity.EmbeddingIdentity != e.embeddingIdentity {
		return nil, ErrExtractionIndexUnavailable
	}
	doc.Extraction = derived
	doc.RawContent = derived.Text
	doc.expectedExtractionRevision = expected
	if err = correctSourceRanges(doc); err != nil {
		return nil, errors.Join(ErrExtractionInvalid, err)
	}
	chunks, err := e.chunker.chunkDocument(doc)
	if err != nil {
		return nil, err
	}
	if len(chunks) == 0 {
		return nil, fmt.Errorf("correction leaves no indexable text")
	}
	vectors, err := store.changedEmbeddings(ctx, doc.ID, chunks, identity.EmbeddingDim, embed)
	if err != nil {
		return nil, err
	}
	doc.ChunkCount = len(chunks)
	doc.Size = int64(len(doc.RawContent))
	doc.ContentHash = GenerateContentHash(doc.RawContent)
	doc.IndexStatus = "ready"
	doc.LastError = ""
	doc.IndexedAt = time.Now().UTC()
	doc.UpdatedAt = doc.IndexedAt
	if doc.Metadata == nil {
		doc.Metadata = map[string]string{}
	}
	doc.Metadata["extractor_revision"] = derived.ID
	doc.Metadata["extraction_method"] = "review"
	identity.ChunkerVersion = chunkerVersion
	identity.UpdatedAt = doc.IndexedAt
	if err = ctx.Err(); err != nil {
		return nil, err
	}
	if err = store.replaceDocumentWithChunks(doc, chunks, vectors, &identity, p); err != nil {
		return nil, err
	}
	if err = e.documentAccess(p, id, false); err != nil {
		return nil, err
	}
	return doc, nil
}

func correctSourceRanges(doc *Document) error {
	ranges := make([]SourceRange, 0, len(doc.SourceRanges))
	for _, prior := range doc.SourceRanges {
		found := false
		for _, u := range doc.Extraction.Units {
			if u.Kind != prior.Kind || u.Index != prior.Index {
				continue
			}
			r := prior
			r.Start = u.Span.Start
			r.End = u.Span.End
			r.Revision = doc.Extraction.ID
			r.Cells = nil
			for _, b := range doc.Extraction.Blocks {
				if b.UnitID == u.ID && b.Cell != nil {
					r.Cells = append(r.Cells, SourceCell{Reference: b.Cell.Reference, Start: b.Span.Start, End: b.Span.End})
				}
			}
			if r.End > r.Start {
				ranges = append(ranges, r)
			}
			found = true
			break
		}
		if !found {
			return fmt.Errorf("corrected source unit unavailable")
		}
	}
	if err := validateSourceRanges(doc.RawContent, ranges); err != nil {
		return err
	}
	doc.SourceRanges = ranges
	return nil
}

// Reuse embeddings only for identical text under the same verified model
// identity. Offset shifts do not require embedding unchanged paragraphs again.
func (s *SQLiteStore) changedEmbeddings(ctx context.Context, id string, chunks []*Chunk, dimension int, embed func(context.Context, []string) ([][]float32, error)) ([][]float32, error) {
	vectors := make([][]float32, len(chunks))
	pending := []int{}
	for i, c := range chunks {
		var text string
		var binary, legacy []byte
		err := s.db.QueryRowContext(ctx, `SELECT content,embedding_v1,embedding FROM chunks WHERE document_id=? AND content_hash=? AND content=? LIMIT 1`, id, c.ContentHash, c.Content).Scan(&text, &binary, &legacy)
		if err == nil {
			if len(binary) > 0 {
				vectors[i], err = decodeVector(binary)
			} else {
				err = json.Unmarshal(legacy, &vectors[i])
			}
			if err != nil || len(vectors[i]) != dimension {
				return nil, fmt.Errorf("retained embedding is invalid")
			}
		} else if errors.Is(err, sql.ErrNoRows) {
			pending = append(pending, i)
		} else {
			return nil, err
		}
	}
	for start := 0; start < len(pending); start += 32 {
		batch := pending[start:min(start+32, len(pending))]
		texts := make([]string, len(batch))
		for j, i := range batch {
			texts[j] = chunks[i].Content
		}
		generated, err := embed(ctx, texts)
		if err != nil {
			return nil, err
		}
		if len(generated) != len(batch) {
			return nil, fmt.Errorf("embedding response count mismatch")
		}
		for j, i := range batch {
			if len(generated[j]) != dimension {
				return nil, fmt.Errorf("embedding dimension mismatch")
			}
			vectors[i] = generated[j]
		}
	}
	for i, c := range chunks {
		if _, err := decodeVector(encodeVector(vectors[i])); err != nil {
			return nil, err
		}
		c.Embedding = vectors[i]
		c.CreatedAt = time.Now().UTC()
	}
	return vectors, nil
}
