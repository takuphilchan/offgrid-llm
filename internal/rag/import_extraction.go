package rag

import (
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"unicode/utf8"
)

// ImportExtraction is review-only evidence, never an indexable document. It is
// separately fetched under the import owner's identity; progress lists omit it.
type ImportExtraction struct {
	Version  int                 `json:"version"`
	SHA256   string              `json:"source_sha256"`
	State    ExtractionState     `json:"state"`
	Units    []SourceUnit        `json:"source_units"`
	Warnings []ExtractionWarning `json:"warnings"`
}

func (v *ImportExtraction) validate(digest string) error {
	if _, err := hex.DecodeString(digest); err != nil || len(digest) != 64 {
		return fmt.Errorf("invalid partial source digest")
	}
	if v.Version != 1 || v.State != ExtractionPartial || v.SHA256 != digest || len(v.Units) == 0 || len(v.Units) > 1000 || len(v.Warnings) > 2000 {
		return fmt.Errorf("invalid retained partial extraction")
	}
	total := 0
	for i, u := range v.Units {
		// Partial review currently describes PDF pages only. Other adapters must
		// qualify their own unit contract before publishing partial evidence.
		if u.Kind != "page" || len(u.Cells) != 0 || u.Index != i+1 || u.SourceSHA256 != digest || u.ExtractorRevision == "" || len(u.ExtractorRevision) > 512 || !utf8.ValidString(u.Text) {
			return fmt.Errorf("invalid partial source unit")
		}
		switch u.State {
		case "ready":
			if strings.TrimSpace(u.Text) == "" {
				return fmt.Errorf("empty ready page")
			}
		case "empty":
			if strings.TrimSpace(u.Text) != "" {
				return fmt.Errorf("nonempty empty page")
			}
		case "ocr_required", "ocr_partial":
		default:
			return fmt.Errorf("invalid partial page state")
		}
		total += len(u.Text)
		if total > 16<<20 {
			return fmt.Errorf("partial extraction exceeds text limit")
		}
	}
	return nil
}

func (s *SQLiteStore) saveImportExtraction(lease importLease, parsed *ParseResult) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if collection, err := importAccess(tx, lease.owner, lease.id, true); err != nil || collection != normalizedCollection(lease.collection) {
		return ErrCollectionUnavailable
	}
	var digest string
	err = tx.QueryRow(`SELECT source_sha256 FROM document_imports WHERE id=? AND owner=? AND attempt=? AND state='extracting' AND schema_version=?`, lease.id, lease.owner, lease.attempt, importSchema).Scan(&digest)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrImportConflict
	}
	if err != nil {
		return err
	}
	v := ImportExtraction{Version: 1, SHA256: digest, State: parsed.State, Units: parsed.SourceUnits, Warnings: parsed.Warnings}
	if err = v.validate(digest); err != nil {
		return err
	}
	data, err := json.Marshal(v)
	if err != nil {
		return err
	}
	if len(data) > 32<<20 {
		return fmt.Errorf("partial extraction exceeds storage limit")
	}
	if err = checkOriginalQuota(tx, s.originalQuota, int64(len(data))); err != nil {
		return err
	}
	_, err = tx.Exec(`UPDATE document_imports SET extraction_json=? WHERE id=?`, string(data), lease.id)
	if err != nil {
		return err
	}
	return tx.Commit()
}

func (s *SQLiteStore) ImportExtraction(owner, id string) (*ImportExtraction, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if _, err := importAccess(s.db, owner, id, false); err != nil {
		return nil, err
	}
	var data, digest string
	err := s.db.QueryRow(`SELECT extraction_json,source_sha256 FROM document_imports WHERE id=? AND owner=? AND schema_version=? AND state IN ('needs_input','interrupted','cancelled','failed') AND LENGTH(CAST(extraction_json AS BLOB)) BETWEEN 1 AND ?`, id, owner, importSchema, 32<<20).Scan(&data, &digest)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrImportNotFound
	}
	if err != nil {
		return nil, err
	}
	var v ImportExtraction
	if err = json.Unmarshal([]byte(data), &v); err != nil {
		return nil, fmt.Errorf("retained extraction is invalid")
	}
	if err = v.validate(digest); err != nil {
		return nil, err
	}
	return &v, nil
}
