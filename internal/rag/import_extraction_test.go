package rag

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"
)

func TestPartialImportReviewPersistenceScopeRetryAndDeletion(t *testing.T) {
	dir := t.TempDir()
	s, err := NewSQLiteStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	r := acceptedImport(t, s, "alice", "partial")
	r, err = s.beginImport("alice", r.ID)
	if err != nil {
		t.Fatal(err)
	}
	lease := importLease{id: r.ID, owner: "alice", attempt: r.Attempt}
	if err = s.advanceImport(lease, "queued", "extracting", ""); err != nil {
		t.Fatal(err)
	}
	parsed := &ParseResult{State: ExtractionPartial, SourceUnits: []SourceUnit{
		{Kind: "page", Index: 1, Text: "Readable café 日本語", State: "ready", SourceSHA256: r.SHA256, ExtractorRevision: "native"},
		{Kind: "page", Index: 2, Text: "Uncertain scan", State: "ocr_partial", SourceSHA256: r.SHA256, ExtractorRevision: "ocr"},
	}}
	if err = s.saveImportExtraction(lease, parsed); err != nil {
		t.Fatal(err)
	}
	if _, err = s.ImportExtraction("alice", r.ID); !errors.Is(err, ErrImportNotFound) {
		t.Fatal("active extraction exposed", err)
	}
	if err = s.finishImportFailure(lease, "needs_input", "document_ocr_required"); err != nil {
		t.Fatal(err)
	}
	s.Close()
	s, err = NewSQLiteStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	got, err := s.ImportExtraction("alice", r.ID)
	if err != nil || got.Units[0].Text != parsed.SourceUnits[0].Text || got.Units[1].State != "ocr_partial" {
		t.Fatal(got, err)
	}
	saved, err := s.Import("alice", r.ID)
	if err != nil || !saved.ReviewAvailable {
		t.Fatal(saved, err)
	}
	data, _ := json.Marshal(saved)
	if strings.Contains(string(data), "Readable") {
		t.Fatal("private extraction in list")
	}
	if _, err = s.ImportExtraction("bob", r.ID); !errors.Is(err, ErrImportNotFound) {
		t.Fatal("cross-owner read", err)
	}
	if _, err = s.beginImport("alice", r.ID); err != nil {
		t.Fatal(err)
	}
	if _, err = s.ImportExtraction("alice", r.ID); !errors.Is(err, ErrImportNotFound) {
		t.Fatal("stale preview during retry", err)
	}
	if err = s.saveImportExtraction(lease, parsed); !errors.Is(err, ErrImportConflict) {
		t.Fatal("stale worker rewrote extraction", err)
	}
	if err = s.stopImport("alice", r.ID, true); err != nil {
		t.Fatal(err)
	}
	if _, err = s.ImportExtraction("alice", r.ID); !errors.Is(err, ErrImportNotFound) {
		t.Fatal("deleted extraction exposed", err)
	}
}

func TestPartialImportReviewRejectsForgeryAndStorageExhaustion(t *testing.T) {
	s, err := NewSQLiteStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	r := acceptedImport(t, s, "alice", "partial")
	r, err = s.beginImport("alice", r.ID)
	if err != nil {
		t.Fatal(err)
	}
	lease := importLease{id: r.ID, owner: "alice", attempt: r.Attempt}
	if err = s.advanceImport(lease, "queued", "extracting", ""); err != nil {
		t.Fatal(err)
	}
	parsed := &ParseResult{State: ExtractionPartial, SourceUnits: []SourceUnit{{Kind: "page", Index: 1, Text: "text", State: "ocr_partial", SourceSHA256: strings.Repeat("b", 64), ExtractorRevision: "ocr"}}}
	if err = s.saveImportExtraction(lease, parsed); err == nil {
		t.Fatal("different original accepted")
	}
	parsed.SourceUnits[0].SourceSHA256 = r.SHA256
	s.originalQuota = r.Bytes
	if err = s.saveImportExtraction(lease, parsed); err == nil {
		t.Fatal("extraction quota ignored")
	}
	s.originalQuota = 1 << 20
	if err = s.saveImportExtraction(lease, parsed); err != nil {
		t.Fatal(err)
	}
	if err = s.finishImportFailure(lease, "needs_input", "document_ocr_required"); err != nil {
		t.Fatal(err)
	}
	if _, err = s.db.Exec(`UPDATE document_imports SET extraction_json='{"version":99}' WHERE id=?`, r.ID); err != nil {
		t.Fatal(err)
	}
	if _, err = s.ImportExtraction("alice", r.ID); err == nil {
		t.Fatal("unknown preview version accepted")
	}
}
