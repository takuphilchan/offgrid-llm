package rag

import (
	"database/sql"
	"fmt"
)

type policyQuery interface{ QueryRow(string, ...any) *sql.Row }

func documentPolicy(q policyQuery, id string) (*CollectionPolicy, int, error) {
	var collection string
	var version, revision int
	if err := q.QueryRow(`SELECT collection_id,schema_version,revision FROM document_collections WHERE document_id=?`, id).Scan(&collection, &version, &revision); err != nil || version != 1 || revision < 1 {
		return nil, 0, ErrCollectionUnavailable
	}
	p, err := scanCollection(q.QueryRow(`SELECT `+collectionColumns+` FROM knowledge_collections WHERE id=?`, collection))
	return p, revision, err
}

// DocumentAccess must be rechecked at delivery, not retained as a permanent
// grant. The HTTP/job layer still enforces actor-level Knowledge permissions.
func (s *SQLiteStore) DocumentAccess(actor, id string, write bool) error {
	s.mu.RLock()
	defer s.mu.RUnlock()
	p, _, err := documentPolicy(s.db, id)
	if err != nil || !p.Allows(actor, write) {
		return ErrCollectionUnavailable
	}
	return nil
}

// AssignDocumentCollection is currently internal only. Do not expose movement
// until list/search/source/export/delete all enforce these bindings.
func (s *SQLiteStore) AssignDocumentCollection(actor, id, destination string, expectedRevision int) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	tx, err := s.db.Begin()
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()
	source, revision, err := documentPolicy(tx, id)
	if err != nil || !source.Allows(actor, true) {
		return 0, ErrCollectionUnavailable
	}
	// Editing content is not authority to change its audience. Private sources
	// may only be moved by their collection owner, even by a current editor.
	if source.ID != WorkspaceCollection && source.Owner != actor {
		return 0, ErrCollectionUnavailable
	}
	target, err := scanCollection(tx.QueryRow(`SELECT `+collectionColumns+` FROM knowledge_collections WHERE id=?`, destination))
	if err != nil || !target.Allows(actor, true) {
		return 0, ErrCollectionUnavailable
	}
	if expectedRevision != revision {
		return 0, ErrCollectionConflict
	}
	if source.ID == destination {
		return revision, nil
	}
	result, err := tx.Exec(`UPDATE document_collections SET collection_id=?,revision=revision+1 WHERE document_id=? AND collection_id=? AND schema_version=1 AND revision=?`, destination, id, source.ID, revision)
	if err != nil {
		return 0, err
	}
	if n, err := result.RowsAffected(); err != nil || n != 1 {
		return 0, ErrCollectionConflict
	}
	if err := tx.Commit(); err != nil {
		return 0, err
	}
	return revision + 1, nil
}

// AuthorizedDocumentIDs resolves current policy before retrieval scoring. nil
// requests all readable documents; an explicitly empty list means none. Callers
// must not turn an empty result into an unfiltered query.
func (s *SQLiteStore) AuthorizedDocumentIDs(actor string, requested []string) ([]string, error) {
	return s.authorizedDocumentIDs(knowledgePrincipal{actor: actor}, requested)
}

func (s *SQLiteStore) authorizedDocumentIDs(principal knowledgePrincipal, requested []string) ([]string, error) {
	if !principal.valid() {
		return nil, ErrCollectionUnavailable
	}
	if requested != nil && len(requested) == 0 {
		return []string{}, nil
	}
	if len(requested) > 4096 {
		return nil, fmt.Errorf("too many selected documents")
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	wanted := map[string]bool{}
	for _, id := range requested {
		if id == "" || wanted[id] {
			return nil, ErrCollectionUnavailable
		}
		wanted[id] = true
	}
	rows, err := s.db.Query(`SELECT d.document_id,c.id,c.schema_version,c.revision,c.name,c.owner,c.readers,c.editors
		FROM document_collections d JOIN knowledge_collections c ON c.id=d.collection_id
		WHERE d.schema_version=1 AND d.revision>0 ORDER BY d.document_id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	allowed := []string{}
	for rows.Next() {
		var id string
		p, err := scanCollection(prefixedPolicyRow{row: rows, first: &id})
		if err != nil {
			if err == ErrCollectionUnavailable {
				continue
			}
			return nil, err
		}
		if requested != nil && !wanted[id] {
			continue
		}
		if principal.allows(p, false) {
			allowed = append(allowed, id)
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if requested != nil && len(allowed) != len(requested) {
		return nil, ErrCollectionUnavailable
	}
	return allowed, nil
}

type prefixedPolicyRow struct {
	row   rowScanner
	first *string
}

func (r prefixedPolicyRow) Scan(values ...any) error {
	return r.row.Scan(append([]any{r.first}, values...)...)
}

// Legacy INSERT OR REPLACE would cascade-delete a private binding and invoke
// the default-workspace insert trigger. Require atomic reindexing instead.
func (s *SQLiteStore) checkLegacyReplacementPolicy(id string) error {
	var blocked bool
	err := s.db.QueryRow(`SELECT EXISTS(SELECT 1 FROM documents WHERE id=?) AND NOT EXISTS(
		SELECT 1 FROM document_collections WHERE document_id=? AND collection_id='workspace' AND source_namespace='workspace' AND schema_version=1 AND revision>0)`, id, id).Scan(&blocked)
	if err != nil {
		return err
	}
	if blocked {
		return fmt.Errorf("use atomic reindexing to preserve document collection policy")
	}
	return nil
}
