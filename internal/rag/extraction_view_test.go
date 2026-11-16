package rag

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/takuphilchan/offgrid-llm/internal/document"
)

func TestStructureProjectionBoundsTextMetadataAndLinks(t *testing.T) {
	e := NewEngine(nil, t.TempDir())
	defer e.Close()
	s := e.store.(*SQLiteStore)
	text := strings.Repeat("Large paragraph. ", 4000) + "\n" + strings.Repeat("Short paragraph.\n", 2100)
	r, err := NewDocumentParser().Parse([]byte(text), "text.txt", ".txt")
	if err != nil {
		t.Fatal(err)
	}
	retained, ranges, err := parsedSourceRanges(r)
	if err != nil {
		t.Fatal(err)
	}
	r.Structure.Blocks[0].Kind = "cell"
	r.Structure.Blocks[0].Cell = &document.Cell{Reference: "A1", Display: strings.Repeat("Metadata", 16000)}
	for _, b := range r.Structure.Blocks[1:] {
		r.Structure.Relationships = append(r.Structure.Relationships, document.Relationship{From: r.Structure.Blocks[0].ID, To: b.ID, Kind: "header_for"})
	}
	if err = r.Structure.Seal(); err != nil {
		t.Fatal(err)
	}
	d := &Document{ID: "large", RawContent: retained, SourceRanges: ranges, Extraction: r.Structure, Metadata: r.Metadata}
	if err = s.AddDocumentWithChunks(d, nil, nil); err != nil {
		t.Fatal(err)
	}
	p, err := e.ForActor("local").Structure(context.Background(), d.ID, "", "", 0, 1, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(p.Blocks) != 1 || len(p.Blocks[0].Text) > 32768 || !p.Blocks[0].Truncated || !p.Blocks[0].MetadataOmitted || p.Blocks[0].Cell != nil || p.Blocks[0].Editable || len(p.Relationships) != 2000 || p.NextRelationshipOffset == nil {
		t.Fatal("unbounded or misleading projection")
	}
	raw, _ := json.Marshal(p)
	if len(raw) > 500000 {
		t.Fatal("oversized response", len(raw))
	}
	next, err := e.ForActor("local").Structure(context.Background(), d.ID, p.Revision, "", 0, 1, *p.NextRelationshipOffset)
	if err != nil || len(next.Relationships) != 100 || next.NextRelationshipOffset != nil {
		t.Fatal("relationship pagination lost links", err)
	}
	if p.Blocks[0].TextSHA256 != document.TextSHA256(retained[r.Structure.Blocks[0].Span.Start:r.Structure.Blocks[0].Span.End]) {
		t.Fatal("digest describes truncated preview")
	}
}
