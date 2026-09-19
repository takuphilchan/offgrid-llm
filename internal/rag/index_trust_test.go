package rag

import (
	"context"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/takuphilchan/offgrid-llm/internal/inference"
)

func TestUnverifiedIndexCannotEnableAndRetainsSource(t *testing.T) {
	for _, schema := range []int{0, 2, IndexSchemaVersion} {
		t.Run(string(rune('0'+schema)), func(t *testing.T) {
			e := NewEngine(inference.NewEmbeddingEngine(), t.TempDir())
			defer e.Close()
			doc := &Document{ID: "retained", Name: "source", RawContent: "Important source", CreatedAt: time.Now(), UpdatedAt: time.Now()}
			if err := e.store.AddDocument(doc); err != nil {
				t.Fatal(err)
			}
			if schema > 0 {
				if err := e.store.(indexMetadataStore).SetIndexMetadata(IndexMetadata{SchemaVersion: schema, EmbeddingModel: "bge", EmbeddingDim: 384}); err != nil {
					t.Fatal(err)
				}
			}
			if err := e.Enable(context.Background(), "bge"); err == nil {
				t.Fatal("accepted unverifiable vectors")
			}
			if e.IsEnabled() {
				t.Fatal("unverified index enabled")
			}
			got, err := e.store.GetDocument(doc.ID)
			if err != nil || got.RawContent != doc.RawContent {
				t.Fatalf("source changed: %v", err)
			}
			if e.Stats()["rebuild_required"] != true {
				t.Fatal("missing recovery status")
			}
		})
	}
}

func TestIndexIdentityRoundTrip(t *testing.T) {
	s, err := NewSQLiteStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	want := IndexMetadata{SchemaVersion: IndexSchemaVersion, EmbeddingModel: "model", EmbeddingIdentity: "model:runtime:pooling", EmbeddingDim: 1024}
	if err := s.SetIndexMetadata(want); err != nil {
		t.Fatal(err)
	}
	got, found, err := s.GetIndexMetadata()
	if err != nil || !found || got.EmbeddingIdentity != want.EmbeddingIdentity {
		t.Fatalf("lost identity: %+v, %v", got, err)
	}
}

func TestContextTruncationPreservesUnicode(t *testing.T) {
	context := &RAGContext{Results: []SearchResult{{DocumentID: "one", DocName: "文書", Chunk: &Chunk{ID: "chunk", Content: strings.Repeat("語", 1000)}}}}
	context.FormatContext()
	context.TruncateContext(501)
	if !utf8.ValidString(context.Context) || !utf8.ValidString(context.Results[0].Chunk.Content) {
		t.Fatal("truncated in the middle of a UTF-8 character")
	}
	context.TruncateContext(-1)
	if context.Context != "" || len(context.Results) != 0 {
		t.Fatal("invalid budget retained context")
	}
}
