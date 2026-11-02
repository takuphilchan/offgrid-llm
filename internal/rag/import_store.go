package rag

import (
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"
)

const importSchema = 1

var ErrImportNotFound = errors.New("document import not found")
var ErrImportConflict = errors.New("document import changed or is already running")

// ImportRecord contains bounded progress metadata, never original/extracted bytes.
// Pending originals require both their owner and current destination access.
// The internal collection binding is not yet an exposed API contract.
type ImportRecord struct {
	CollectionID    string    `json:"-"`
	ID              string    `json:"id"`
	Name            string    `json:"name"`
	SHA256          string    `json:"sha256"`
	Bytes           int64     `json:"bytes"`
	State           string    `json:"state"`
	Attempt         int       `json:"attempt"`
	ErrorCode       string    `json:"error_code,omitempty"`
	DocumentID      string    `json:"document_id,omitempty"`
	ReviewAvailable bool      `json:"review_available,omitempty"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
}

type importLease struct {
	id, owner  string
	collection string
	attempt    int
}

const importColumns = `id,name,source_sha256,byte_size,state,attempt,error_code,document_id,created_at,updated_at,(extraction_json<>'' AND state IN ('needs_input','interrupted','cancelled','failed')),COALESCE((SELECT collection_id FROM import_collections WHERE import_id=document_imports.id AND schema_version=1),'')`

type rowScanner interface{ Scan(...any) error }

func scanImport(row rowScanner) (*ImportRecord, error) {
	var r ImportRecord
	err := row.Scan(&r.ID, &r.Name, &r.SHA256, &r.Bytes, &r.State, &r.Attempt, &r.ErrorCode, &r.DocumentID, &r.CreatedAt, &r.UpdatedAt, &r.ReviewAvailable, &r.CollectionID)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrImportNotFound
	}
	if err == nil && r.CollectionID == "" {
		return nil, ErrCollectionUnavailable
	}
	return &r, err
}

func (s *SQLiteStore) AcceptImport(owner, key, name string, data []byte) (*ImportRecord, error) {
	return s.AcceptImportIntoCollection(owner, key, name, data, WorkspaceCollection)
}

// Collection-management endpoints remain gated until saved-context policy and
// retention checks pass. The default client import stays in the shared workspace.
func (s *SQLiteStore) AcceptImportIntoCollection(owner, key, name string, data []byte, collection string) (*ImportRecord, error) {
	collection = normalizedCollection(collection)
	if owner == "" || len(owner) > 256 || key == "" || len(key) > 128 || strings.ContainsAny(key, "\x00\r\n") || name == "" || len(name) > 512 || !utf8.ValidString(name) || strings.ContainsAny(name, "/\\\x00\r\n") {
		return nil, &ParseError{Code: "document_invalid", Message: "Import identity and filename are invalid."}
	}
	if len(data) == 0 || len(data) > maxOriginalBytes {
		return nil, &ParseError{Code: "document_limit", Message: "Documents must contain between 1 byte and 50 MiB."}
	}
	digest := sha256.Sum256(data)
	hash := hex.EncodeToString(digest[:])
	s.mu.Lock()
	defer s.mu.Unlock()
	tx, err := s.db.Begin()
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	if err := collectionAccess(tx, knowledgePrincipal{actor: owner}, collection, true); err != nil {
		return nil, err
	}
	prior, err := scanImport(tx.QueryRow(`SELECT `+importColumns+` FROM document_imports WHERE owner=? AND request_key=? AND schema_version=?`, owner, key, importSchema))
	if err == nil {
		if prior.CollectionID != collection || prior.Name != name || prior.SHA256 != hash || prior.Bytes != int64(len(data)) {
			return nil, ErrImportConflict
		}
		return prior, nil
	}
	if !errors.Is(err, ErrImportNotFound) {
		return nil, err
	}
	var count int
	if err = tx.QueryRow(`SELECT COUNT(*) FROM document_imports`).Scan(&count); err != nil {
		return nil, err
	}
	if count >= 4096 {
		return nil, &ParseError{Code: "document_storage_limit", Message: "Knowledge import history has reached its storage limit."}
	}
	if err = checkOriginalQuota(tx, s.originalQuota, int64(len(data))); err != nil {
		return nil, err
	}
	var random [16]byte
	if _, err = rand.Read(random[:]); err != nil {
		return nil, err
	}
	now := time.Now().UTC()
	r := &ImportRecord{CollectionID: collection, ID: "import-" + hex.EncodeToString(random[:]), Name: name, SHA256: hash, Bytes: int64(len(data)), State: "accepted", CreatedAt: now, UpdatedAt: now}
	_, err = tx.Exec(`INSERT INTO document_imports(id,schema_version,owner,request_key,name,source_sha256,byte_size,content,state,created_at,updated_at) VALUES(?,?,?,?,?,?,?,?,?,?,?)`, r.ID, importSchema, owner, key, name, hash, len(data), data, r.State, now, now)
	if err != nil {
		return nil, err
	}
	if _, err = tx.Exec(`UPDATE import_collections SET collection_id=? WHERE import_id=? AND schema_version=1`, collection, r.ID); err != nil {
		return nil, err
	}
	if err = tx.Commit(); err != nil {
		return nil, err
	}
	return r, nil
}

func (s *SQLiteStore) Import(owner, id string) (*ImportRecord, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if _, err := importAccess(s.db, owner, id, false); err != nil {
		return nil, err
	}
	return scanImport(s.db.QueryRow(`SELECT `+importColumns+` FROM document_imports WHERE owner=? AND id=? AND schema_version=?`, owner, id, importSchema))
}

func (s *SQLiteStore) Imports(owner string) ([]*ImportRecord, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	rows, err := s.db.Query(`SELECT `+importColumns+` FROM document_imports WHERE owner=? AND schema_version=? ORDER BY created_at DESC,id LIMIT 100`, owner, importSchema)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := []*ImportRecord{}
	for rows.Next() {
		r, err := scanImport(rows)
		if err != nil {
			return nil, err
		}
		result = append(result, r)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if err := rows.Close(); err != nil {
		return nil, err
	}
	visible := make([]*ImportRecord, 0, len(result))
	for _, r := range result {
		if err := collectionAccess(s.db, knowledgePrincipal{actor: owner}, r.CollectionID, false); err == nil {
			visible = append(visible, r)
		}
	}
	return visible, nil
}

func checkOriginalQuota(tx *sql.Tx, limit, additional int64) error {
	var used int64
	err := tx.QueryRow(`SELECT (SELECT COALESCE(SUM(byte_size),0) FROM document_originals)+(SELECT COALESCE(SUM(COALESCE(LENGTH(content),0)+LENGTH(CAST(extraction_json AS BLOB))),0) FROM document_imports)`).Scan(&used)
	if err != nil {
		return err
	}
	if used+additional > limit {
		return &ParseError{Code: "document_storage_limit", Message: "Knowledge originals and pending imports have reached the 2 GiB storage limit. Remove unneeded documents or imports before adding more."}
	}
	return nil
}

// RecoverImports is called only by the owning Engine during startup, never by
// read-only store inspection. Restart cannot silently replay extraction/indexing.
func (s *SQLiteStore) RecoverImports() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	_, err := s.db.Exec(`UPDATE document_imports SET state='interrupted',error_code='document_interrupted',attempt=attempt+1,updated_at=? WHERE schema_version=? AND state IN ('queued','extracting','indexing')`, time.Now().UTC(), importSchema)
	return err
}

func (s *SQLiteStore) beginImport(owner, id string) (*ImportRecord, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	tx, err := s.db.Begin()
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	if _, err := importAccess(tx, owner, id, true); err != nil {
		return nil, err
	}
	r, err := scanImport(tx.QueryRow(`SELECT `+importColumns+` FROM document_imports WHERE owner=? AND id=? AND schema_version=?`, owner, id, importSchema))
	if err != nil {
		return nil, err
	}
	switch r.State {
	case "accepted", "failed", "needs_input", "cancelled", "interrupted":
	default:
		return nil, ErrImportConflict
	}
	r.State = "queued"
	r.Attempt++
	r.ErrorCode = ""
	r.UpdatedAt = time.Now().UTC()
	_, err = tx.Exec(`UPDATE document_imports SET state=?,attempt=?,error_code='',extraction_json='',updated_at=? WHERE id=?`, r.State, r.Attempt, r.UpdatedAt, id)
	if err != nil {
		return nil, err
	}
	if err = tx.Commit(); err != nil {
		return nil, err
	}
	return r, nil
}

func (s *SQLiteStore) importBytes(lease importLease, r *ImportRecord) ([]byte, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if collection, err := importAccess(s.db, lease.owner, lease.id, true); err != nil || collection != normalizedCollection(lease.collection) {
		return nil, ErrCollectionUnavailable
	}
	var data []byte
	err := s.db.QueryRow(`SELECT content FROM document_imports WHERE id=? AND owner=? AND attempt=? AND state='extracting' AND schema_version=? AND LENGTH(content)=? AND byte_size BETWEEN 1 AND ?`, lease.id, lease.owner, lease.attempt, importSchema, r.Bytes, maxOriginalBytes).Scan(&data)
	if err != nil {
		return nil, fmt.Errorf("retained import original unavailable")
	}
	digest := sha256.Sum256(data)
	if hex.EncodeToString(digest[:]) != r.SHA256 {
		return nil, fmt.Errorf("retained import original failed integrity check")
	}
	return data, nil
}
