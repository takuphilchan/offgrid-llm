package rag

import (
	"errors"
	"testing"
	"time"
)

func TestRetentionHoldSurvivesReindexAndRestart(t *testing.T) {
	dir := t.TempDir()
	s, err := NewSQLiteStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	doc := originalFixture([]byte("held source"))
	if err = s.AddDocumentWithChunks(doc, nil, nil); err != nil {
		t.Fatal(err)
	}
	p, err := s.SetDocumentRetention("owner", doc.ID, 1, time.Now().Add(-time.Hour).Unix(), true)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.SetDocumentRetention("owner", doc.ID, 1, 0, false); !errors.Is(err, ErrCollectionConflict) {
		t.Fatal("stale update removed hold", err)
	}
	if err = s.ReplaceDocumentWithChunks(doc, nil, nil); err != nil {
		t.Fatal(err)
	}
	s.Close()
	s, err = NewSQLiteStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if err = s.DeleteDocument(doc.ID); !errors.Is(err, ErrDocumentHeld) {
		t.Fatal("hold lost", err)
	}
	if n, err := s.expireDocuments(time.Now(), 100); err != nil || n != 0 {
		t.Fatal("held source expired", n, err)
	}
	if _, err = s.original(doc.ID); err != nil {
		t.Fatal("held original removed", err)
	}
	if _, err = s.SetDocumentRetention("owner", doc.ID, p.Revision, time.Now().Add(-time.Hour).Unix(), false); err != nil {
		t.Fatal(err)
	}
	if n, err := s.expireDocuments(time.Now(), 1); err != nil || n != 1 {
		t.Fatal("released source not expired", n, err)
	}
	if _, err = s.original(doc.ID); !errors.Is(err, ErrOriginalUnavailable) {
		t.Fatal("expired original remained", err)
	}
}

func TestRetentionExpiryFencesStaleImportAndPreservesOthers(t *testing.T) {
	dir := t.TempDir()
	s, err := NewSQLiteStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	r := acceptedImport(t, s, "alice", "first")
	doc, _ := indexingImport(t, s, r, "alice")
	chunk, index := publicationFixture(doc)
	if err = s.PublishDocument(doc, []*Chunk{chunk}, [][]float32{{1, 0}}, index, false); err != nil {
		t.Fatal(err)
	}
	r2 := acceptedImport(t, s, "alice", "second")
	stale, _ := indexingImport(t, s, r2, "alice")
	other := originalFixture([]byte("unrelated"))
	if err = s.AddDocumentWithChunks(other, nil, nil); err != nil {
		t.Fatal(err)
	}
	if _, err = s.SetDocumentRetention("alice", doc.ID, 1, time.Now().Add(-time.Second).Unix(), false); err != nil {
		t.Fatal(err)
	}
	if n, err := s.expireDocuments(time.Now(), 10); err != nil || n != 1 {
		t.Fatal(n, err)
	}
	if err = s.PublishDocument(stale, []*Chunk{chunk}, [][]float32{{1, 0}}, index, false); err == nil {
		t.Fatal("stale worker resurrected expired source")
	}
	s.Close()
	s, err = NewSQLiteStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if _, err = s.original(doc.ID); !errors.Is(err, ErrOriginalUnavailable) {
		t.Fatal(err)
	}
	if _, err = s.original(other.ID); err != nil {
		t.Fatal("unrelated source removed", err)
	}
	var count int
	if err = s.db.QueryRow(`SELECT COUNT(*) FROM chunks WHERE document_id=?`, doc.ID).Scan(&count); err != nil || count != 0 {
		t.Fatal("vectors/text survived", count, err)
	}
	for _, id := range []string{r.ID, r2.ID} {
		var state, text string
		var bytes int
		if err = s.db.QueryRow(`SELECT state,extraction_json,COALESCE(LENGTH(content),0) FROM document_imports WHERE id=?`, id).Scan(&state, &text, &bytes); err != nil || state != "deleted" || text != "" || bytes != 0 {
			t.Fatal("import retained deleted content", state, bytes, err)
		}
	}
}

func TestRetentionUnknownPolicyFailsClosed(t *testing.T) {
	s, err := NewSQLiteStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	doc := originalFixture([]byte("future policy"))
	if err = s.AddDocumentWithChunks(doc, nil, nil); err != nil {
		t.Fatal(err)
	}
	if _, err = s.db.Exec(`UPDATE document_retention SET schema_version=99 WHERE document_id=?`, doc.ID); err != nil {
		t.Fatal(err)
	}
	if err = s.DeleteDocument(doc.ID); !errors.Is(err, ErrRetentionUnavailable) {
		t.Fatal("unknown policy authorized removal", err)
	}
	if _, err = s.SetDocumentRetention("owner", doc.ID, 1, 0, false); !errors.Is(err, ErrRetentionUnavailable) {
		t.Fatal("unknown policy overwritten", err)
	}
}

func TestRetentionWorkerClosesAndImportRemovalHonorsHold(t *testing.T) {
	e := NewEngine(nil, t.TempDir())
	s, _ := e.ImportStore()
	r := acceptedImport(t, s, "alice", "held-import")
	doc, _ := indexingImport(t, s, r, "alice")
	chunk, index := publicationFixture(doc)
	if err := s.PublishDocument(doc, []*Chunk{chunk}, [][]float32{{1, 0}}, index, false); err != nil {
		t.Fatal(err)
	}
	if _, err := s.SetDocumentRetention("alice", doc.ID, 1, 1, true); err != nil {
		t.Fatal(err)
	}
	if err := e.StopImport("alice", r.ID, true); !errors.Is(err, ErrDocumentHeld) {
		t.Fatal("held import removed", err)
	}
	if _, err := s.SetDocumentRetention("alice", doc.ID, 2, 1, false); err != nil {
		t.Fatal(err)
	}
	e.StartRetention()
	e.StartRetention()
	deadline := time.Now().Add(3 * time.Second)
	for e.RetentionStatus().CheckedAt == 0 && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	if status := e.RetentionStatus(); status.Deleted != 1 || status.ErrorCode != "" {
		t.Fatal(status)
	}
	if e.ForActor("alice").GetDocument(doc.ID) != nil {
		t.Fatal("expiry worker retained document")
	}
	if err := e.Close(); err != nil {
		t.Fatal(err)
	}
	e.StartRetention() // A closed engine cannot create a replacement worker.
	select {
	case <-e.retention.done:
	default:
		t.Fatal("maintenance worker outlived storage")
	}
}
