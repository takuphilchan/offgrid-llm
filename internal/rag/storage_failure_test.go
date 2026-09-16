package rag

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestStorageFailureNeverFallsBackToVolatileKnowledge(t *testing.T) {
	for _, kind := range []string{"corrupt-database", "not-a-directory"} {
		t.Run(kind, func(t *testing.T) {
			root := t.TempDir()
			if kind == "not-a-directory" {
				if err := os.WriteFile(filepath.Join(root, "rag"), []byte("blocked"), 0600); err != nil {
					t.Fatal(err)
				}
			} else {
				if err := os.Mkdir(filepath.Join(root, "rag"), 0700); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(root, "rag", "rag.db"), []byte("not a sqlite database"), 0600); err != nil {
					t.Fatal(err)
				}
			}
			engine := NewEngine(nil, root)
			if engine.StorageError() == nil || engine.store != nil {
				t.Fatal("persistent failure was hidden behind another store")
			}
			if err := engine.Enable(context.Background(), "embedding"); err == nil {
				t.Fatal("enabled broken storage")
			}
			if _, err := engine.IngestText(context.Background(), "notes", "important document contents", nil); err == nil {
				t.Fatal("acknowledged volatile ingestion")
			}
			if _, err := engine.ReindexDocument(context.Background(), "id"); err == nil {
				t.Fatal("reindexed broken storage")
			}
			if _, err := engine.Search(context.Background(), "question", DefaultSearchOptions()); err == nil {
				t.Fatal("searched broken storage")
			}
			if engine.IsEnabled() || engine.Stats()["storage_available"] != false {
				t.Fatalf("incorrect status: %+v", engine.Stats())
			}
			if len(engine.ListDocuments()) != 0 || engine.GetDocument("missing") != nil || engine.DeleteDocument("missing") {
				t.Fatal("unexpected data on unavailable engine")
			}
		})
	}
}
