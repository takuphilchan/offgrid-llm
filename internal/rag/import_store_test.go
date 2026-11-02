package rag

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"sync"
	"testing"
	"time"
)

func acceptedImport(t *testing.T, s *SQLiteStore, owner, key string) *ImportRecord {
	t.Helper()
	r, err := s.AcceptImport(owner, key, "source.txt", []byte("Private source café 日本語. Exact original."))
	if err != nil {
		t.Fatal(err)
	}
	return r
}

func indexingImport(t *testing.T, s *SQLiteStore, r *ImportRecord, owner string) (*Document, importLease) {
	t.Helper()
	active, err := s.beginImport(owner, r.ID)
	if err != nil {
		t.Fatal(err)
	}
	lease := importLease{id: r.ID, owner: owner, attempt: active.Attempt, collection: active.CollectionID}
	if err = s.advanceImport(lease, "queued", "extracting", ""); err != nil {
		t.Fatal(err)
	}
	data, err := s.importBytes(lease, r)
	if err != nil {
		t.Fatal(err)
	}
	if err = s.advanceImport(lease, "extracting", "indexing", ""); err != nil {
		t.Fatal(err)
	}
	doc := originalFixture(data)
	doc.collection = active.CollectionID
	doc.publicationActor = &knowledgePrincipal{actor: owner}
	doc.ID = collectionDocumentID(active.CollectionID, data)
	doc.original.importRef = &lease
	return doc, lease
}

func TestImportAcceptanceIdentityQuotaAndMigration(t *testing.T) {
	dir := t.TempDir()
	s, err := NewSQLiteStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	r := acceptedImport(t, s, "alice", "key")
	again := acceptedImport(t, s, "alice", "key")
	if r.ID != again.ID {
		t.Fatal("idempotency lost")
	}
	other := acceptedImport(t, s, "bob", "key")
	if other.ID == r.ID {
		t.Fatal("cross-owner identity shared")
	}
	if _, err = s.Import("bob", r.ID); !errors.Is(err, ErrImportNotFound) {
		t.Fatal("cross-owner read", err)
	}
	if _, err = s.AcceptImport("alice", "key", "source.txt", []byte("changed content")); !errors.Is(err, ErrImportConflict) {
		t.Fatal("key reused for new content", err)
	}
	raw, _ := json.Marshal(r)
	if bytes.Contains(raw, []byte("Private source")) {
		t.Fatal("original leaked in projection")
	}
	s.originalQuota = 1
	if _, err = s.AcceptImport("alice", "new", "source.txt", []byte("more")); err == nil {
		t.Fatal("quota bypassed")
	}
	s.Close()
	s, err = NewSQLiteStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	saved, err := s.Import("alice", r.ID)
	if err != nil || saved.State != "accepted" {
		t.Fatal("acceptance lost on restart", err)
	}
	if _, err = s.db.Exec(`UPDATE document_imports SET schema_version=99 WHERE id=?`, r.ID); err != nil {
		t.Fatal(err)
	}
	if _, err = s.beginImport("alice", r.ID); !errors.Is(err, ErrImportNotFound) {
		t.Fatal("unknown record version executed")
	}
}

func TestImportPublicationAndLostAcknowledgmentDoNotDuplicate(t *testing.T) {
	s, err := NewSQLiteStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	r := acceptedImport(t, s, "alice", "one")
	doc, _ := indexingImport(t, s, r, "alice")
	s.originalQuota = r.Bytes // transfer, not a second copy charged against quota
	chunk, index := publicationFixture(doc)
	if err = s.PublishDocument(doc, []*Chunk{chunk}, [][]float32{{1, 0}}, index, false); err != nil {
		t.Fatal(err)
	}
	saved, err := s.Import("alice", r.ID)
	if err != nil || saved.State != "ready" || saved.DocumentID != doc.ID {
		t.Fatal("ready receipt not atomic", err)
	}
	if _, err = s.beginImport("alice", r.ID); !errors.Is(err, ErrImportConflict) {
		t.Fatal("completed import replayed")
	}
	var retained int
	if err = s.db.QueryRow(`SELECT COALESCE(LENGTH(content),0) FROM document_imports WHERE id=?`, r.ID).Scan(&retained); err != nil || retained != 0 {
		t.Fatal("duplicate original retained", err)
	}
	again := acceptedImport(t, s, "alice", "one")
	if again.ID != r.ID || again.State != "ready" {
		t.Fatal("lost acknowledgment duplicated acceptance")
	}
	s.originalQuota = 2 * r.Bytes
	r2 := acceptedImport(t, s, "alice", "two")
	duplicate, _ := indexingImport(t, s, r2, "alice")
	if err = s.linkExistingImport(doc, duplicate.original); err != nil {
		t.Fatal(err)
	}
	docs, err := s.ListDocuments()
	if err != nil || len(docs) != 1 {
		t.Fatal("duplicate publication", err)
	}
	if err = s.DeleteDocument(doc.ID); err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{r.ID, r2.ID} {
		state, _ := s.Import("alice", id)
		if state.State != "deleted" {
			t.Fatal("deleted import can return old success")
		}
	}
}

