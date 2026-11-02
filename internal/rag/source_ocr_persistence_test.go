package rag

import (
	"errors"
	"reflect"
	"testing"
	"time"
)

// Called with actual worker results by TestParserOCRRealWorker. This verifies
// mechanical citation fidelity; synthetic vectors do not qualify relevance.
func assertOCRCitationPersistence(t *testing.T, parsed *ParseResult) {
	t.Helper()
	text, ranges, err := parsedSourceRanges(parsed)
	if err != nil {
		t.Fatal(err)
	}
	doc := &Document{ID: GenerateDocumentID([]byte(text)), Name: "ocr.pdf", RawContent: text, SourceRanges: ranges, CreatedAt: time.Now(), UpdatedAt: time.Now()}
	chunks, err := NewChunker(ChunkingOptions{ChunkSize: 17, ChunkOverlap: 5}).chunkDocument(doc)
	if err != nil || len(chunks) == 0 {
		t.Fatal("OCR source did not produce bounded page chunks", err)
	}
	vectors := make([][]float32, len(chunks))
	for i, chunk := range chunks {
		vectors[i] = []float32{1, 0}
		chunk.CreatedAt = time.Now()
	}
	dir := t.TempDir()
	store, err := NewSQLiteStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	if err = store.AddDocumentWithChunks(doc, chunks, vectors); err != nil {
		store.Close()
		t.Fatal(err)
	}
	store.Close()
	store, err = NewSQLiteStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	saved, err := store.GetDocument(doc.ID)
	if err != nil || !reflect.DeepEqual(saved.SourceRanges, ranges) {
		t.Fatal("OCR identity changed after restart", err)
	}
	results, err := store.Search([]float32{1, 0}, len(chunks), 0)
	if err != nil || len(results) != len(chunks) {
		t.Fatal("OCR source chunks lost", err)
	}
	for _, result := range results {
		loc := result.Locator
		if loc.Source == nil || loc.Page != loc.Source.Index {
			t.Fatal("OCR citation lost page identity", loc)
		}
		got, err := saved.ReadSourcePassage(loc.StartChar, loc.EndChar, loc.Source.SHA256, loc.Source.Revision, loc.ContentHash)
		if err != nil || got != result.Chunk.Content {
			t.Fatal("OCR citation changed text", err)
		}
		if _, err = saved.ReadSourcePassage(loc.StartChar, loc.EndChar, loc.Source.SHA256, "obsolete-ocr", loc.ContentHash); err == nil {
			t.Fatal("changed OCR revision accepted as the original citation")
		}
	}
}

func assertPartialOCRReviewPersistence(t *testing.T, source []byte, parsed *ParseResult) {
	t.Helper()
	if _, _, err := parsedSourceRanges(parsed); err == nil {
		t.Fatal("partial OCR accepted for complete indexing")
	}
	dir := t.TempDir()
	store, err := NewSQLiteStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	// Mirror the durable parser boundary using the real original and worker
	// result; no fake ready document or fabricated OCR passage is inserted.
	record, err := store.AcceptImport("alice", "partial-ocr", "mixed.pdf", source)
	if err != nil {
		store.Close()
		t.Fatal(err)
	}
	record, err = store.beginImport("alice", record.ID)
	if err != nil {
		store.Close()
		t.Fatal(err)
	}
	lease := importLease{id: record.ID, owner: "alice", attempt: record.Attempt}
	if err = store.advanceImport(lease, "queued", "extracting", ""); err == nil {
		err = store.saveImportExtraction(lease, parsed)
	}
	if err == nil {
		err = store.finishImportFailure(lease, "needs_input", "document_ocr_language_missing")
	}
	store.Close()
	if err != nil {
		t.Fatal(err)
	}
	store, err = NewSQLiteStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	preview, err := store.ImportExtraction("alice", record.ID)
	if err != nil || !reflect.DeepEqual(preview.Units, parsed.SourceUnits) || preview.SHA256 != record.SHA256 {
		t.Fatal("partial OCR page identity/text lost", err)
	}
	if _, err = store.ImportExtraction("bob", record.ID); !errors.Is(err, ErrImportNotFound) {
		t.Fatal("partial OCR source crossed ownership boundary", err)
	}
	if doc, err := store.GetDocument(GenerateDocumentID(source)); err != nil || doc != nil {
		t.Fatal("review-only OCR published a document", err)
	}
}
