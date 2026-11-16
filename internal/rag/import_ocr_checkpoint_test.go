package rag

import (
	"bytes"
	"errors"
	"strings"
	"testing"

	"github.com/takuphilchan/offgrid-llm/internal/documentocr"
)

func TestImportOCRCheckpointsSurviveRestartAndRejectStaleWorkers(t *testing.T) {
	dir := t.TempDir()
	s, err := NewSQLiteStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	r, err := s.AcceptImport("owner", "key", "scan.png", []byte("original"))
	if err != nil {
		t.Fatal(err)
	}
	start := func(s *SQLiteStore) importLease {
		t.Helper()
		r, err := s.beginImport("owner", r.ID)
		if err != nil {
			t.Fatal(err)
		}
		lease := importLease{id: r.ID, owner: "owner", attempt: r.Attempt, collection: r.CollectionID}
		if err := s.advanceImport(lease, "queued", "extracting", ""); err != nil {
			t.Fatal(err)
		}
		return lease
	}
	old := start(s)
	key, raw := strings.Repeat("a", 64), []byte(`{"retained":"page one"}`)
	if err := s.importOCRCheckpoints(old).Write(1, key, raw); err != nil {
		t.Fatal(err)
	}
	s.Close()
	s, err = NewSQLiteStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if err := s.RecoverImports(); err != nil {
		t.Fatal(err)
	}
	if err := s.importOCRCheckpoints(old).Write(2, key, raw); !errors.Is(err, ErrImportConflict) {
		t.Fatal("stale worker accepted", err)
	}
	lease := start(s)
	store := s.importOCRCheckpoints(lease)
	if got, err := store.Read(1, key); err != nil || !bytes.Equal(got, raw) {
		t.Fatal(string(got), err)
	}
	if got, err := store.Read(1, strings.Repeat("b", 64)); err != nil || got != nil {
		t.Fatal("wrong raster reused", err)
	}
	other := lease
	other.owner = "other"
	if _, err := s.importOCRCheckpoints(other).Read(1, key); err == nil {
		t.Fatal("private checkpoint leaked")
	}
	if err := s.stopImport("owner", r.ID, false); err != nil {
		t.Fatal(err)
	}
	if err := store.Write(2, key, raw); err == nil {
		t.Fatal("cancelled worker published")
	}
	lease = start(s)
	if _, err := s.importOCRCheckpoints(lease).Read(1, key); err != nil {
		t.Fatal(err)
	}
	if err := s.stopImport("owner", r.ID, true); err != nil {
		t.Fatal(err)
	}
	var count int
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM document_import_ocr_pages`).Scan(&count); err != nil || count != 0 {
		t.Fatal("retention cascade failed", count, err)
	}
	if err := s.importOCRCheckpoints(lease).Write(2, key, raw); err == nil {
		t.Fatal("removed import resurrected")
	}
}

func TestImportOCRCheckpointCorruptionAndQuota(t *testing.T) {
	s, err := NewSQLiteStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	r, err := s.AcceptImport("owner", "key", "scan.png", []byte("original"))
	if err != nil {
		t.Fatal(err)
	}
	r, err = s.beginImport("owner", r.ID)
	if err != nil {
		t.Fatal(err)
	}
	lease := importLease{id: r.ID, owner: "owner", attempt: r.Attempt, collection: r.CollectionID}
	if err := s.advanceImport(lease, "queued", "extracting", ""); err != nil {
		t.Fatal(err)
	}
	store, key := s.importOCRCheckpoints(lease), strings.Repeat("a", 64)
	if err := store.Write(1, key, []byte(`{"text":"source"}`)); err != nil {
		t.Fatal(err)
	}
	if _, err := s.db.Exec(`UPDATE document_import_ocr_pages SET receipt='changed' WHERE import_id=?`, r.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Read(1, key); !errors.Is(err, documentocr.ErrCheckpointInvalid) {
		t.Fatal("corruption accepted", err)
	}
	s.originalQuota = 1
	if err := store.Write(2, key, []byte(`{}`)); err == nil {
		t.Fatal("quota bypassed")
	}
	if err := store.Write(1001, key, []byte(`{}`)); !errors.Is(err, documentocr.ErrCheckpointInvalid) {
		t.Fatal(err)
	}
}
