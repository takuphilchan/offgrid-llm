package rag

import (
	"context"
	"errors"
	"strings"
	"testing"
)

func TestSelectedEvidencePinsBlockAndRejectsBroaderFallback(t *testing.T) {
	e, d, _ := correctionFixture(t)
	ctx := context.Background()
	block := d.Extraction.Blocks[1]
	read := func(actor, revision string, blocks []string, budget int) (*RAGContext, error) {
		return e.ForActor(actor).SelectEvidence(ctx, d.ID, revision, 0, blocks, budget)
	}
	got, err := read("reader", d.ExtractionRevision, []string{block.ID}, 6000)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Results) != 1 || got.Results[0].Chunk.Content != d.RawContent[block.Span.Start:block.Span.End] || strings.Contains(got.Context, "Preserved clause") || got.RetrievalMode != "selected-evidence" {
		t.Fatal("scope broadened", got)
	}
	c := got.Results[0].Chunk
	_, text, err := e.ForActor("reader").ReadSourcePassage(d.ID, c.StartChar, c.EndChar, c.Source.SHA256, c.Source.Revision, c.ContentHash)
	if err != nil || text != c.Content {
		t.Fatal("citation mismatch", err)
	}
	for _, test := range []struct {
		actor, revision string
		blocks          []string
		budget          int
	}{{"stranger", d.ExtractionRevision, []string{block.ID}, 6000}, {"reader", "missing", []string{block.ID}, 6000}, {"reader", d.ExtractionRevision, []string{"missing"}, 6000}, {"reader", d.ExtractionRevision, []string{block.ID, block.ID}, 6000}, {"reader", d.ExtractionRevision, []string{block.ID}, 1}} {
		if _, err := read(test.actor, test.revision, test.blocks, test.budget); err == nil {
			t.Fatal("invalid scope accepted", test)
		}
	}
	if _, err = e.ForActor("reader").SelectEvidence(ctx, d.ID, d.ExtractionRevision, 8, nil, 6000); err == nil {
		t.Fatal("missing page broadened")
	}
}

func TestSelectedEvidenceKeepsHistoricalRevisionAndHonorsRevocation(t *testing.T) {
	e, d, policy := correctionFixture(t)
	ctx := context.Background()
	oldRevision, block := d.ExtractionRevision, d.Extraction.Blocks[1]
	oldText := d.RawContent[block.Span.Start:block.Span.End]
	updated, err := e.correctDocument(e.ForActor("editor").context(ctx), d.ID, oldRevision, correctionFor(d), func(_ context.Context, texts []string) ([][]float32, error) {
		vectors := make([][]float32, len(texts))
		for i := range vectors {
			vectors[i] = []float32{0, 1}
		}
		return vectors, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	old, err := e.ForActor("reader").SelectEvidence(ctx, d.ID, oldRevision, 0, []string{block.ID}, 6000)
	if err != nil || old.Results[0].Chunk.Content != oldText || strings.Contains(old.Context, "18.250") {
		t.Fatal("historical selection substituted", err)
	}
	current, err := e.ForActor("reader").SelectEvidence(ctx, d.ID, updated.ExtractionRevision, 0, []string{block.ID}, 6000)
	if err != nil || !strings.Contains(current.Context, "18.250") {
		t.Fatal("current selection missing correction", err)
	}
	chunk := old.Results[0].Chunk
	_, text, err := e.ForActor("reader").ReadSourcePassage(d.ID, chunk.StartChar, chunk.EndChar, chunk.Source.SHA256, chunk.Source.Revision, chunk.ContentHash)
	if err != nil || text != oldText {
		t.Fatal("historical citation changed", err)
	}
	if _, err = e.store.(*SQLiteStore).UpdateCollection("owner", policy.ID, policy.Revision, policy.Name, nil, policy.Editors); err != nil {
		t.Fatal(err)
	}
	if _, err = e.ForActor("reader").SelectEvidence(ctx, d.ID, oldRevision, 0, []string{block.ID}, 6000); !errors.Is(err, ErrCollectionUnavailable) {
		t.Fatal("revoked reader retrieved historical scope", err)
	}
	if _, _, err = e.ForActor("reader").ReadSourcePassage(d.ID, chunk.StartChar, chunk.EndChar, chunk.Source.SHA256, chunk.Source.Revision, chunk.ContentHash); err == nil {
		t.Fatal("revoked reader opened citation")
	}
}

func TestSelectedEvidenceRetainsLinkedHeadersAndExceptions(t *testing.T) {
	e, d := linkedFixture(t)
	for _, pair := range [][2]string{{"17.25", "Annual cap (USD)"}, {"Applications close June 30.", "Exception: rural applications close July 31."}} {
		var selected string
		for _, block := range d.Extraction.Blocks {
			if strings.Contains(d.RawContent[block.Span.Start:block.Span.End], pair[0]) && block.Kind != "table" {
				selected = block.ID
				break
			}
		}
		if selected == "" {
			t.Fatal("missing fixture block", pair[0])
		}
		got, err := e.ForActor("local").SelectEvidence(context.Background(), d.ID, d.ExtractionRevision, 0, []string{selected}, 16000)
		if err != nil || !strings.Contains(got.Context, pair[0]) || !strings.Contains(got.Context, pair[1]) {
			t.Fatal("linked context dropped", pair, err)
		}
		for _, result := range got.Results {
			if result.EvidenceReceipt == nil || !result.EvidenceReceipt.LinksComplete {
				t.Fatal("missing coverage receipt")
			}
			c := result.Chunk
			_, text, err := e.ForActor("local").ReadSourcePassage(d.ID, c.StartChar, c.EndChar, c.Source.SHA256, c.Source.Revision, c.ContentHash)
			if err != nil || text != c.Content {
				t.Fatal("linked citation mismatch", err)
			}
		}
	}
}
