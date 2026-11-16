package rag

import (
	"database/sql"
	"encoding/json"
	"fmt"

	"github.com/takuphilchan/offgrid-llm/internal/document"
)

func checkExtractionQuota(tx *sql.Tx, limit int64) error {
	var used int64
	if err := tx.QueryRow(`SELECT COALESCE(SUM(storage_bytes),0) FROM document_extractions`).Scan(&used); err != nil {
		return err
	}
	if used > limit {
		return &ParseError{Code: "document_storage_limit", Message: "Retained document extractions have reached the 2 GiB storage limit. Remove unneeded documents before creating more revisions."}
	}
	return nil
}

// Store graph projections in the same transaction as the active index. All
// history belongs to the document's current access and retention boundary.
func storeExtraction(tx *sql.Tx, doc *Document) error {
	if doc.RawContent == "" {
		return nil
	}
	if err := doc.ensureExtraction(); err != nil {
		return err
	}
	e := doc.Extraction
	if e.ParentID != "" {
		var source string
		if err := tx.QueryRow(`SELECT source_sha256 FROM document_extractions WHERE document_id=? AND revision=?`, doc.ID, e.ParentID).Scan(&source); err != nil || source != e.OriginalSHA256 {
			return fmt.Errorf("derived extraction parent unavailable")
		}
	}
	var exists bool
	if err := tx.QueryRow(`SELECT EXISTS(SELECT 1 FROM document_extractions WHERE document_id=? AND revision=?)`, doc.ID, e.ID).Scan(&exists); err != nil {
		return err
	}
	if !exists {
		header := *e
		header.Text = ""
		header.Blocks = nil
		header.Relationships = nil
		data, err := json.Marshal(header)
		if err != nil {
			return err
		}
		metadata, err := json.Marshal(doc.Metadata)
		if err != nil {
			return err
		}
		storedBytes := int64(len(data) + len(metadata) + len(e.Text) + len(sourceRangesJSON(doc.SourceRanges)))
		if _, err = tx.Exec(`INSERT INTO document_extractions(document_id,revision,schema_version,source_sha256,raw_text,graph_json,source_ranges,metadata) VALUES(?,?,?,?,?,?,?,?)`, doc.ID, e.ID, document.ExtractionSchema, e.OriginalSHA256, e.Text, string(data), sourceRangesJSON(doc.SourceRanges), string(metadata)); err != nil {
			return err
		}
		stmt, err := tx.Prepare(`INSERT INTO document_blocks(document_id,revision,block_id,ordinal,unit_id,kind,start_byte,end_byte,block_json) VALUES(?,?,?,?,?,?,?,?,?)`)
		if err != nil {
			return err
		}
		defer stmt.Close()
		for i, b := range e.Blocks {
			data, err := json.Marshal(b)
			if err != nil {
				return err
			}
			storedBytes += int64(len(data))
			if _, err = stmt.Exec(doc.ID, e.ID, b.ID, i, b.UnitID, b.Kind, b.Span.Start, b.Span.End, string(data)); err != nil {
				return err
			}
		}
		rels, err := tx.Prepare(`INSERT INTO document_relationships(document_id,revision,ordinal,from_id,to_id,kind) VALUES(?,?,?,?,?,?)`)
		if err != nil {
			return err
		}
		defer rels.Close()
		for i, r := range e.Relationships {
			storedBytes += int64(len(r.From) + len(r.To) + len(r.Kind))
			if _, err = rels.Exec(doc.ID, e.ID, i, r.From, r.To, r.Kind); err != nil {
				return err
			}
		}
		if _, err = tx.Exec(`UPDATE document_extractions SET storage_bytes=? WHERE document_id=? AND revision=?`, storedBytes, doc.ID, e.ID); err != nil {
			return err
		}
	}
	_, err := tx.Exec(`UPDATE documents SET extraction_revision=? WHERE id=?`, e.ID, doc.ID)
	return err
}

// Preserve pre-migration source identity before its first replacement. This is
// lazy: opening the application never rewrites or reindexes old extractions.
func archiveCurrentExtraction(tx *sql.Tx, id string) error {
	var d Document
	d.ID = id
	var ranges, metadata string
	err := tx.QueryRow(`SELECT raw_content,source_ranges,metadata,extraction_revision FROM documents WHERE id=?`, id).Scan(&d.RawContent, &ranges, &metadata, &d.ExtractionRevision)
	if err != nil {
		return err
	}
	if d.ExtractionRevision != "" || d.RawContent == "" {
		return nil
	}
	if err = json.Unmarshal([]byte(ranges), &d.SourceRanges); err != nil {
		return err
	}
	if metadata != "" {
		if err = json.Unmarshal([]byte(metadata), &d.Metadata); err != nil {
			return err
		}
	}
	return storeExtraction(tx, &d)
}

func (s *SQLiteStore) extraction(id, revision string) (*document.Extraction, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.extractionLocked(id, revision)
}

func (s *SQLiteStore) extractionLocked(id, revision string) (*document.Extraction, error) {
	var header, text string
	err := s.db.QueryRow(`SELECT graph_json,raw_text FROM document_extractions WHERE document_id=? AND revision=? AND schema_version=? AND LENGTH(CAST(raw_text AS BLOB))<=? AND LENGTH(CAST(graph_json AS BLOB))<=?`, id, revision, document.ExtractionSchema, 16<<20, 4<<20).Scan(&header, &text)
	if err != nil {
		return nil, err
	}
	var e document.Extraction
	if err = json.Unmarshal([]byte(header), &e); err != nil {
		return nil, err
	}
	e.Text = text
	rows, err := s.db.Query(`SELECT block_json FROM document_blocks WHERE document_id=? AND revision=? ORDER BY ordinal LIMIT 200001`, id, revision)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var raw string
		var b document.Block
		if err = rows.Scan(&raw); err == nil {
			err = json.Unmarshal([]byte(raw), &b)
		}
		if err != nil {
			rows.Close()
			return nil, err
		}
		e.Blocks = append(e.Blocks, b)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return nil, err
	}
	rows, err = s.db.Query(`SELECT from_id,to_id,kind FROM document_relationships WHERE document_id=? AND revision=? ORDER BY ordinal LIMIT 400001`, id, revision)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var r document.Relationship
		if err = rows.Scan(&r.From, &r.To, &r.Kind); err != nil {
			rows.Close()
			return nil, err
		}
		e.Relationships = append(e.Relationships, r)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return nil, err
	}
	if e.ID != revision {
		return nil, fmt.Errorf("extraction revision mismatch")
	}
	if err = e.Verify(); err != nil {
		return nil, err
	}
	return &e, nil
}
