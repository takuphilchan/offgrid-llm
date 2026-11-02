package rag

import (
	"bytes"
	"errors"
	"testing"
)

func TestCollectionImportsIsolateIdenticalSourcesAndDeletion(t *testing.T) {
	dir := t.TempDir()
	s, err := NewSQLiteStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { s.Close() }()
	a, err := s.CreateCollection("owner", "A", nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	b, err := s.CreateCollection("owner", "B", nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	data := []byte("Exact original — café 日本語")
	accept := func(key, collection string) *ImportRecord {
		t.Helper()
		r, err := s.AcceptImportIntoCollection("owner", key, "source.txt", data, collection)
		if err != nil {
			t.Fatal(err)
		}
		return r
	}
	ra, rb := accept("a", a.ID), accept("b", b.ID)
	if _, err := s.AcceptImportIntoCollection("owner", "a", "source.txt", data, b.ID); !errors.Is(err, ErrImportConflict) {
		t.Fatal("destination changed on replay", err)
	}
	da, _ := indexingImport(t, s, ra, "owner")
	db, _ := indexingImport(t, s, rb, "owner")
	if da.ID == db.ID || da.ID == GenerateDocumentID(data) {
		t.Fatal("collection identities overlap")
	}
	for _, d := range []*Document{da, db} {
		if err := s.AddDocumentWithChunks(d, nil, nil); err != nil {
			t.Fatal(err)
		}
	}
	duplicate := accept("a2", a.ID)
	dd, _ := indexingImport(t, s, duplicate, "owner")
	if err := s.linkExistingImport(da, dd.original); err != nil {
		t.Fatal(err)
	}
	wrong := accept("b-wrong", b.ID)
	wrongDoc, _ := indexingImport(t, s, wrong, "owner")
	if err := s.linkExistingImport(da, wrongDoc.original); !errors.Is(err, ErrCollectionUnavailable) {
		t.Fatal("cross-collection duplicate linked", err)
	}
	pa, pb := accept("pending-a", a.ID), accept("pending-b", b.ID)
	stale, _ := indexingImport(t, s, pa, "owner")
	s.Close()
	s, err = NewSQLiteStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, d := range []*Document{da, db} {
		original, err := s.original(d.ID)
		if err != nil || !bytes.Equal(original.Bytes, data) {
			t.Fatal("namespace lost on restart", err)
		}
	}
	if err := s.DeleteDocument(da.ID); err != nil {
		t.Fatal(err)
	}
	for _, r := range []*ImportRecord{ra, duplicate, pa} {
		got, err := s.Import("owner", r.ID)
		if err != nil || got.State != "deleted" {
			t.Fatal("source may resurrect", got, err)
		}
	}
	if err := s.AddDocumentWithChunks(stale, nil, nil); err == nil {
		t.Fatal("stale worker resurrected deleted source")
	}
	for _, r := range []*ImportRecord{rb, pb, wrong} {
		got, err := s.Import("owner", r.ID)
		if err != nil || got.State == "deleted" {
			t.Fatal("other collection changed", got, err)
		}
	}
	if _, err := s.original(db.ID); err != nil {
		t.Fatal("other original lost", err)
	}
}

func TestCollectionImportRevocationFencesPublicationAndPreview(t *testing.T) {
	s, err := NewSQLiteStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	p, err := s.CreateCollection("owner", "Private", []string{"reader"}, []string{"editor"})
	if err != nil {
		t.Fatal(err)
	}
	data := []byte("Confidential source bytes")
	for _, actor := range []string{"reader", "stranger"} {
		if _, err := s.AcceptImportIntoCollection(actor, "denied", "secret.txt", data, p.ID); !errors.Is(err, ErrCollectionUnavailable) {
			t.Fatal("unauthorized admission", err)
		}
	}
	r, err := s.AcceptImportIntoCollection("editor", "one", "secret.txt", data, p.ID)
	if err != nil {
		t.Fatal(err)
	}
	doc, lease := indexingImport(t, s, r, "editor")
	if _, err := s.UpdateCollection("owner", p.ID, p.Revision, p.Name, nil, nil); err != nil {
		t.Fatal(err)
	}
	chunk, index := publicationFixture(doc)
	if err := s.PublishDocument(doc, []*Chunk{chunk}, [][]float32{{1, 0}}, index, false); !errors.Is(err, ErrCollectionUnavailable) {
		t.Fatal("revoked publication", err)
	}
	if got, _ := s.GetDocument(doc.ID); got != nil {
		t.Fatal("ghost publication")
	}
	if _, err := s.original(doc.ID); !errors.Is(err, ErrOriginalUnavailable) {
		t.Fatal("original published", err)
	}
	if _, err := s.Import("editor", r.ID); !errors.Is(err, ErrCollectionUnavailable) {
		t.Fatal("revoked metadata", err)
	}
	if got, err := s.Imports("editor"); err != nil || len(got) != 0 {
		t.Fatal("revoked names leaked", err)
	}
	if _, err := s.ImportExtraction("editor", r.ID); !errors.Is(err, ErrCollectionUnavailable) {
		t.Fatal("revoked preview", err)
	}
	if err := s.saveImportExtraction(lease, &ParseResult{}); !errors.Is(err, ErrCollectionUnavailable) {
		t.Fatal("revoked extraction persisted", err)
	}
	if _, err := s.importBytes(lease, r); !errors.Is(err, ErrCollectionUnavailable) {
		t.Fatal("revoked bytes delivered", err)
	}
	if _, err := s.beginImport("editor", r.ID); !errors.Is(err, ErrCollectionUnavailable) {
		t.Fatal("revoked retry", err)
	}
	var state string
	if err := s.db.QueryRow(`SELECT state FROM document_imports WHERE id=?`, r.ID).Scan(&state); err != nil || state == "ready" {
		t.Fatal("false completion", err)
	}
}

func TestImportCollectionMigrationAndCorruptionFailClosed(t *testing.T) {
	dir := t.TempDir()
	s, err := NewSQLiteStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { s.Close() }()
	r := acceptedImport(t, s, "owner", "legacy")
	if _, err := s.db.Exec(`DROP TRIGGER documents_retention_insert; DROP TABLE document_retention; DROP TRIGGER imports_collection_insert; DROP TABLE import_collections; ALTER TABLE document_collections DROP COLUMN source_namespace; DELETE FROM knowledge_policy_migrations WHERE version>=3`); err != nil {
		t.Fatal(err)
	}
	s.Close()
	s, err = NewSQLiteStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	migrated, err := s.Import("owner", r.ID)
	if err != nil || migrated.CollectionID != WorkspaceCollection || migrated.SHA256 != r.SHA256 || migrated.State != "accepted" {
		t.Fatal("legacy import changed", migrated, err)
	}
	if _, err := s.db.Exec(`UPDATE import_collections SET schema_version=99 WHERE import_id=?`, r.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := s.beginImport("owner", r.ID); !errors.Is(err, ErrImportNotFound) {
		t.Fatal("future binding executed", err)
	}
	s.Close()
	s, err = NewSQLiteStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.Import("owner", r.ID); !errors.Is(err, ErrImportNotFound) {
		t.Fatal("restart regranted future binding", err)
	}
}
