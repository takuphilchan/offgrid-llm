package rag

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/takuphilchan/offgrid-llm/internal/storage"
)

func seedRebuildStore(t *testing.T, dir, content string, vector []float32) *SQLiteStore {
	t.Helper()
	s, err := NewSQLiteStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })
	doc := &Document{ID: "one", Name: "document", RawContent: content, IndexStatus: "ready", ChunkCount: 1, CreatedAt: time.Now(), UpdatedAt: time.Now()}
	chunk := &Chunk{ID: "chunk-one", DocumentID: doc.ID, Content: content, CreatedAt: time.Now()}
	if err := s.AddDocumentWithChunks(doc, []*Chunk{chunk}, [][]float32{vector}); err != nil {
		t.Fatal(err)
	}
	if err := s.SetIndexMetadata(IndexMetadata{SchemaVersion: IndexSchemaVersion, EmbeddingDim: 2, EmbeddingIdentity: "verified", EmbeddingModel: "model"}); err != nil {
		t.Fatal(err)
	}
	return s
}

func TestPublishStagedIndexIsAtomicAndChecksSources(t *testing.T) {
	for _, failure := range []string{"success", "different-source", "bad-dimensions", "write-failure", "cancelled"} {
		t.Run(failure, func(t *testing.T) {
			live := seedRebuildStore(t, filepath.Join(t.TempDir(), "live"), "original", []float32{1, 0})
			content := "original"
			vector := []float32{0, 1}
			if failure == "different-source" {
				content = "changed"
			}
			if failure == "bad-dimensions" {
				vector = []float32{1}
			}
			stage := seedRebuildStore(t, filepath.Join(t.TempDir(), "stage"), content, vector)
			if failure == "write-failure" {
				if _, err := live.db.Exec(`CREATE TRIGGER reject_publish BEFORE INSERT ON chunks BEGIN SELECT RAISE(ABORT,'injected write failure'); END`); err != nil {
					t.Fatal(err)
				}
			}
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			if failure == "cancelled" {
				cancel()
			}
			err := live.publishStagedIndex(ctx, stage.dbPath)
			if (err == nil) != (failure == "success") {
				t.Fatalf("publication error: %v", err)
			}
			var stored string
			if err := live.db.QueryRow(`SELECT embedding FROM chunks WHERE id='chunk-one'`).Scan(&stored); err != nil {
				t.Fatal(err)
			}
			want := "[1,0]"
			if failure == "success" {
				want = "[0,1]"
			}
			if stored != want {
				t.Fatalf("vectors=%s, want %s", stored, want)
			}
			doc, _ := live.GetDocument("one")
			if doc.RawContent != "original" {
				t.Fatal("source modified")
			}
		})
	}
}

func TestOfflineRebuildRefusesActiveWorkspace(t *testing.T) {
	dir := t.TempDir()
	owner, err := storage.AcquireOwnership(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer owner.Close()
	err = RebuildKnowledgeOffline(context.Background(), dir, "model", "unused", "unused", nil)
	if !errors.Is(err, storage.ErrWorkspaceInUse) {
		t.Fatalf("active workspace accepted: %v", err)
	}
}
