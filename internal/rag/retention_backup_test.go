package rag

import (
	"context"
	"crypto/sha256"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/takuphilchan/offgrid-llm/internal/storage"
)

// Exercise real workspace archives, not a copy of a policy struct. A backup is
// deliberately a separate snapshot: live expiry must neither rewrite it nor
// erase the hold that existed when it was taken.
func TestRetentionAcrossOfflineBackupAndRestore(t *testing.T) {
	root := t.TempDir()
	workspace := filepath.Join(root, "workspace")
	s, err := NewSQLiteStore(filepath.Join(workspace, "rag"))
	if err != nil {
		t.Fatal(err)
	}
	held := originalFixture([]byte("held archive evidence"))
	expires := originalFixture([]byte("expiring archive evidence"))
	for _, doc := range []*Document{held, expires} {
		if err := s.AddDocumentWithChunks(doc, nil, nil); err != nil {
			t.Fatal(err)
		}
		if _, err := s.SetDocumentRetention("owner", doc.ID, 1, 1, doc.ID == held.ID); err != nil {
			t.Fatal(err)
		}
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	archive := filepath.Join(root, "before-expiry.zip")
	const version = "0.4.24"
	if _, err := storage.BackupWorkspace(context.Background(), workspace, archive, version); err != nil {
		t.Fatal(err)
	}
	before, err := os.ReadFile(archive)
	if err != nil {
		t.Fatal(err)
	}
	s, err = NewSQLiteStore(filepath.Join(workspace, "rag"))
	if err != nil {
		t.Fatal(err)
	}
	if n, err := s.expireDocuments(time.Now(), 100); err != nil || n != 1 {
		t.Fatal(n, err)
	}
	if _, err := s.original(expires.ID); !errors.Is(err, ErrOriginalUnavailable) {
		t.Fatal("expired original remained", err)
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	after, err := os.ReadFile(archive)
	if err != nil || sha256.Sum256(before) != sha256.Sum256(after) {
		t.Fatal("live retention modified an independent backup", err)
	}
	if _, err := storage.RestoreWorkspace(context.Background(), archive, workspace, version); err == nil {
		t.Fatal("restore merged old data over the current workspace")
	}
	restored := filepath.Join(root, "restored")
	if _, err := storage.RestoreWorkspace(context.Background(), archive, restored, version); err != nil {
		t.Fatal(err)
	}
	r, err := NewSQLiteStore(filepath.Join(restored, "rag"))
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	if _, err := r.original(expires.ID); err != nil {
		t.Fatal("snapshot unexpectedly lost its original", err)
	}
	if err := r.DeleteDocument(held.ID); !errors.Is(err, ErrDocumentHeld) {
		t.Fatal("restore lost legal hold", err)
	}
	if n, err := r.expireDocuments(time.Now(), 100); err != nil || n != 1 {
		t.Fatal("restored expiry was not reapplied", n, err)
	}
	if _, err := r.original(held.ID); err != nil {
		t.Fatal("held snapshot source disappeared", err)
	}
	if _, err := r.original(expires.ID); !errors.Is(err, ErrOriginalUnavailable) {
		t.Fatal("expired restored source remained", err)
	}
}

func TestConcurrentRetentionAndManualDeletionFencePublication(t *testing.T) {
	s, err := NewSQLiteStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	accepted := acceptedImport(t, s, "alice", "publication")
	doc, _ := indexingImport(t, s, accepted, "alice")
	chunk, index := publicationFixture(doc)
	if err := s.PublishDocument(doc, []*Chunk{chunk}, [][]float32{{1, 0}}, index, false); err != nil {
		t.Fatal(err)
	}
	duplicate := acceptedImport(t, s, "alice", "pending-duplicate")
	stale, _ := indexingImport(t, s, duplicate, "alice")
	if _, err := s.SetDocumentRetention("alice", doc.ID, 1, 1, false); err != nil {
		t.Fatal(err)
	}
	start := make(chan struct{})
	var workers sync.WaitGroup
	for i := 0; i < 12; i++ {
		workers.Add(1)
		go func(expiry bool) {
			defer workers.Done()
			<-start
			var err error
			if expiry {
				_, err = s.expireDocuments(time.Now(), 100)
			} else {
				err = s.DeleteDocument(doc.ID)
			}
			if err != nil {
				t.Error(err)
			}
		}(i%2 == 0)
	}
	close(start)
	workers.Wait()
	if err := s.PublishDocument(stale, []*Chunk{chunk}, [][]float32{{1, 0}}, index, false); err == nil {
		t.Fatal("stale job resurrected deleted content")
	}
	for _, table := range []string{"documents", "chunks", "document_originals", "document_retention", "document_collections"} {
		key := "document_id"
		if table == "documents" {
			key = "id"
		}
		var count int
		if err := s.db.QueryRow("SELECT COUNT(*) FROM "+table+" WHERE "+key+"=?", doc.ID).Scan(&count); err != nil || count != 0 {
			t.Fatal(table, count, err)
		}
	}
	results, err := s.Search([]float32{1, 0}, 10, 0)
	if err != nil || len(results) != 0 {
		t.Fatal("deleted vectors still searchable", results, err)
	}
	if _, err := s.original(doc.ID); !errors.Is(err, ErrOriginalUnavailable) {
		t.Fatal("deleted original still exportable", err)
	}
}
