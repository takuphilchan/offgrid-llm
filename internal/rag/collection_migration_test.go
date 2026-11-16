package rag

import (
	"database/sql"
	"path/filepath"
	"testing"
)

func TestCollectionAuditMigrationBacksUpAndPreservesPrivateBindings(t *testing.T) {
	dir := t.TempDir()
	s, err := NewSQLiteStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	p, err := s.CreateCollection("owner", "Private", []string{"reader"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	r, err := s.AcceptImportIntoCollection("owner", "legacy-import", "source.txt", []byte("Retained source"), p.ID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.db.Exec(`DROP TABLE knowledge_collection_audit; ALTER TABLE import_collections DROP COLUMN expected_revision; ALTER TABLE document_imports DROP COLUMN ocr_language; DELETE FROM knowledge_policy_migrations WHERE version>=5`); err != nil {
		t.Fatal(err)
	}
	s.Close()
	s, err = NewSQLiteStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	got, err := s.Import("owner", r.ID)
	if err != nil || got.CollectionID != p.ID || got.SHA256 != r.SHA256 {
		t.Fatal("migration changed import", got, err)
	}
	policy, err := s.Collection("reader", p.ID)
	if err != nil || policy.Owner != "owner" || policy.Revision != 1 {
		t.Fatal("migration changed owner/audience", policy, err)
	}
	paths, err := filepath.Glob(filepath.Join(dir, ".before-document-upgrade-*.db"))
	if err != nil || len(paths) != 1 {
		t.Fatal("missing pre-policy snapshot", paths, err)
	}
	db, err := sql.Open("sqlite", paths[0])
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	var version int
	var hash string
	if err = db.QueryRow(`SELECT MAX(version) FROM knowledge_policy_migrations`).Scan(&version); err != nil || version != 4 {
		t.Fatal("snapshot not compatible with old policy", version, err)
	}
	if err = db.QueryRow(`SELECT source_sha256 FROM document_imports WHERE id=?`, r.ID).Scan(&hash); err != nil || hash != r.SHA256 {
		t.Fatal("snapshot lost source", err)
	}
}
