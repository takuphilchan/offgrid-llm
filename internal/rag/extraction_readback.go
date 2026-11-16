package rag

import (
	"encoding/json"
	"fmt"
	"unicode/utf8"
)

// historicalPassage uses the original citation's digest, parser revision, byte
// range and content hash. It never substitutes the newest revision's prose.
// The enclosing KnowledgeView authorizes before and after this read.
func (s *SQLiteStore) historicalPassage(id string, start, end int, digest, parserRevision, hash string) (*Document, string, error) {
	if start < 0 || end <= start || end-start > 800000 || len(digest) != 64 || len(hash) != 32 || len(parserRevision) == 0 || len(parserRevision) > 512 {
		return nil, "", fmt.Errorf("invalid citation")
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	rows, err := s.db.Query(`SELECT revision,source_ranges,metadata,LENGTH(CAST(raw_text AS BLOB)),CAST(substr(CAST(raw_text AS BLOB),?,?) AS TEXT) FROM document_extractions WHERE document_id=? AND source_sha256=? AND schema_version=1 AND LENGTH(CAST(raw_text AS BLOB))>=? ORDER BY created_at DESC,revision`, start+1, end-start, id, digest, end)
	if err != nil {
		return nil, "", err
	}
	defer rows.Close()
	for rows.Next() {
		var revision, rangesJSON, metadata, text string
		var size int64
		if err = rows.Scan(&revision, &rangesJSON, &metadata, &size, &text); err != nil {
			return nil, "", err
		}
		if len(text) != end-start || !utf8.ValidString(text) || GenerateContentHash(text) != hash {
			continue
		}
		var ranges []SourceRange
		if err = json.Unmarshal([]byte(rangesJSON), &ranges); err != nil {
			return nil, "", err
		}
		for _, r := range ranges {
			if r.Start <= start && r.End >= end && r.SHA256 == digest && r.Revision == parserRevision {
				d := &Document{ID: id, ExtractionRevision: revision, SourceRetained: true, SourceRanges: ranges, Size: size, IndexStatus: "historical"}
				if err = json.Unmarshal([]byte(metadata), &d.Metadata); err != nil {
					return nil, "", err
				}
				return d, text, nil
			}
		}
	}
	if err = rows.Err(); err != nil {
		return nil, "", err
	}
	return nil, "", fmt.Errorf("source passage no longer matches its saved citation")
}

// ReadSourcePassage includes retained historical revisions under today's
// collection membership. Revocation and deletion apply to every revision.
func (v *KnowledgeView) ReadSourcePassage(id string, start, end int, digest, revision, hash string) (*Document, string, error) {
	e := v.engine
	e.mu.RLock()
	defer e.mu.RUnlock()
	if err := e.documentAccess(v.principal, id, false); err != nil {
		return nil, "", err
	}
	doc, err := e.store.GetDocument(id)
	if err != nil {
		return nil, "", err
	}
	if doc == nil {
		return nil, "", ErrCollectionUnavailable
	}
	text, err := doc.ReadSourcePassage(start, end, digest, revision, hash)
	if err != nil {
		if store, ok := e.store.(*SQLiteStore); ok {
			var old *Document
			old, text, err = store.historicalPassage(id, start, end, digest, revision, hash)
			if err == nil {
				old.Name, old.ContentType, old.OriginalRetained = doc.Name, doc.ContentType, doc.OriginalRetained
				doc = old
			}
		}
	}
	if err != nil {
		return nil, "", err
	}
	if err = e.documentAccess(v.principal, id, false); err != nil {
		return nil, "", err
	}
	return doc, text, nil
}
