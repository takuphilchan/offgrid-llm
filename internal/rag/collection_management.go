package rag

import (
	"context"
	"database/sql"
	"errors"
	"time"
)

var ErrCollectionInvalid = errors.New("invalid Knowledge collection request")

// The actor and owner are always supplied by authentication, never this payload.
type CollectionUpdate struct {
	ExpectedRevision int      `json:"expected_revision"`
	Name             string   `json:"name"`
	Readers          []string `json:"readers"`
	Editors          []string `json:"editors"`
}

type DocumentCollection struct {
	DocumentID string            `json:"document_id"`
	Revision   int               `json:"revision"`
	Collection *CollectionPolicy `json:"collection"`
}

// Both audience revisions are pinned to the policies the caller reviewed.
type CollectionMove struct {
	DocumentID                  string `json:"document_id"`
	ExpectedRevision            int    `json:"expected_revision"`
	ExpectedSourceRevision      int    `json:"expected_source_revision"`
	DestinationID               string `json:"destination_id"`
	ExpectedDestinationRevision int    `json:"expected_destination_revision"`
}

type CollectionAudit struct {
	Sequence  int64     `json:"sequence"`
	Actor     string    `json:"actor"`
	Action    string    `json:"action"`
	Subject   string    `json:"subject"`
	Revision  int       `json:"revision"`
	CreatedAt time.Time `json:"created_at"`
}

// No names, text, filenames, member lists or extracted content enter the ledger.
func recordCollectionAudit(tx *sql.Tx, actor, collection, action, subject string, revision int) error {
	_, err := tx.Exec(`INSERT INTO knowledge_collection_audit(actor,collection_id,action,subject,revision,created_at) VALUES(?,?,?,?,?,?)`, actor, collection, action, subject, revision, time.Now().UTC())
	return err
}

func (s *SQLiteStore) Collections(actor string) ([]*CollectionPolicy, error) {
	if !validKnowledgeActor(actor) {
		return nil, ErrCollectionUnavailable
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	rows, err := s.db.Query(`SELECT ` + collectionColumns + ` FROM knowledge_collections ORDER BY id LIMIT 1025`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := []*CollectionPolicy{}
	for rows.Next() {
		p, err := scanCollection(rows)
		if err != nil {
			return nil, err
		}
		if p.Allows(actor, false) {
			result = append(result, p)
		}
	}
	return result, rows.Err()
}

func (s *SQLiteStore) DocumentCollection(actor, id string) (*DocumentCollection, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	p, revision, err := documentPolicy(s.db, id)
	if err != nil || !p.Allows(actor, false) {
		return nil, ErrCollectionUnavailable
	}
	return &DocumentCollection{DocumentID: id, Revision: revision, Collection: p}, nil
}

func (s *SQLiteStore) CollectionAudit(actor, id string, after int64) ([]CollectionAudit, error) {
	if after < 0 {
		return nil, ErrCollectionInvalid
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	p, err := scanCollection(s.db.QueryRow(`SELECT `+collectionColumns+` FROM knowledge_collections WHERE id=?`, id))
	if err != nil || !validKnowledgeActor(actor) || p.Owner != actor {
		return nil, ErrCollectionUnavailable
	}
	rows, err := s.db.Query(`SELECT sequence,actor,action,subject,revision,created_at FROM knowledge_collection_audit WHERE collection_id=? AND sequence>? ORDER BY sequence LIMIT 100`, id, after)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := []CollectionAudit{}
	for rows.Next() {
		var event CollectionAudit
		if err := rows.Scan(&event.Sequence, &event.Actor, &event.Action, &event.Subject, &event.Revision, &event.CreatedAt); err != nil {
			return nil, err
		}
		result = append(result, event)
	}
	return result, rows.Err()
}

func (v *KnowledgeView) MoveDocument(ctx context.Context, request CollectionMove) (*DocumentCollection, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if request.DocumentID == "" || request.ExpectedRevision < 1 || request.ExpectedSourceRevision < 1 || request.ExpectedDestinationRevision < 1 || request.DestinationID == "" {
		return nil, ErrCollectionInvalid
	}
	e := v.engine
	e.mu.Lock()
	defer e.mu.Unlock()
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	s, ok := e.store.(*SQLiteStore)
	if !ok || !v.principal.valid() || v.principal.sharedOnly {
		return nil, ErrCollectionUnavailable
	}
	_, err := s.assignDocumentCollection(v.principal.actor, request.DocumentID, request.DestinationID, request.ExpectedRevision, request.ExpectedSourceRevision, request.ExpectedDestinationRevision)
	if err != nil {
		return nil, err
	}
	return s.DocumentCollection(v.principal.actor, request.DocumentID)
}
