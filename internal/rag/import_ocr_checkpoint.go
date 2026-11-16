package rag

import (
	"database/sql"
	"errors"
	"time"

	"github.com/takuphilchan/offgrid-llm/internal/documentocr"
)

func (s *SQLiteStore) importOCRCheckpoints(lease importLease) documentocr.Checkpoints {
	return documentocr.Checkpoints{
		Read: func(page int, key string) ([]byte, error) { return s.importOCRCheckpoint(lease, page, key, nil) },
		Write: func(page int, key string, raw []byte) error {
			_, err := s.importOCRCheckpoint(lease, page, key, raw)
			return err
		},
	}
}

func (s *SQLiteStore) importOCRCheckpoint(lease importLease, page int, key string, raw []byte) ([]byte, error) {
	if !documentocr.ValidCheckpointKey(page, key) || raw != nil && (len(raw) == 0 || len(raw) > documentocr.MaxCheckpointBytes) {
		return nil, documentocr.ErrCheckpointInvalid
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	tx, err := s.db.Begin()
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	collection, err := importAccess(tx, lease.owner, lease.id, true)
	if err != nil || collection != normalizedCollection(lease.collection) {
		return nil, ErrCollectionUnavailable
	}
	var active int
	if err := tx.QueryRow(`SELECT COUNT(*) FROM document_imports WHERE id=? AND owner=? AND attempt=? AND state='extracting'`, lease.id, lease.owner, lease.attempt).Scan(&active); err != nil {
		return nil, err
	}
	if active != 1 {
		return nil, ErrImportConflict
	}
	if raw == nil {
		var size int
		var digest string
		err = tx.QueryRow(`SELECT LENGTH(receipt),receipt_sha256 FROM document_import_ocr_pages WHERE import_id=? AND page=? AND checkpoint_key=?`, lease.id, page, key).Scan(&size, &digest)
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		if err != nil {
			return nil, err
		}
		if size < 1 || size > documentocr.MaxCheckpointBytes {
			return nil, documentocr.ErrCheckpointInvalid
		}
		var data []byte
		if err = tx.QueryRow(`SELECT receipt FROM document_import_ocr_pages WHERE import_id=? AND page=? AND checkpoint_key=?`, lease.id, page, key).Scan(&data); err != nil {
			return nil, err
		}
		if len(data) != size || ocrDigest(data) != digest {
			return nil, documentocr.ErrCheckpointInvalid
		}
		return data, nil
	}
	var used, old int64
	if err = tx.QueryRow(`SELECT COALESCE(SUM(LENGTH(receipt)),0), COALESCE(MAX(CASE WHEN page=? THEN LENGTH(receipt) ELSE 0 END),0) FROM document_import_ocr_pages WHERE import_id=?`, page, lease.id).Scan(&used, &old); err != nil {
		return nil, err
	}
	if used-old+int64(len(raw)) > documentocr.MaxCheckpointSourceBytes {
		return nil, &ParseError{Code: "document_storage_limit", Message: "Saved OCR pages have reached the document storage limit."}
	}
	if err = checkOriginalQuota(tx, s.originalQuota, int64(len(raw))-old); err != nil {
		return nil, err
	}
	if _, err = tx.Exec(`INSERT INTO document_import_ocr_pages VALUES(?,?,?,?,?) ON CONFLICT(import_id,page) DO UPDATE SET checkpoint_key=excluded.checkpoint_key,receipt=excluded.receipt,receipt_sha256=excluded.receipt_sha256`, lease.id, page, key, raw, ocrDigest(raw)); err != nil {
		return nil, err
	}
	if _, err = tx.Exec(`UPDATE document_imports SET updated_at=? WHERE id=?`, time.Now().UTC(), lease.id); err != nil {
		return nil, err
	}
	return nil, tx.Commit()
}
