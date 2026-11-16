package rag

import (
	"strings"
	"testing"

	"github.com/takuphilchan/offgrid-llm/internal/document"
)

func TestDOCXStructuredClauseNoteAndTable(t *testing.T) {
	parts := map[string]string{
		"word/document.xml":  docxTestStart + `<w:p><w:pPr><w:outlineLvl w:val="0"/></w:pPr><w:r><w:t>  政策</w:t></w:r></w:p><w:p><w:r><w:t>Deadline June.</w:t><w:footnoteReference w:id="1"/></w:r></w:p><w:tbl><w:tr><w:tc><w:p><w:r><w:t>Amount</w:t></w:r></w:p></w:tc><w:tc><w:p><w:r><w:t>17.25</w:t></w:r></w:p></w:tc></w:tr></w:tbl>` + docxTestEnd,
		"word/footnotes.xml": `<w:footnotes xmlns:w="` + wordNamespace + `"><w:footnote w:id="1"><w:p><w:r><w:t>Exception July.</w:t></w:r></w:p></w:footnote></w:footnotes>`,
	}
	r, err := NewDocumentParser().Parse(parserZip(t, parts), "policy.docx", ".docx")
	if err != nil {
		t.Fatal(err)
	}
	if r.Structure == nil || r.Structure.Verify() != nil {
		t.Fatal("no verified structure")
	}
	blocks := map[string]document.Block{}
	counts := map[string]int{}
	for _, b := range r.Structure.Blocks {
		blocks[b.ID] = b
		counts[b.Kind]++
		value := r.Content[b.Span.Start:b.Span.End]
		if b.Kind == "heading" && value != "政策" {
			t.Fatal("heading offset lost", value)
		}
	}
	if counts["heading"] != 1 || counts["footnote"] != 1 || counts["table"] != 1 || counts["row"] != 1 || counts["cell"] != 2 {
		t.Fatal("lost typed structure", counts)
	}
	linked := false
	for _, rel := range r.Structure.Relationships {
		if rel.Kind == "footnote_for" {
			from, to := blocks[rel.From], blocks[rel.To]
			if !strings.Contains(r.Content[from.Span.Start:from.Span.End], "Exception July.") || !strings.Contains(r.Content[to.Span.Start:to.Span.End], "Deadline June.") {
				t.Fatal("note associated with wrong clause")
			}
			linked = true
		}
	}
	if !linked {
		t.Fatal("note lacks clause relationship")
	}
	store, err := NewSQLiteStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	text, ranges, err := parsedSourceRanges(r)
	if err != nil {
		t.Fatal(err)
	}
	doc := &Document{ID: "structured", RawContent: text, SourceRanges: ranges, Metadata: r.Metadata, Extraction: r.Structure}
	if err = store.AddDocumentWithChunks(doc, nil, nil); err != nil {
		t.Fatal(err)
	}
	saved, err := store.GetDocument(doc.ID)
	if err != nil {
		t.Fatal(err)
	}
	if err = store.ReplaceDocumentWithChunks(saved, nil, nil); err != nil {
		t.Fatal(err)
	}
	graph, err := store.extraction(doc.ID, doc.ExtractionRevision)
	if err != nil || len(graph.Relationships) != len(r.Structure.Relationships) || saved.ExtractionRevision != doc.ExtractionRevision {
		t.Fatal("reindex discarded structured relationships", err)
	}
}
