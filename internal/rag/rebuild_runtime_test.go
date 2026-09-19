package rag

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/takuphilchan/offgrid-llm/internal/inference"
)

func TestRealEmbeddingRebuildPreservesSourcesAndResumes(t *testing.T) {
	model, runtime := os.Getenv("OFFGRID_TEST_EMBEDDING_MODEL"), os.Getenv("OFFGRID_LLAMA_SERVER_PATH")
	if model == "" || runtime == "" {
		t.Skip("set explicit local embedding model and runtime for qualification")
	}
	dir := t.TempDir()
	s := seedRebuildStore(t, filepath.Join(dir, "rag"), "Cats and kittens sleep on sofas and couches.", []float32{1, 0})
	if err := s.SetIndexMetadata(IndexMetadata{SchemaVersion: 2, EmbeddingModel: "bge-m3", EmbeddingDim: 2}); err != nil {
		t.Fatal(err)
	}
	_ = s.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	interrupted, stop := context.WithCancel(ctx)
	if err := RebuildKnowledgeOffline(interrupted, dir, "bge-m3", model, runtime, func(done, total int) { stop() }); err == nil {
		t.Fatal("cancelled rebuild committed")
	}
	check, err := NewSQLiteStore(filepath.Join(dir, "rag"))
	if err != nil {
		t.Fatal(err)
	}
	meta, _, _ := check.GetIndexMetadata()
	_ = check.Close()
	if meta.SchemaVersion != 2 {
		t.Fatal("interrupted rebuild replaced original")
	}
	if err := RebuildKnowledgeOffline(ctx, dir, "bge-m3", model, runtime, nil); err != nil {
		t.Fatal(err)
	}
	worker := inference.NewEmbeddingEngine(t.TempDir())
	defer worker.Unload()
	e := NewEngine(worker, dir)
	defer e.Close()
	e.SetModelResolver(func(string) (string, error) { return model, nil })
	if err := e.Enable(ctx, "bge-m3"); err != nil {
		t.Fatal(err)
	}
	result, err := e.Search(ctx, "Where does a kitten rest?", DefaultSearchOptions())
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Results) == 0 || result.Results[0].DocumentID != "one" {
		t.Fatalf("retrieval failed: %+v", result)
	}
	doc := e.GetDocument("one")
	if doc == nil || doc.RawContent != "Cats and kittens sleep on sofas and couches." {
		t.Fatal("lost source")
	}
	t.Logf("rebuilt and resumed real embedding index; %d retrieval results", len(result.Results))
}
