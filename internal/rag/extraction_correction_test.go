package rag

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/takuphilchan/offgrid-llm/internal/document"
)

func correctionFixture(t *testing.T) (*Engine, *Document, *CollectionPolicy) {
	t.Helper()
	e := NewEngine(nil, t.TempDir())
	t.Cleanup(func() { e.Close() })
	e.enabled = true
	e.embeddingModel = "fixture"
	e.embeddingIdentity = "fixture-sha"
	r, err := NewDocumentParser().Parse([]byte(strings.Repeat("Preserved clause. ", 40)+"\n"+strings.Repeat("Review clause. ", 40)+"17.25"), "policy.txt", ".txt")
	if err != nil {
		t.Fatal(err)
	}
	text, ranges, err := parsedSourceRanges(r)
	if err != nil {
		t.Fatal(err)
	}
	d := &Document{ID: "policy", Name: "Policy", RawContent: text, SourceRanges: ranges, Metadata: r.Metadata, Extraction: r.Structure, CreatedAt: time.Now(), UpdatedAt: time.Now()}
	chunks, err := e.chunker.chunkDocument(d)
	if err != nil {
		t.Fatal(err)
	}
	vectors := make([][]float32, len(chunks))
	for i := range vectors {
		vectors[i] = []float32{1, 0}
	}
	s := e.store.(*SQLiteStore)
	identity := IndexMetadata{SchemaVersion: IndexSchemaVersion, EmbeddingModel: e.embeddingModel, EmbeddingIdentity: e.embeddingIdentity, EmbeddingDim: 2, ChunkerVersion: chunkerVersion, ParserVersion: parserVersion, UpdatedAt: time.Now()}
	if err = s.PublishDocument(d, chunks, vectors, identity, false); err != nil {
		t.Fatal(err)
	}
	p, err := s.CreateCollection("owner", "Policy records", []string{"reader"}, []string{"editor"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.AssignDocumentCollection("owner", d.ID, p.ID, 1); err != nil {
		t.Fatal(err)
	}
	return e, d, p
}

func correctionFor(d *Document) []document.Correction {
	b := d.Extraction.Blocks[1]
	text := d.RawContent[b.Span.Start:b.Span.End]
	return []document.Correction{{BlockID: b.ID, ExpectedSHA256: document.TextSHA256(text), Text: strings.ReplaceAll(text, "17.25", "18.250")}}
}

func TestCorrectionReusesUnchangedVectorsAndRetainsCitation(t *testing.T) {
	e, d, _ := correctionFixture(t)
	old := d.ExtractionRevision
	b := d.Extraction.Blocks[1]
	original := d.RawContent[b.Span.Start:b.Span.End]
	generated := 0
	embed := func(_ context.Context, texts []string) ([][]float32, error) {
		generated += len(texts)
		out := make([][]float32, len(texts))
		for i, text := range texts {
			if strings.Contains(text, "Preserved clause") {
				t.Error("unchanged paragraph re-embedded")
			}
			out[i] = []float32{0, 1}
		}
		return out, nil
	}
	updated, err := e.correctDocument(e.ForActor("editor").context(context.Background()), d.ID, old, correctionFor(d), embed)
	if err != nil {
		t.Fatal(err)
	}
	if generated != 1 || updated.ExtractionRevision == old || !strings.Contains(updated.RawContent, "18.250") {
		t.Fatal("incorrect incremental publication", generated)
	}
	source := d.SourceRanges[0]
	_, text, err := e.ForActor("reader").ReadSourcePassage(d.ID, b.Span.Start, b.Span.End, source.SHA256, source.Revision, GenerateContentHash(original))
	if err != nil || text != original {
		t.Fatal("old citation no longer readable", err)
	}
	if _, err = e.correctDocument(e.ForActor("editor").context(context.Background()), d.ID, old, correctionFor(d), embed); !errors.Is(err, ErrExtractionConflict) {
		t.Fatal("stale correction replaced newer extraction", err)
	}
	if _, err = e.correctDocument(e.ForActor("reader").context(context.Background()), d.ID, updated.ExtractionRevision, correctionFor(d), embed); !errors.Is(err, ErrCollectionUnavailable) {
		t.Fatal("reader changed evidence", err)
	}
}

func TestCorrectionRevocationDuringEmbeddingPreventsPublication(t *testing.T) {
	e, d, p := correctionFixture(t)
	s := e.store.(*SQLiteStore)
	embed := func(_ context.Context, texts []string) ([][]float32, error) {
		if _, err := s.UpdateCollection("owner", p.ID, p.Revision, p.Name, p.Readers, nil); err != nil {
			t.Fatal(err)
		}
		out := make([][]float32, len(texts))
		for i := range out {
			out[i] = []float32{0, 1}
		}
		return out, nil
	}
	_, err := e.correctDocument(e.ForActor("editor").context(context.Background()), d.ID, d.ExtractionRevision, correctionFor(d), embed)
	if !errors.Is(err, ErrCollectionUnavailable) {
		t.Fatal("revoked writer published", err)
	}
	current, err := s.GetDocument(d.ID)
	if err != nil || current.ExtractionRevision != d.ExtractionRevision || current.RawContent != d.RawContent {
		t.Fatal("revocation changed source", err)
	}
	var n int
	if err = s.db.QueryRow(`SELECT COUNT(*) FROM document_extractions WHERE document_id=?`, d.ID).Scan(&n); err != nil || n != 1 {
		t.Fatal("revoked correction retained private artifacts", n, err)
	}
}
