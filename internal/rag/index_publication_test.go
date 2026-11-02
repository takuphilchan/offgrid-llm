package rag

import (
	"bytes"
	"testing"
	"time"
)

func publicationFixture(doc *Document) (*Chunk, IndexMetadata) {
	return &Chunk{ID: doc.ID + "-chunk", DocumentID: doc.ID, Content: doc.RawContent, CreatedAt: time.Now()}, IndexMetadata{
		SchemaVersion: IndexSchemaVersion, EmbeddingModel: "fixture-model", EmbeddingIdentity: "fixture-digest", EmbeddingDim: 2,
		ChunkerVersion: chunkerVersion, ParserVersion: parserVersion, UpdatedAt: time.Now().UTC(),
	}
}

func TestPublicationMetadataFailureRollsBackOriginalAndIndex(t *testing.T) {
	s, err := NewSQLiteStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	doc := originalFixture([]byte("retained original bytes"))
	chunk, index := publicationFixture(doc)
	if _, err = s.db.Exec(`CREATE TRIGGER fail_identity BEFORE INSERT ON index_metadata BEGIN SELECT RAISE(ABORT,'fixture failure'); END`); err != nil {
		t.Fatal(err)
	}
	if err = s.PublishDocument(doc, []*Chunk{chunk}, [][]float32{{1, 0}}, index, false); err == nil {
		t.Fatal("metadata failure accepted")
	}
	for _, table := range []string{"documents", "chunks", "document_originals", "index_metadata"} {
		var count int
		if err = s.db.QueryRow("SELECT COUNT(*) FROM " + table).Scan(&count); err != nil || count != 0 {
			t.Fatal("partial publication", table, count, err)
		}
	}
}

func TestFailedReindexRetainsOriginalSourceAndSearchableIndex(t *testing.T) {
	dir := t.TempDir()
	s, err := NewSQLiteStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	doc := originalFixture([]byte("original survives reindex failure"))
	chunk, index := publicationFixture(doc)
	if err = s.PublishDocument(doc, []*Chunk{chunk}, [][]float32{{1, 0}}, index, false); err != nil {
		t.Fatal(err)
	}
	if _, err = s.db.Exec(`CREATE TRIGGER fail_identity BEFORE UPDATE ON index_metadata BEGIN SELECT RAISE(ABORT,'fixture failure'); END`); err != nil {
		t.Fatal(err)
	}
	replacement := *doc
	replacement.RawContent = "replacement that must not become visible"
	newChunk := &Chunk{ID: doc.ID + "-replacement", DocumentID: doc.ID, Content: replacement.RawContent, CreatedAt: time.Now()}
	if err = s.PublishDocument(&replacement, []*Chunk{newChunk}, [][]float32{{0, 1}}, index, true); err == nil {
		t.Fatal("failed replacement accepted")
	}
	s.Close()
	s, err = NewSQLiteStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	saved, err := s.GetDocument(doc.ID)
	if err != nil || saved.RawContent != doc.RawContent {
		t.Fatal("source replaced", err)
	}
	original, err := s.original(doc.ID)
	if err != nil || !bytes.Equal(original.Bytes, doc.original.Bytes) {
		t.Fatal("original changed", err)
	}
	results, err := s.Search([]float32{1, 0}, 5, 0)
	if err != nil || len(results) != 1 || results[0].Chunk.ID != chunk.ID {
		t.Fatal("old index lost", err)
	}
	savedIndex, found, err := s.GetIndexMetadata()
	if err != nil || !found || savedIndex.EmbeddingIdentity != index.EmbeddingIdentity {
		t.Fatal("index identity lost", err)
	}
}

func TestPublicationRejectsMixedEmbeddingIdentity(t *testing.T) {
	s, err := NewSQLiteStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	a := originalFixture([]byte("first original"))
	chunk, index := publicationFixture(a)
	if err = s.PublishDocument(a, []*Chunk{chunk}, [][]float32{{1, 0}}, index, false); err != nil {
		t.Fatal(err)
	}
	b := originalFixture([]byte("second original"))
	other, incompatible := publicationFixture(b)
	incompatible.EmbeddingIdentity = "different-runtime-digest"
	if err = s.PublishDocument(b, []*Chunk{other}, [][]float32{{1, 0}}, incompatible, false); err == nil {
		t.Fatal("mixed identity accepted")
	}
	if got, err := s.GetDocument(b.ID); err != nil || got != nil {
		t.Fatal("rejected source published", err)
	}
}
