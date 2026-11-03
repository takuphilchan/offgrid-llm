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

func TestRebuildCannotResurrectDeletedSources(t *testing.T) {
	live := seedRebuildStore(t, filepath.Join(t.TempDir(), "live"), "removed source", []float32{1, 0})
	stage := seedRebuildStore(t, filepath.Join(t.TempDir(), "stage"), "removed source", []float32{0, 1})
	if err := live.DeleteDocument("one"); err != nil {
		t.Fatal(err)
	}
	if err := live.publishStagedIndex(context.Background(), stage.dbPath); err == nil {
		t.Fatal("staging resurrected a deleted document")
	}
	if doc, err := live.GetDocument("one"); err != nil || doc != nil {
		t.Fatal("deleted source returned", doc, err)
	}
	if results, err := live.Search([]float32{0, 1}, 10, 0); err != nil || len(results) != 0 {
		t.Fatal("staged vectors became searchable", results, err)
	}
}

func TestRebuildPreservesCurrentAccessAndRetention(t *testing.T) {
	live := seedRebuildStore(t, filepath.Join(t.TempDir(), "live"), "private source", []float32{1, 0})
	stage := seedRebuildStore(t, filepath.Join(t.TempDir(), "stage"), "private source", []float32{0, 1})
	policy, err := live.CreateCollection("owner", "Private records", []string{"reader"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := live.AssignDocumentCollection("owner", "one", policy.ID, 1); err != nil {
		t.Fatal(err)
	}
	if _, err := live.SetDocumentRetention("owner", "one", 1, 1, true); err != nil {
		t.Fatal(err)
	}
	if _, err := live.UpdateCollection("owner", policy.ID, policy.Revision, policy.Name, nil, nil); err != nil {
		t.Fatal(err)
	}
	// Staging has default shared policy and no hold. Publishing its derived
	// index must never restore those obsolete authority/retention defaults.
	if err := live.publishStagedIndex(context.Background(), stage.dbPath); err != nil {
		t.Fatal(err)
	}
	if err := live.DocumentAccess("reader", "one", false); !errors.Is(err, ErrCollectionUnavailable) {
		t.Fatal("rebuild restored revoked access", err)
	}
	if err := live.DocumentAccess("owner", "one", false); err != nil {
		t.Fatal(err)
	}
	if err := live.DeleteDocument("one"); !errors.Is(err, ErrDocumentHeld) {
		t.Fatal("rebuild discarded legal hold", err)
	}
	if n, err := live.expireDocuments(time.Now(), 100); err != nil || n != 0 {
		t.Fatal("rebuild made held source eligible for expiry", n, err)
	}
}
