package rag

import (
	"database/sql"
	"fmt"
	"time"
)

func (s *SQLiteStore) advanceImport(lease importLease, from, to, code string) error {
	valid := from == "queued" && to == "extracting" || from == "extracting" && to == "indexing"
	if !valid {
		return fmt.Errorf("invalid import transition")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if collection, err := importAccess(s.db, lease.owner, lease.id, true); err != nil || collection != normalizedCollection(lease.collection) {
		return ErrCollectionUnavailable
	}
	result, err := s.db.Exec(`UPDATE document_imports SET state=?,error_code=?,updated_at=? WHERE id=? AND owner=? AND attempt=? AND state=? AND schema_version=?`, to, code, time.Now().UTC(), lease.id, lease.owner, lease.attempt, from, importSchema)
	if err != nil {
		return err
	}
	n, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if n != 1 {
		return ErrImportConflict
	}
	return nil
}

func (s *SQLiteStore) finishImportFailure(lease importLease, state, code string) error {
	if state != "failed" && state != "needs_input" && state != "cancelled" {
		return fmt.Errorf("invalid import failure state")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	_, err := s.db.Exec(`UPDATE document_imports SET state=?,error_code=?,updated_at=? WHERE id=? AND owner=? AND attempt=? AND schema_version=? AND state IN ('queued','extracting','indexing')`, state, code, time.Now().UTC(), lease.id, lease.owner, lease.attempt, importSchema)
	return err
}

func (s *SQLiteStore) stopImport(owner, id string, remove bool) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, err := importAccess(s.db, owner, id, true); err != nil {
		return err
	}
	var result sql.Result
	var err error
	if remove {
		// Removing an import must not erase retained evidence for a held source.
		rows, queryErr := s.db.Query(`SELECT d.document_id FROM document_originals d JOIN document_collections c ON c.document_id=d.document_id JOIN document_imports i ON i.source_sha256=d.source_sha256 JOIN import_collections ic ON ic.import_id=i.id AND ic.collection_id=c.source_namespace WHERE i.id=? AND i.owner=?`, id, owner)
		if queryErr != nil {
			return queryErr
		}
		var ids []string
		for rows.Next() {
			var linked string
			if err = rows.Scan(&linked); err != nil {
				rows.Close()
				return err
			}
			ids = append(ids, linked)
		}
		err = rows.Err()
		rows.Close()
		if err != nil {
			return err
		}
		for _, linked := range ids {
			policy, policyErr := retentionPolicy(s.db, linked)
			if policyErr != nil {
				return policyErr
			}
			if policy.LegalHold {
				return ErrDocumentHeld
			}
		}
		result, err = s.db.Exec(`DELETE FROM document_imports WHERE id=? AND owner=? AND schema_version=?`, id, owner, importSchema)
	} else {
		result, err = s.db.Exec(`UPDATE document_imports SET state='cancelled',error_code='document_cancelled',attempt=attempt+1,updated_at=? WHERE id=? AND owner=? AND schema_version=? AND state NOT IN ('ready','deleted')`, time.Now().UTC(), id, owner, importSchema)
	}
	if err != nil {
		return err
	}
	n, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if n != 1 {
		return ErrImportConflict
	}
	return nil
}

// This runs in the SAME transaction as original/vector/index publication.
func completeImport(tx *sql.Tx, doc *Document) error {
	if doc.original == nil || doc.original.importRef == nil {
		return nil
	}
	lease := doc.original.importRef
	collection, err := importAccess(tx, lease.owner, lease.id, true)
	if err != nil || collection != normalizedCollection(lease.collection) {
		return ErrCollectionUnavailable
	}
	policy, _, err := documentPolicy(tx, doc.ID)
	if err != nil || policy.ID != collection || !policy.Allows(lease.owner, true) {
		return ErrCollectionUnavailable
	}
	result, err := tx.Exec(`UPDATE document_imports SET state='ready',document_id=?,content=NULL,extraction_json='',error_code='',updated_at=? WHERE id=? AND owner=? AND attempt=? AND state='indexing' AND schema_version=? AND source_sha256=? AND byte_size=?`, doc.ID, time.Now().UTC(), lease.id, lease.owner, lease.attempt, importSchema, doc.original.SHA256, len(doc.original.Bytes))
	if err != nil {
		return err
	}
	n, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if n != 1 {
		return ErrImportConflict
	}
	return nil
}

func (s *SQLiteStore) linkExistingImport(doc *Document, original *OriginalSource) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var hash string
	if err = tx.QueryRow(`SELECT source_sha256 FROM document_originals WHERE document_id=? AND schema_version=?`, doc.ID, originalSchema).Scan(&hash); err != nil || hash != original.SHA256 {
		return fmt.Errorf("existing document has no matching retained original")
	}
	copy := *doc
	copy.original = original
	if err = completeImport(tx, &copy); err != nil {
		return err
	}
	return tx.Commit()
}