func TestImportFailureCancelDeleteAndRestartFencePublication(t *testing.T) {
	for _, boundary := range []string{"cancel", "delete", "restart", "metadata"} {
		t.Run(boundary, func(t *testing.T) {
			s, err := NewSQLiteStore(t.TempDir())
			if err != nil {
				t.Fatal(err)
			}
			defer s.Close()
			r := acceptedImport(t, s, "alice", "key")
			doc, _ := indexingImport(t, s, r, "alice")
			switch boundary {
			case "cancel":
				err = s.stopImport("alice", r.ID, false)
			case "delete":
				err = s.stopImport("alice", r.ID, true)
			case "restart":
				err = s.RecoverImports()
			case "metadata":
				_, err = s.db.Exec(`CREATE TRIGGER fail_identity BEFORE INSERT ON index_metadata BEGIN SELECT RAISE(ABORT,'fixture failure'); END`)
			}
			if err != nil {
				t.Fatal(err)
			}
			chunk, index := publicationFixture(doc)
			if err = s.PublishDocument(doc, []*Chunk{chunk}, [][]float32{{1, 0}}, index, false); err == nil {
				t.Fatal("stale or failed publication committed")
			}
			if got, _ := s.GetDocument(doc.ID); got != nil {
				t.Fatal("ghost ready document")
			}
			if boundary == "delete" {
				return
			}
			saved, err := s.Import("alice", r.ID)
			if err != nil || saved.State == "ready" {
				t.Fatal("false completion", err)
			}
			var data []byte
			if err = s.db.QueryRow(`SELECT content FROM document_imports WHERE id=?`, r.ID).Scan(&data); err != nil || !bytes.Equal(data, doc.original.Bytes) {
				t.Fatal("failed import lost original", err)
			}
		})
	}
}

func TestImportConcurrentAcceptanceUsesOneIdentity(t *testing.T) {
	s, err := NewSQLiteStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	var group sync.WaitGroup
	results := make(chan *ImportRecord, 8)
	for i := 0; i < 8; i++ {
		group.Add(1)
		go func() {
			defer group.Done()
			r, err := s.AcceptImport("alice", "same", "source.txt", []byte("same original"))
			if err != nil {
				t.Error(err)
				return
			}
			results <- r
		}()
	}
	group.Wait()
	close(results)
	id := ""
	for r := range results {
		if id != "" && id != r.ID {
			t.Fatal("duplicate acceptance")
		}
		id = r.ID
	}
	rows, err := s.Imports("alice")
	if err != nil || len(rows) != 1 {
		t.Fatal("acceptance count", err)
	}
}

func TestEngineImportFailureRetainsOriginalAndRetryIdentity(t *testing.T) {
	e := NewEngine(nil, t.TempDir())
	defer e.Close()
	s, _ := e.ImportStore()
	r, err := s.AcceptImport("alice", "bad", "broken.docx", []byte("invalid zip bytes"))
	if err != nil {
		t.Fatal(err)
	}
	for attempt := 1; attempt <= 2; attempt++ {
		if _, err = e.RunImport(context.Background(), "alice", r.ID); err == nil {
			t.Fatal("invalid document accepted")
		}
		saved, _ := s.Import("alice", r.ID)
		if saved.State != "needs_input" || saved.Attempt != attempt || saved.ErrorCode != "document_invalid" {
			t.Fatalf("wrong recovery state %+v", saved)
		}
	}
	if len(e.ListDocuments()) != 0 {
		t.Fatal("failed parse published")
	}
}

func TestEngineImportQueuedCancellationAndShutdown(t *testing.T) {
	e := NewEngine(nil, t.TempDir())
	defer e.Close()
	s, _ := e.ImportStore()
	r := acceptedImport(t, s, "alice", "queued")
	e.imports.gate = make(chan struct{}, 1)
	e.imports.gate <- struct{}{}
	e.imports.running = map[string]context.CancelFunc{}
	done := make(chan error, 1)
	go func() { _, err := e.RunImport(context.Background(), "alice", r.ID); done <- err }()
	deadline := time.Now().Add(2 * time.Second)
	for {
		saved, _ := s.Import("alice", r.ID)
		if saved.State == "queued" {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("import did not queue")
		}
		time.Sleep(time.Millisecond)
	}
	if err := e.StopImport("bob", r.ID, false); !errors.Is(err, ErrImportNotFound) {
		t.Fatal("cross-owner cancellation")
	}
	if err := e.StopImport("alice", r.ID, false); err != nil {
		t.Fatal(err)
	}
	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Fatal(err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("queued cancellation blocked")
	}
	saved, _ := s.Import("alice", r.ID)
	if saved.State != "cancelled" {
		t.Fatal("cancellation lost")
	}
	<-e.imports.gate
}
