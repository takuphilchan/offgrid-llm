package rag

import (
	"testing"
	"time"
)

func TestLiteralSearchQuestionsAndFailures(t *testing.T) {
	s, err := NewSQLiteStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	now := time.Now().UTC()
	doc := &Document{ID: "policy", Name: "Policy", IndexStatus: "ready", CreatedAt: now, UpdatedAt: now}
	chunk := &Chunk{ID: "clause", DocumentID: doc.ID, Content: "Harvest allocation. UK case 42: 青岚项目预算为七百万元。", CreatedAt: now}
	if err = s.AddDocumentWithChunks(doc, []*Chunk{chunk}, [][]float32{{0, 1}}); err != nil {
		t.Fatal(err)
	}
	for _, query := range []string{"harvest?", "What is the harvest allocation?", "UK 42", "青岚项目预算是多少？", `"harvest allocation"`, "harvest OR NOT (42*)"} {
		results, err := s.HybridSearchWithOptions([]float32{1, 0}, query, SearchOptions{TopK: 5, MinScore: .2, IncludeContent: true}, .7)
		if err != nil || len(results) != 1 {
			t.Errorf("%q: %v %v", query, results, err)
		}
	}
	if _, err = s.db.Exec(`DROP TABLE chunk_terms`); err != nil {
		t.Fatal(err)
	}
	if _, err = s.HybridSearchWithOptions([]float32{1, 0}, "harvest?", SearchOptions{TopK: 5}, .7); err == nil {
		t.Fatal("keyword failure silently degraded")
	}
}
