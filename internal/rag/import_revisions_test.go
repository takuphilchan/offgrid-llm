package rag

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/takuphilchan/offgrid-llm/internal/document"
	"github.com/takuphilchan/offgrid-llm/internal/inference"
)

func importedCorrectionFixture(t *testing.T) ([]byte, []*document.Extraction) {
	t.Helper()
	data := []byte("Applications close in June.\nRural exception: July.")
	parsed, err := NewDocumentParser().Parse(data, "policy.txt", ".txt")
	if err != nil {
		t.Fatal(err)
	}
	parent := parsed.Structure
	b := parent.Blocks[len(parent.Blocks)-1]
	next, err := parent.Correct("owner", time.Now().UTC(), []document.Correction{{BlockID: b.ID, ExpectedSHA256: document.TextSHA256(parent.Text[b.Span.Start:b.Span.End]), Text: "Rural exception: August."}})
	if err != nil {
		t.Fatal(err)
	}
	return data, []*document.Extraction{parent, next}
}

func TestImportRevisionsRealIndexAndHistoricalSources(t *testing.T) {
	model := os.Getenv("OFFGRID_TEST_EMBEDDING_MODEL")
	if model == "" || os.Getenv("OFFGRID_LLAMA_SERVER_PATH") == "" {
		t.Skip("requires explicitly selected local embedding model/runtime")
	}
	data, history := importedCorrectionFixture(t)
	cache := t.TempDir()
	t.Setenv("LOCALAPPDATA", cache)
	t.Setenv("XDG_CACHE_HOME", cache)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	worker := inference.NewEmbeddingEngine(t.TempDir())
	defer worker.Unload()
	e := NewEngine(worker, t.TempDir())
	defer e.Close()
	e.SetModelResolver(func(string) (string, error) { return model, nil })
	if err := e.Enable(ctx, "correction-qualification"); err != nil {
		t.Fatal(err)
	}
	store, err := e.ImportStore()
	if err != nil {
		t.Fatal(err)
	}
	r, err := store.AcceptImportWithRevisions("owner", "real", "policy.txt", data, WorkspaceCollection, "eng", history, 1)
	if err != nil {
		t.Fatal(err)
	}
	doc, err := e.RunImport(ctx, "owner", r.ID)
	if err != nil || doc.ExtractionRevision != history[1].ID || doc.RawContent != history[1].Text {
		t.Fatal("correction lost during real indexing", err)
	}
	for _, graph := range history {
		b := graph.Blocks[len(graph.Blocks)-1]
		text := graph.Text[b.Span.Start:b.Span.End]
		_, actual, err := e.ForActor("owner").ReadSourcePassage(doc.ID, b.Span.Start, b.Span.End, graph.OriginalSHA256, graph.ID, GenerateContentHash(text))
		if err != nil || actual != text {
			t.Fatal("retained source changed", err)
		}
	}
	again, err := e.RunImport(ctx, "owner", r.ID)
	if err != nil || again.ID != doc.ID {
		t.Fatal("ready import replayed", err)
	}
}

