package rag

import (
	"encoding/json"
	"fmt"
	"math"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestVectorProjectionExactAndRejectsDamage(t *testing.T) {
	values := []float32{0, -1, 0.338, 1.25, math.SmallestNonzeroFloat32}
	data := encodeVector(values)
	got, err := decodeVector(data)
	if err != nil || !reflect.DeepEqual(got, values) {
		t.Fatal(got, err)
	}
	for _, bad := range [][]byte{nil, data[:len(data)-1], append([]byte("BAD!"), data[4:]...), encodeVector([]float32{float32(math.NaN())})} {
		if _, err := decodeVector(bad); err == nil {
			t.Fatal("damaged vector accepted")
		}
	}
}

func TestDerivedSearchMigrationPreservesOldIndexAndSource(t *testing.T) {
	dir := t.TempDir()
	s, err := NewSQLiteStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	doc := &Document{ID: "policy", Name: "Policy", RawContent: "Harvest allocation 42", IndexStatus: "ready", CreatedAt: now, UpdatedAt: now}
	chunk := &Chunk{ID: "clause", DocumentID: doc.ID, Content: doc.RawContent, CreatedAt: now}
	if err = s.AddDocumentWithChunks(doc, []*Chunk{chunk}, [][]float32{{0, 1}}); err != nil {
		t.Fatal(err)
	}
	// Simulate an old index with no derived vector or multilingual terms.
	if _, err = s.db.Exec(`UPDATE chunks SET embedding_v1=NULL,search_terms=''`); err != nil {
		t.Fatal(err)
	}
	s.Close()
	s, err = NewSQLiteStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	var original, projection []byte
	var text string
	if err = s.db.QueryRow(`SELECT embedding,embedding_v1,content FROM chunks WHERE id='clause'`).Scan(&original, &projection, &text); err != nil {
		t.Fatal(err)
	}
	var before []float32
	_ = json.Unmarshal(original, &before)
	after, err := decodeVector(projection)
	if err != nil || !reflect.DeepEqual(before, after) || text != doc.RawContent {
		t.Fatal("migration changed evidence/vector")
	}
	results, err := s.HybridSearch([]float32{1, 0}, "harvest?", 5, .2, .7)
	if err != nil || len(results) != 1 {
		t.Fatal(results, err)
	}
	if _, err = s.db.Exec(`UPDATE chunks SET embedding='[1,0]' WHERE id='clause'`); err != nil {
		t.Fatal(err)
	}
	var stale int
	if err = s.db.QueryRow(`SELECT COUNT(*) FROM chunks WHERE embedding_v1 IS NOT NULL`).Scan(&stale); err != nil || stale != 0 {
		t.Fatal("stale vector projection", stale, err)
	}
	results, err = s.Search([]float32{1, 0}, 5, .9)
	if err != nil || len(results) != 1 {
		t.Fatal("updated compatibility vector not used", results, err)
	}
}

func TestBoundedFusionReadsOnlyCandidateBodies(t *testing.T) {
	s, err := NewSQLiteStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	now := time.Now().UTC()
	doc := &Document{ID: "good", Name: "Good", CreatedAt: now, UpdatedAt: now}
	chunks := make([]*Chunk, 80)
	vectors := make([][]float32, 80)
	for i := range chunks {
		chunks[i] = &Chunk{ID: fmt.Sprintf("chunk-%03d", i), DocumentID: doc.ID, Content: fmt.Sprintf("Administrative record %d", i), CreatedAt: now}
		vectors[i] = []float32{1, float32(i + 1)}
	}
	if err = s.AddDocumentWithChunks(doc, chunks, vectors); err != nil {
		t.Fatal(err)
	}
	// A low-ranked document has deliberately invalid metadata. It must not be
	// materialized at all for this query, unlike the former full-body scan.
	other := &Document{ID: "unrelated", Name: "Unrelated", CreatedAt: now, UpdatedAt: now}
	if err = s.AddDocumentWithChunks(other, []*Chunk{{ID: "unrelated-chunk", DocumentID: other.ID, Content: strings.Repeat("unrelated ", 1000), CreatedAt: now}}, [][]float32{{-1, 0}}); err != nil {
		t.Fatal(err)
	}
	if _, err = s.db.Exec(`UPDATE documents SET source_ranges='invalid' WHERE id='unrelated'`); err != nil {
		t.Fatal(err)
	}
	results, err := s.HybridSearchWithOptions([]float32{1, 0}, "missing", SearchOptions{TopK: 5, IncludeContent: true}, .7)
	if err != nil || len(results) != 5 || results[0].Chunk.ID != "chunk-000" {
		t.Fatal(results, err)
	}
	if _, err = s.db.Exec(`UPDATE chunks SET embedding_v1=x'4f475631' WHERE id='chunk-000'`); err != nil {
		t.Fatal(err)
	}
	if _, err = s.HybridSearch([]float32{1, 0}, "administrative", 5, 0, .7); err == nil {
		t.Fatal("corrupt semantic channel silently ignored")
	}
}
