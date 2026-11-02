package rag

import (
	"database/sql"
	"errors"
	"time"
)

var ErrDocumentHeld = errors.New("document is under a legal hold")
var ErrRetentionUnavailable = errors.New("document retention policy is unavailable")

// DeleteAfter is an opt-in expiry (UTC Unix seconds), not a promise of secure
// erasure. Zero preserves the existing retain-until-deleted behavior. Backups
// and saved answers have separate retention; this policy covers source storage.
type DocumentRetention struct {
	Version     int   `json:"version"`
	Revision    int   `json:"revision"`
	DeleteAfter int64 `json:"delete_after"`
	LegalHold   bool  `json:"legal_hold"`
}

func retentionPolicy(q policyQuery, id string) (*DocumentRetention, error) {
	p := &DocumentRetention{}
	var held int
	err := q.QueryRow(`SELECT schema_version,revision,delete_after,legal_hold FROM document_retention WHERE document_id=?`, id).Scan(&p.Version, &p.Revision, &p.DeleteAfter, &held)
	if err != nil || p.Version != 1 || p.Revision < 1 || p.DeleteAfter < 0 || (held != 0 && held != 1) {
		return nil, ErrRetentionUnavailable
	}
	p.LegalHold = held == 1
	return p, nil
}

// Only the collection owner may change private-source retention, not an editor.
// The service must enforce its administrative permission for the shared library.
// The authenticated service exposes this through revision-checked retention APIs.
func (s *SQLiteStore) SetDocumentRetention(actor, id string, expectedRevision int, deleteAfter int64, held bool) (*DocumentRetention, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !validKnowledgeActor(actor) || deleteAfter < 0 || deleteAfter > 253402300799 {
		return nil, ErrRetentionUnavailable
	}
	policy, _, err := documentPolicy(s.db, id)
	if err != nil || !policy.Allows(actor, true) || (policy.ID != WorkspaceCollection && policy.Owner != actor) {
		return nil, ErrCollectionUnavailable
	}
	current, err := retentionPolicy(s.db, id)
	if err != nil {
		return nil, err
	}
	if current.Revision != expectedRevision {
		return nil, ErrCollectionConflict
	}
	result, err := s.db.Exec(`UPDATE document_retention SET revision=revision+1,delete_after=?,legal_hold=? WHERE document_id=? AND revision=?`, deleteAfter, held, id, expectedRevision)
	if err != nil {
		return nil, err
	}
	if count, err := result.RowsAffected(); err != nil {
		return nil, err
	} else if count != 1 {
		return nil, ErrCollectionConflict
	}
	return retentionPolicy(s.db, id)
}

// Expiry uses the same transactional deletion path as manual removal. Batch
// bounds keep cleanup cooperative; no default deadline is assigned to old data.
// Callers must serialize with engine ingestion and apply service-level authority.
func (s *SQLiteStore) expireDocuments(now time.Time, limit int) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if limit < 1 || limit > 100 {
		return 0, ErrRetentionUnavailable
	}
	rows, err := s.db.Query(`SELECT r.document_id FROM document_retention r JOIN documents d ON d.id=r.document_id WHERE r.schema_version=1 AND r.revision>0 AND r.delete_after>0 AND r.delete_after<=? AND r.legal_hold=0 ORDER BY r.delete_after,r.document_id LIMIT ?`, now.Unix(), limit)
	if err != nil {
		return 0, err
	}
	var ids []string
	for rows.Next() {
		var id string
		if err = rows.Scan(&id); err != nil {
			rows.Close()
			return 0, err
		}
		ids = append(ids, id)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return 0, err
	}
	for i, id := range ids {
		if err = s.deleteDocument(id); err != nil {
			return i, err
		}
	}
	return len(ids), nil
}

func checkDeletionRetention(tx *sql.Tx, id string) error {
	var exists bool
	if err := tx.QueryRow(`SELECT EXISTS(SELECT 1 FROM documents WHERE id=?)`, id).Scan(&exists); err != nil {
		return err
	}
	if !exists {
		return nil
	} // Preserve idempotent legacy deletion.
	policy, err := retentionPolicy(tx, id)
	if err != nil {
		return err
	}
	if policy.LegalHold {
		return ErrDocumentHeld
	}
	return nil
}
