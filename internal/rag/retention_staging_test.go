package rag

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func retentionStage(t *testing.T, root, digest string) *SQLiteStore {
	t.Helper()
	s, err := NewSQLiteStore(filepath.Join(root, "rag", "rebuild", digest))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

func retentionSource(t *testing.T, s *SQLiteStore, id, content string) {
	t.Helper()
	doc := originalFixture([]byte(id))
	doc.RawContent = content
	chunk := &Chunk{ID: "chunk-" + id, DocumentID: doc.ID, Content: content, CreatedAt: time.Now()}
	if err := s.AddDocumentWithChunks(doc, []*Chunk{chunk}, [][]float32{{1, 0}}); err != nil {
		t.Fatal(err)
	}
}

func assertStagingSource(t *testing.T, s *SQLiteStore, id string, present bool) {
	t.Helper()
	id = GenerateDocumentID([]byte(id))
	doc, err := s.GetDocument(id)
	if err != nil || (doc != nil) != present {
		t.Fatalf("source %s present=%v want=%v: %v", id, doc != nil, present, err)
	}
	for _, table := range []string{"documents", "document_originals", "chunks", "document_retention", "document_collections"} {
		column := "document_id"
		if table == "documents" {
			column = "id"
		}
		var count int
		if err := s.db.QueryRow("SELECT COUNT(*) FROM "+table+" WHERE "+column+"=?", id).Scan(&count); err != nil || (count > 0) != present {
			t.Fatalf("%s retained=%d want=%v: %v", table, count, present, err)
		}
	}
}

func TestRetentionRebuildStagingPreservesCurrentAndHeldSources(t *testing.T) {
	root := t.TempDir()
	e := NewEngine(nil, root)
	defer e.Close()
	live, err := e.ImportStore()
	if err != nil {
		t.Fatal(err)
	}
	stage := retentionStage(t, root, strings.Repeat("a", 64))
	for _, id := range []string{"current", "held", "replaced", "expired", "deleted"} {
		retentionSource(t, live, id, "current text")
		content := "current text"
		if id == "held" || id == "replaced" {
			content = "previous text"
		}
		retentionSource(t, stage, id, content)
	}
	if _, err := live.SetDocumentRetention("owner", GenerateDocumentID([]byte("held")), 1, 1, true); err != nil {
		t.Fatal(err)
	}
	if _, err := live.SetDocumentRetention("owner", GenerateDocumentID([]byte("expired")), 1, 1, false); err != nil {
		t.Fatal(err)
	}
	if err := live.DeleteDocument(GenerateDocumentID([]byte("deleted"))); err != nil {
		t.Fatal(err)
	}
	if n, err := e.ExpireDocuments(time.Now(), 100); err != nil || n != 1 {
		t.Fatal(n, err)
	}
	for _, id := range []string{"current", "held", "replaced", "expired", "deleted"} {
		assertStagingSource(t, stage, id, id == "current" || id == "held")
		assertStagingSource(t, live, id, id != "expired" && id != "deleted")
	}
	results, err := stage.Search([]float32{1, 0}, 10, 0)
	if err != nil || len(results) != 2 {
		t.Fatal("deleted vectors remain searchable", results, err)
	}
}

func TestRetentionRebuildStagingRestartAndBatchProgress(t *testing.T) {
	root := t.TempDir()
	e := NewEngine(nil, root)
	defer e.Close()
	stage := retentionStage(t, root, strings.Repeat("b", 64))
	for i := 0; i < 101; i++ {
		retentionSource(t, stage, fmt.Sprintf("source-%03d", i), "stale cached text")
	}
	if _, err := e.ExpireDocuments(time.Now(), 100); err != nil {
		t.Fatal(err)
	}
	docs, err := stage.ListDocuments()
	if err != nil || len(docs) != 1 {
		t.Fatal("cleanup was not bounded to one hundred sources", len(docs), err)
	}
	if err := e.Close(); err != nil {
		t.Fatal(err)
	}
	e = NewEngine(nil, root)
	defer e.Close()
	e.StartRetention()
	deadline := time.Now().Add(5 * time.Second)
	for e.RetentionStatus().CheckedAt == 0 && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	if status := e.RetentionStatus(); status.CheckedAt == 0 || status.ErrorCode != "" {
		t.Fatal(status)
	}
	for i := 0; i < 101; i++ {
		assertStagingSource(t, stage, fmt.Sprintf("source-%03d", i), false)
	}
}

func TestRetentionRebuildStagingRoundRobin(t *testing.T) {
	root := t.TempDir()
	e := NewEngine(nil, root)
	defer e.Close()
	var stages []*SQLiteStore
	for i := 0; i < 9; i++ {
		s := retentionStage(t, root, fmt.Sprintf("%064x", i))
		retentionSource(t, s, "stale", "cached text")
		stages = append(stages, s)
	}
	for pass := 0; pass < 2; pass++ {
		if _, err := e.ExpireDocuments(time.Now(), 100); err != nil {
			t.Fatal(err)
		}
		assertStagingSource(t, stages[8], "stale", pass == 0)
	}
}

func TestRetentionRebuildStagingFailureAndCancellation(t *testing.T) {
	for _, failure := range []string{"cancelled", "write", "hold", "unknown-policy", "unknown-schema"} {
		t.Run(failure, func(t *testing.T) {
			root := t.TempDir()
			e := NewEngine(nil, root)
			defer e.Close()
			stage := retentionStage(t, root, strings.Repeat("c", 64))
			retentionSource(t, stage, "stale", "cached text")
			switch failure {
			case "write":
				_, err := stage.db.Exec(`CREATE TRIGGER reject_cleanup BEFORE DELETE ON documents BEGIN SELECT RAISE(ABORT,'injected cleanup failure'); END`)
				if err != nil {
					t.Fatal(err)
				}
			case "hold":
				if _, err := stage.SetDocumentRetention("owner", GenerateDocumentID([]byte("stale")), 1, 0, true); err != nil {
					t.Fatal(err)
				}
			case "unknown-policy":
				if _, err := stage.db.Exec(`UPDATE document_retention SET schema_version=99`); err != nil {
					t.Fatal(err)
				}
			case "unknown-schema":
				if _, err := stage.db.Exec(`INSERT INTO knowledge_policy_migrations VALUES(99)`); err != nil {
					t.Fatal(err)
				}
			}
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			if failure == "cancelled" {
				cancel()
			}
			_, err := e.expireDocuments(ctx, time.Now(), 100)
			if err == nil {
				t.Fatal("cleanup failed open")
			}
			if failure == "cancelled" && !errors.Is(err, context.Canceled) {
				t.Fatal(err)
			}
			assertStagingSource(t, stage, "stale", true)
			if failure == "write" {
				if _, err := stage.db.Exec(`DROP TRIGGER reject_cleanup`); err != nil {
					t.Fatal(err)
				}
				if _, err := e.ExpireDocuments(time.Now(), 100); err != nil {
					t.Fatal(err)
				}
				assertStagingSource(t, stage, "stale", false)
			}
		})
	}
}

func TestRetentionRebuildStagingRejectsRedirectedPaths(t *testing.T) {
	for _, kind := range []string{"root", "stage", "database"} {
		t.Run(kind, func(t *testing.T) {
			root := t.TempDir()
			e := NewEngine(nil, root)
			defer e.Close()
			external, err := NewSQLiteStore(t.TempDir())
			if err != nil {
				t.Fatal(err)
			}
			defer external.Close()
			retentionSource(t, external, "private", "outside workspace")
			path := filepath.Join(root, "rag", "rebuild")
			target := filepath.Dir(external.dbPath)
			if kind != "root" {
				path = filepath.Join(path, strings.Repeat("d", 64))
			}
			if kind == "database" {
				path = filepath.Join(path, "rag.db")
				target = external.dbPath
			}
			if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
				t.Fatal(err)
			}
			if err := os.Symlink(target, path); err != nil {
				t.Skipf("symlink creation unavailable: %v", err)
			}
			if _, err := e.ExpireDocuments(time.Now(), 100); err == nil {
				t.Fatal("followed redirected cache")
			}
			assertStagingSource(t, external, "private", true)
		})
	}
}
