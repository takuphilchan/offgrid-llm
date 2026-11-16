package rag

import (
	"bytes"
	"context"
	"database/sql"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestOCRSelectionPersistsAndCannotChangeThroughIdempotency(t *testing.T) {
	for _, language := range []string{"chi_sim", "chi_sim+fra"} {
		t.Run(language, func(t *testing.T) { testOCRSelectionPersists(t, language) })
	}
}

func testOCRSelectionPersists(t *testing.T, selection string) {
	dir := t.TempDir()
	s, err := NewSQLiteStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	r, err := s.AcceptImportWithOCRLanguage("owner", "same", "scan.png", []byte("source"), WorkspaceCollection, selection)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.AcceptImportWithOCRLanguage("owner", "same", "scan.png", []byte("source"), WorkspaceCollection, "eng"); !errors.Is(err, ErrImportConflict) {
		t.Fatal("selection changed", err)
	}
	s.Close()
	s, err = NewSQLiteStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	got, err := s.Import("owner", r.ID)
	if err != nil || got.OCRLanguage != selection || got.SHA256 != r.SHA256 {
		t.Fatal(got, err)
	}
	if _, err = s.Import("other", r.ID); err == nil {
		t.Fatal("private import leaked")
	}
	if _, err = s.AcceptImportWithOCRLanguage("owner", "unknown", "x", []byte("source"), WorkspaceCollection, "sn"); err == nil {
		t.Fatal("unsupported language substituted")
	}
	lease, err := s.beginImport("owner", r.ID)
	if err != nil || lease.OCRLanguage != selection {
		t.Fatal("retry changed selection", lease, err)
	}
}

func TestOCRSelectionMigrationSnapshotsLegacyEnglish(t *testing.T) {
	dir := t.TempDir()
	s, err := NewSQLiteStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	r, err := s.AcceptImport("owner", "legacy", "x.txt", []byte("source"))
	if err != nil {
		t.Fatal(err)
	}
	_, err = s.db.Exec(`ALTER TABLE document_imports DROP COLUMN ocr_language; DELETE FROM knowledge_policy_migrations WHERE version>=6; DROP TABLE document_import_ocr_pages`)
	if err != nil {
		t.Fatal(err)
	}
	s.Close()
	s, err = NewSQLiteStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	got, err := s.Import("owner", r.ID)
	if err != nil || got.OCRLanguage != "eng" || got.SHA256 != r.SHA256 {
		t.Fatal(got, err)
	}
	paths, _ := filepath.Glob(filepath.Join(dir, ".before-document-upgrade-*.db"))
	if len(paths) != 1 {
		t.Fatal(paths)
	}
	db, err := sql.Open("sqlite", paths[0])
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	var version int
	if err = db.QueryRow(`SELECT MAX(version) FROM knowledge_policy_migrations`).Scan(&version); err != nil || version != 5 {
		t.Fatal(version, err)
	}
}

func TestNonEnglishPackImportIsIsolatedAndVerified(t *testing.T) {
	fixture := os.Getenv("OFFGRID_TEST_OCR_PACKS")
	if fixture == "" {
		t.Skip("requires explicit pinned packs")
	}
	dir := t.TempDir()
	data, err := os.ReadFile(filepath.Join(fixture, "chi_sim.traineddata"))
	if err != nil {
		t.Fatal(err)
	}
	if err = ImportOCRLanguage(context.Background(), dir, "chi_sim", bytes.NewReader(data)); err != nil {
		t.Fatal(err)
	}
	p := NewDocumentParserForOCRLanguage(dir, "chi_sim")
	if p.ocrSetupErr != nil || !bytes.Equal(p.ocrLanguage, data) || p.ocrPack().Language != "chi_sim" {
		t.Fatal("selection lost")
	}
	if p := NewDocumentParserForOCRLanguage(dir, "eng"); len(p.ocrLanguage) != 0 {
		t.Fatal("Chinese silently used for English")
	}
	if err = ImportOCRLanguage(context.Background(), dir, "eng", bytes.NewReader(data)); err == nil {
		t.Fatal("wrong language accepted")
	}
	if p := NewDocumentParserForOCRLanguage(dir, "sn"); p.ocrSetupErr == nil || len(p.ocrLanguage) != 0 {
		t.Fatal("unsupported language substituted")
	}
}
