package rag

import (
	"fmt"
	"strings"
	"testing"
	"time"
)

func TestSQLiteSelectedDocumentKeepsKeywordRankWithManyExcludedMatches(t *testing.T) {
	store, err := NewSQLiteStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	now := time.Now().UTC()
	add := func(id, content string) {
		t.Helper()
		doc := &Document{ID: id, Name: id, IndexStatus: "ready", CreatedAt: now, UpdatedAt: now}
		chunk := &Chunk{ID: id + "-chunk", DocumentID: id, Content: content, CreatedAt: now}
		if err := store.AddDocumentWithChunks(doc, []*Chunk{chunk}, [][]float32{{0, 1}}); err != nil {
			t.Fatal(err)
		}
	}
	add("selected", "harvest "+strings.Repeat("background ", 100))
	for i := 0; i < 20; i++ {
		add(fmt.Sprintf("excluded-%02d", i), "harvest")
	}
	results, err := store.HybridSearchWithOptions([]float32{1, 0}, "harvest", SearchOptions{
		TopK: 1, MinScore: 0.5, DocumentFilter: []string{"selected"}, IncludeContent: true,
	}, 0)
	if err != nil || len(results) != 1 || results[0].DocumentID != "selected" || results[0].Score != 1 {
		t.Fatalf("excluded documents consumed keyword candidates: %+v %v", results, err)
	}
	// With no selection the keyword-only request must still find matches,
	// even though every synthetic vector has zero query similarity. This
	// catches a broken FTS query silently becoming semantic-only retrieval.
	results, err = store.HybridSearchWithOptions([]float32{1, 0}, "harvest", SearchOptions{
		TopK: 10, MinScore: 0.5, IncludeContent: true,
	}, 0)
	if err != nil || len(results) != 10 {
		t.Fatal("keyword retrieval silently fell back to zero semantic matches", results, err)
	}
	for _, result := range results {
		if !strings.HasPrefix(result.DocumentID, "excluded-") {
			t.Fatal("fixture must crowd the selected source out of the global top ten", result.DocumentID)
		}
	}
	// IDs remain data even when callers supply SQL-looking text.
	results, err = store.HybridSearchWithOptions([]float32{1, 0}, "harvest", SearchOptions{
		TopK: 1, DocumentFilter: []string{"selected') OR 1=1 --"}, IncludeContent: true,
	}, 0)
	if err != nil || len(results) != 0 {
		t.Fatal("document selection broadened", results, err)
	}
}
