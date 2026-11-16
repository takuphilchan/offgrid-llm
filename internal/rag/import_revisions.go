package rag

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/takuphilchan/offgrid-llm/internal/document"
)

const maxImportRevisionBytes = 64 << 20

func encodeImportRevisions(history []*document.Extraction, digest string) ([]byte, error) {
	if len(history) == 0 {
		return nil, nil
	}
	if len(history) > 65 {
		return nil, ErrExtractionInvalid
	}
	parent := ""
	seen := map[string]bool{}
	for _, graph := range history {
		if graph == nil || graph.Verify() != nil || graph.OriginalSHA256 != digest || graph.ParentID != parent || seen[graph.ID] || strings.TrimSpace(graph.Text) != graph.Text {
			return nil, ErrExtractionInvalid
		}
		parent = graph.ID
		seen[parent] = true
	}
	data, err := json.Marshal(history)
	if err != nil {
		return nil, err
	}
	if len(data) > maxImportRevisionBytes {
		return nil, &ParseError{Code: "document_limit", Message: "The retained correction history exceeds the 64 MiB import limit."}
	}
	return data, nil
}

func (s *SQLiteStore) importRevisions(lease importLease, digest string) ([]*document.Extraction, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if collection, err := importAccess(s.db, lease.owner, lease.id, true); err != nil || collection != normalizedCollection(lease.collection) {
		return nil, ErrCollectionUnavailable
	}
	var version, size int
	var current, hash string
	err := s.db.QueryRow(`SELECT schema_version,current_revision,history_sha256,COALESCE(LENGTH(history),0) FROM document_import_revisions WHERE import_id=?`, lease.id).Scan(&version, &current, &hash, &size)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if version != 1 || size < 1 || size > maxImportRevisionBytes {
		return nil, ErrExtractionUnavailable
	}
	var data []byte
	err = s.db.QueryRow(`SELECT history FROM document_import_revisions WHERE import_id=? AND LENGTH(history)=? AND EXISTS(SELECT 1 FROM document_imports WHERE id=? AND owner=? AND attempt=? AND state='extracting')`, lease.id, size, lease.id, lease.owner, lease.attempt).Scan(&data)
	if err != nil {
		return nil, ErrImportConflict
	}
	var history []*document.Extraction
	if document.TextSHA256(string(data)) != hash || json.Unmarshal(data, &history) != nil || len(history) == 0 {
		return nil, ErrExtractionUnavailable
	}
	if _, err = encodeImportRevisions(history, digest); err != nil || history[len(history)-1].ID != current {
		return nil, ErrExtractionUnavailable
	}
	return history, nil
}

func revisionSourceRanges(graph *document.Extraction) ([]SourceRange, error) {
	var ranges []SourceRange
	for _, unit := range graph.Units {
		if unit.Span.End <= unit.Span.Start {
			continue
		}
		ranges = append(ranges, SourceRange{Kind: unit.Kind, Index: unit.Index, Name: unit.Name, Start: unit.Span.Start, End: unit.Span.End, SHA256: graph.OriginalSHA256, Revision: graph.ID})
	}
	doc := &Document{RawContent: graph.Text, Extraction: graph, SourceRanges: ranges}
	if err := correctSourceRanges(doc); err != nil {
		return nil, err
	}
	return doc.SourceRanges, nil
}

func revisionMetadata(graph *document.Extraction) map[string]string {
	metadata := map[string]string{"format": graph.Format, "extractor_revision": graph.ID, "extraction_method": graph.Method}
	if graph.Coverage != nil {
		data, _ := json.Marshal(graph.Coverage)
		metadata["extraction_coverage"] = string(data)
	}
	return metadata
}

func (e *Engine) ingestImportedRevisions(ctx context.Context, r *ImportRecord, data []byte, history []*document.Extraction, lease *importLease) (*Document, error) {
	graph := history[len(history)-1]
	ranges, err := revisionSourceRanges(graph)
	if err != nil {
		return nil, err
	}
	if len(ranges) == 0 {
		return nil, fmt.Errorf("corrected extraction contains no searchable source units")
	}
	original := &OriginalSource{importRef: lease, revisions: history[:len(history)-1], Name: r.Name, MediaType: "application/octet-stream", SHA256: r.SHA256, Bytes: data}
	metadata := revisionMetadata(graph)
	metadata["original_filename"] = r.Name
	return e.ingestText(ctx, r.Name, graph.Text, metadata, ranges, original.MediaType, original, graph)
}

func storeImportedAncestors(tx *sql.Tx, doc *Document) error {
	if doc.original == nil {
		return nil
	}
	for _, graph := range doc.original.revisions {
		ranges, err := revisionSourceRanges(graph)
		if err != nil {
			return err
		}
		prior := &Document{ID: doc.ID, RawContent: graph.Text, Extraction: graph, SourceRanges: ranges, Metadata: revisionMetadata(graph)}
		if err := storeExtraction(tx, prior); err != nil {
			return err
		}
	}
	return nil
}
