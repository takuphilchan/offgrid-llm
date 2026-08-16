package rag

import (
	"testing"
	"time"
)

func TestSQLiteStoreRetainsSourceAndAtomicallyReindexes(t *testing.T) {
	store, err := NewSQLiteStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	now := time.Now().UTC()
	doc := &Document{
		ID: "doc-one", Name: "One", ContentType: "text/plain", RawContent: "original source",
		ContentHash: GenerateContentHash("original source"), SourceRetained: true,
		IndexStatus: "ready", ChunkCount: 1, CreatedAt: now, UpdatedAt: now, IndexedAt: now,
	}
	oldChunk := &Chunk{ID: "old", DocumentID: doc.ID, Content: "old content", ContentHash: GenerateContentHash("old content"), CreatedAt: now}
	if err := store.AddDocumentWithChunks(doc, []*Chunk{oldChunk}, [][]float32{{1, 0}}); err != nil {
		t.Fatal(err)
	}
	loaded, err := store.GetDocument(doc.ID)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.RawContent != "original source" || !loaded.SourceRetained {
		t.Fatalf("source was not retained: %#v", loaded)
	}

	loaded.RawContent = "replacement source"
	loaded.ContentHash = GenerateContentHash(loaded.RawContent)
	loaded.UpdatedAt = now.Add(time.Minute)
	newChunk := &Chunk{ID: "new", DocumentID: doc.ID, Content: "replacement content", ContentHash: GenerateContentHash("replacement content"), CreatedAt: now}
	if err := store.ReplaceDocumentWithChunks(loaded, []*Chunk{newChunk}, [][]float32{{0, 1}}); err != nil {
		t.Fatal(err)
	}
	results, err := store.Search([]float32{0, 1}, 5, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 || results[0].Chunk.ID != "new" {
		t.Fatalf("expected only replacement chunk, got %#v", results)
	}
}

func TestSQLiteHybridSearchHonorsDocumentFilter(t *testing.T) {
	store, err := NewSQLiteStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	now := time.Now().UTC()
	for _, id := range []string{"allowed", "excluded"} {
		doc := &Document{ID: id, Name: id, IndexStatus: "ready", CreatedAt: now, UpdatedAt: now}
		chunk := &Chunk{ID: id + "-chunk", DocumentID: id, Content: "shared searchable phrase", CreatedAt: now}
		if err := store.AddDocumentWithChunks(doc, []*Chunk{chunk}, [][]float32{{1, 0}}); err != nil {
			t.Fatal(err)
		}
	}
	results, err := store.HybridSearchWithOptions([]float32{1, 0}, "shared", SearchOptions{
		TopK: 5, MinScore: 0, DocumentFilter: []string{"allowed"}, IncludeContent: true,
	}, 0.7)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 || results[0].DocumentID != "allowed" {
		t.Fatalf("filter leaked unrelated results: %#v", results)
	}
}