func TestImportRevisionsAtomicPublicationRetainsAllEvidence(t *testing.T) {
	data, history := importedCorrectionFixture(t)
	s, err := NewSQLiteStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	r, err := s.AcceptImportWithRevisions("owner", "key", "policy.txt", data, WorkspaceCollection, "eng", history, 1)
	if err != nil {
		t.Fatal(err)
	}
	active, err := s.beginImport("owner", r.ID)
	if err != nil {
		t.Fatal(err)
	}
	lease := importLease{id: r.ID, owner: "owner", attempt: active.Attempt, collection: WorkspaceCollection}
	if err = s.advanceImport(lease, "queued", "extracting", ""); err != nil {
		t.Fatal(err)
	}
	loaded, err := s.importRevisions(lease, r.SHA256)
	if err != nil || len(loaded) != 2 || loaded[1].ID != history[1].ID {
		t.Fatal("lost retained corrections", err)
	}
	if _, err = s.importRevisions(importLease{id: r.ID, owner: "other", attempt: active.Attempt, collection: WorkspaceCollection}, r.SHA256); err == nil {
		t.Fatal("history crossed owner boundary")
	}
	if err = s.advanceImport(lease, "extracting", "indexing", ""); err != nil {
		t.Fatal(err)
	}
	doc := originalFixture(data)
	doc.collection = WorkspaceCollection
	doc.publicationActor = &knowledgePrincipal{actor: "owner"}
	doc.ID = collectionDocumentID(WorkspaceCollection, data)
	doc.original.importRef = &lease
	doc.original.revisions = history[:1]
	doc.RawContent = history[1].Text
	doc.Extraction = history[1]
	doc.Metadata = revisionMetadata(history[1])
	doc.SourceRanges, err = revisionSourceRanges(history[1])
	if err != nil {
		t.Fatal(err)
	}
	chunk, index := publicationFixture(doc)
	if err = s.PublishDocument(doc, []*Chunk{chunk}, [][]float32{{1, 0}}, index, false); err != nil {
		t.Fatal(err)
	}
	for _, g := range history {
		retained, err := s.extraction(doc.ID, g.ID)
		if err != nil || retained.Text != g.Text || retained.Verify() != nil {
			t.Fatal("history not atomically published", err)
		}
	}
	original, err := s.original(doc.ID)
	if err != nil || string(original.Bytes) != string(data) {
		t.Fatal("original changed", err)
	}
	again, err := s.AcceptImportWithRevisions("owner", "key", "policy.txt", data, WorkspaceCollection, "eng", history, 1)
	if err != nil || again.State != "ready" || again.ID != r.ID {
		t.Fatal("lost acknowledgement", err)
	}
	if _, err = s.AcceptImportWithRevisions("owner", "key", "policy.txt", data, WorkspaceCollection, "eng", nil, 1); !errors.Is(err, ErrImportConflict) {
		t.Fatal("revision key reused", err)
	}
	var size int
	if err = s.db.QueryRow(`SELECT COALESCE(LENGTH(history),0) FROM document_import_revisions WHERE import_id=?`, r.ID).Scan(&size); err != nil || size != 0 {
		t.Fatal("staged history not released", err)
	}
}

func TestImportRevisionsRejectInvalidOrChangedHistory(t *testing.T) {
	data, history := importedCorrectionFixture(t)
	s, err := NewSQLiteStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	for _, chain := range [][]*document.Extraction{{history[1]}, {history[0], history[0]}, {nil}} {
		if _, err = s.AcceptImportWithRevisions("owner", "bad", "policy.txt", data, WorkspaceCollection, "eng", chain, 1); err == nil {
			t.Fatal("invalid history accepted")
		}
	}
	r, err := s.AcceptImportWithRevisions("owner", "good", "policy.txt", data, WorkspaceCollection, "eng", history, 1)
	if err != nil {
		t.Fatal(err)
	}
	active, err := s.beginImport("owner", r.ID)
	if err != nil {
		t.Fatal(err)
	}
	lease := importLease{id: r.ID, owner: "owner", attempt: active.Attempt, collection: WorkspaceCollection}
	if err = s.advanceImport(lease, "queued", "extracting", ""); err != nil {
		t.Fatal(err)
	}
	if _, err = s.db.Exec(`UPDATE document_import_revisions SET history_sha256='changed' WHERE import_id=?`, r.ID); err != nil {
		t.Fatal(err)
	}
	if _, err = s.importRevisions(lease, r.SHA256); err == nil {
		t.Fatal("tampered history reparsed or accepted")
	}
	if err = s.stopImport("owner", r.ID, true); err != nil {
		t.Fatal(err)
	}
	var n int
	if err = s.db.QueryRowContext(context.Background(), `SELECT COUNT(*) FROM document_import_revisions`).Scan(&n); err != nil || n != 0 {
		t.Fatal("retained deleted history", err)
	}
}
