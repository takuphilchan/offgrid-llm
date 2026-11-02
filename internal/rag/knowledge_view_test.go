package rag

import (
	"context"
	"errors"
	"reflect"
	"testing"
)

func TestKnowledgeViewsDoNotTurnLegacyCallersIntoPrivateReaders(t *testing.T) {
	e := NewEngine(nil, t.TempDir())
	defer e.Close()
	s := e.store.(*SQLiteStore)
	doc := originalFixture([]byte("Private source bytes for exact download"))
	chunk, _ := publicationFixture(doc)
	doc.ChunkCount = 1
	if err := s.AddDocumentWithChunks(doc, []*Chunk{chunk}, [][]float32{{1, 0}}); err != nil {
		t.Fatal(err)
	}
	p, err := s.CreateCollection("owner", "Records", []string{"reader"}, []string{"editor"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.AssignDocumentCollection("owner", doc.ID, p.ID, 1); err != nil {
		t.Fatal(err)
	}
	for _, view := range []*KnowledgeView{e.viewForContext(context.Background()), e.ForActor("stranger"), e.ForActor("")} {
		if view.GetDocument(doc.ID) != nil || len(view.ListDocuments()) != 0 || view.CanReadDocument(doc.ID) {
			t.Fatal("private metadata visible")
		}
		if _, err := view.Original(doc.ID); !errors.Is(err, ErrCollectionUnavailable) {
			t.Fatal("private original visible", err)
		}
		if view.DeleteDocument(doc.ID) {
			t.Fatal("unrelated actor deleted document")
		}
		if view.Stats()["document_count"] != 0 || view.Stats()["chunk_count"] != 0 {
			t.Fatal("private counts visible")
		}
	}
	reader := e.ForActor("reader")
	if reader.GetDocument(doc.ID) == nil || len(reader.ListDocuments()) != 1 {
		t.Fatal("reader cannot inspect allowed source")
	}
	original, err := reader.Original(doc.ID)
	if err != nil || !reflect.DeepEqual(original.Bytes, doc.original.Bytes) {
		t.Fatal("allowed original changed", err)
	}
	if reader.DeleteDocument(doc.ID) {
		t.Fatal("read permission granted deletion")
	}
	if _, err := s.UpdateCollection("owner", p.ID, p.Revision, p.Name, nil, p.Editors); err != nil {
		t.Fatal(err)
	}
	if reader.GetDocument(doc.ID) != nil || reader.CanReadDocument(doc.ID) {
		t.Fatal("view cached revoked membership")
	}
	if _, err := reader.Original(doc.ID); !errors.Is(err, ErrCollectionUnavailable) {
		t.Fatal("revoked original returned", err)
	}
	if !e.ForActor("editor").DeleteDocument(doc.ID) || e.ForActor("owner").GetDocument(doc.ID) != nil {
		t.Fatal("authorized deletion failed")
	}
}

func TestKnowledgeSearchEnforcesPolicyBeforeRankingAndAfterInference(t *testing.T) {
	e := NewEngine(nil, t.TempDir())
	defer e.Close()
	e.enabled, e.hybridAlpha = true, 0
	s := e.store.(*SQLiteStore)
	p, err := s.CreateCollection("owner", "Private", []string{"reader"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{"public", "private"} {
		doc := originalFixture([]byte(id + " bytes"))
		doc.ID, doc.original, doc.RawContent, doc.Name = id, nil, "harvest", id
		chunk, _ := publicationFixture(doc)
		if err := s.AddDocumentWithChunks(doc, []*Chunk{chunk}, [][]float32{{0, 1}}); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := s.AssignDocumentCollection("owner", "private", p.ID, 1); err != nil {
		t.Fatal(err)
	}
	calls := 0
	embed := func(context.Context, []string) ([][]float32, error) { calls++; return [][]float32{{1, 0}}, nil }
	opts := SearchOptions{TopK: 10, MinScore: 0.1, IncludeContent: true}
	result, err := e.search(e.ForActor("stranger").context(context.Background()), "harvest", opts, embed)
	if err != nil || len(result.Results) != 1 || result.Results[0].DocumentID != "public" {
		t.Fatal("private ranking candidate escaped", result, err)
	}
	opts.DocumentFilter = []string{"private"}
	before := calls
	if _, err := e.search(e.ForActor("stranger").context(context.Background()), "harvest", opts, embed); !errors.Is(err, ErrCollectionUnavailable) || calls != before {
		t.Fatal("denied selection reached inference", err, calls)
	}
	result, err = e.search(e.ForActor("reader").context(context.Background()), "harvest", opts, embed)
	if err != nil || len(result.Results) != 1 || result.Results[0].DocumentID != "private" {
		t.Fatal("allowed selection failed", result, err)
	}
	revokeDuringInference := func(ctx context.Context, text []string) ([][]float32, error) {
		if _, err := s.UpdateCollection("owner", p.ID, 1, p.Name, nil, nil); err != nil {
			t.Fatal(err)
		}
		return embed(ctx, text)
	}
	result, err = e.search(e.ForActor("reader").context(context.Background()), "harvest", opts, revokeDuringInference)
	if result != nil || !errors.Is(err, ErrCollectionUnavailable) {
		t.Fatal("revoked passage returned", result, err)
	}
	if err := s.DeleteDocument("public"); err != nil {
		t.Fatal(err)
	}
	opts.DocumentFilter = nil
	before = calls
	result, err = e.search(context.Background(), "harvest", opts, embed)
	if err != nil || len(result.Results) != 0 || calls != before {
		t.Fatal("empty allowed set became unfiltered search", result, err)
	}
}

func TestKnowledgeReindexRechecksPermissionAtPublication(t *testing.T) {
	e := NewEngine(nil, t.TempDir())
	defer e.Close()
	e.enabled = true
	s := e.store.(*SQLiteStore)
	doc := originalFixture([]byte("retained private original"))
	chunk, identity := publicationFixture(doc)
	if err := s.PublishDocument(doc, []*Chunk{chunk}, [][]float32{{1, 0}}, identity, false); err != nil {
		t.Fatal(err)
	}
	p, err := s.CreateCollection("owner", "Private", []string{"reader"}, []string{"editor"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.AssignDocumentCollection("owner", doc.ID, p.ID, 1); err != nil {
		t.Fatal(err)
	}
	// No embedding engine exists: denied work must stop before loading/generation.
	if _, err := e.ForActor("reader").ReindexDocument(context.Background(), doc.ID); !errors.Is(err, ErrCollectionUnavailable) {
		t.Fatal("reader reached reindex", err)
	}
	if _, err := e.ReindexDocument(context.Background(), doc.ID); !errors.Is(err, ErrCollectionUnavailable) {
		t.Fatal("legacy caller reached private reindex", err)
	}
	if _, err := s.UpdateCollection("owner", p.ID, 1, p.Name, nil, nil); err != nil {
		t.Fatal(err)
	}
	replacement := *doc
	replacement.RawContent = "Must not publish"
	if err := s.replaceDocumentWithChunks(&replacement, nil, nil, &identity, knowledgePrincipal{actor: "editor"}); !errors.Is(err, ErrCollectionUnavailable) {
		t.Fatal("revoked editor published", err)
	}
	retained := e.ForActor("owner").GetDocument(doc.ID)
	if retained == nil || retained.RawContent != doc.RawContent {
		t.Fatal("failed reindex changed source")
	}
}

func TestLegacyIngestionDoesNotDisclosePrivateDuplicate(t *testing.T) {
	e := NewEngine(nil, t.TempDir())
	defer e.Close()
	e.enabled = true
	s := e.store.(*SQLiteStore)
	content := "Private matching source bytes"
	doc := originalFixture([]byte(content))
	doc.RawContent = content
	doc.Name = "confidential-title.pdf"
	if err := s.AddDocumentWithChunks(doc, nil, nil); err != nil {
		t.Fatal(err)
	}
	p, err := s.CreateCollection("owner", "Private", nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.AssignDocumentCollection("owner", doc.ID, p.ID, 1); err != nil {
		t.Fatal(err)
	}
	for _, actor := range []string{"owner", "stranger"} {
		// Metadata and identity do not change the shared compatibility import's
		// destination or authorize linking an existing private publication.
		ctx := e.ForActor(actor).context(context.Background())
		if result, err := e.IngestText(ctx, "copy", content, map[string]string{"collection_id": p.ID}); result != nil || !errors.Is(err, ErrCollectionUnavailable) {
			t.Fatal("private duplicate disclosed or shared", result, err)
		}
	}
}
