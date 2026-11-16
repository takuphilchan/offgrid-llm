package sessions

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/takuphilchan/offgrid-llm/internal/document"
)

func linkedAttachmentFixture(t *testing.T, longNote bool) AttachmentText {
	t.Helper()
	header, clause, note := "Annual cap (USD)\n", "Zirconium clinics: 17.25\n", "Exception: island clinics receive 19.50.\n"
	if longNote {
		note += strings.Repeat("Required qualifying condition. ", 400)
	}
	text := header + strings.Repeat("Ordinary archived material.\n", 300) + clause + strings.Repeat("Unrelated correspondence.\n", 300) + note
	clauseStart, noteStart := strings.Index(text, clause), strings.Index(text, note)
	graph := &document.Extraction{SourceKind: "legacy_text", Method: "legacy", Text: text, Units: []document.Unit{{ID: "section", Kind: "section", Index: 1, Span: document.Span{End: len(text)}}},
		Blocks:        []document.Block{{ID: "header", UnitID: "section", Kind: "header", Method: "native", Span: document.Span{End: len(header)}}, {ID: "clause", UnitID: "section", Kind: "paragraph", Method: "native", Span: document.Span{Start: clauseStart, End: clauseStart + len(clause)}}, {ID: "note", UnitID: "section", Kind: "footnote", Method: "native", Span: document.Span{Start: noteStart, End: len(text)}}},
		Relationships: []document.Relationship{{From: "header", To: "clause", Kind: "header_for"}, {From: "note", To: "clause", Kind: "footnote_for"}}}
	if err := graph.Seal(); err != nil {
		t.Fatal(err)
	}
	return AttachmentText{Attachment: Attachment{ID: "file-source", Revision: "source-revision", ExtractionRevision: graph.ID, Name: "Policy.docx"}, Text: text, Structure: graph}
}

func TestAttachmentSelectionCarriesRemoteHeaderAndException(t *testing.T) {
	source := linkedAttachmentFixture(t, false)
	selected, err := SelectAttachmentText([]AttachmentText{source}, "Zirconium clinics", 4000)
	if err != nil {
		t.Fatal(err)
	}
	encoded, _ := json.Marshal(selected)
	if len(encoded) > 4000 || !strings.Contains(string(encoded), "17.25") || !strings.Contains(string(encoded), "Annual cap (USD)") || !strings.Contains(string(encoded), "19.50") {
		t.Fatal("lost linked evidence", string(encoded))
	}
	r := selected[0].LinkedEvidence
	if r == nil || !r.LinksComplete || r.Revision != source.Structure.ID {
		t.Fatal("incorrect link receipt", r)
	}
	for _, excerpt := range selected[0].Excerpts {
		if excerpt.Text != source.Text[excerpt.StartByte:excerpt.EndByte] {
			t.Fatal("invented excerpt span")
		}
	}
	saved := AttachmentSelections(selected)
	data, _ := json.Marshal(saved)
	var restored []AttachmentSelection
	if json.Unmarshal(data, &restored) != nil || restored[0].LinkedEvidence == nil || !restored[0].LinkedEvidence.LinksComplete {
		t.Fatal("saved answer lost receipt")
	}
	if source.LinkedEvidence != nil || source.Text != source.Structure.Text {
		t.Fatal("selection mutated retained source")
	}
}

func TestAttachmentSelectionDisclosesUnfittableException(t *testing.T) {
	source := linkedAttachmentFixture(t, true)
	selected, err := SelectAttachmentText([]AttachmentText{source}, "Zirconium clinics", 3000)
	if err != nil {
		t.Fatal(err)
	}
	r := selected[0].LinkedEvidence
	if r == nil || r.LinksComplete || len(r.OmittedBlocks) == 0 || !strings.Contains(strings.Join(r.OmittedBlocks, ","), "note") {
		t.Fatal("missing exception represented as complete", r)
	}
	data, _ := json.Marshal(selected)
	if len(data) > 3000 || !strings.Contains(string(data), "17.25") {
		t.Fatal("lost usable partial evidence", string(data))
	}
	if !strings.Contains(AttachmentPrompt("question", selected), "do not interpret a value or rule that depends on missing context") {
		t.Fatal("missing model restriction")
	}
}

func TestAttachmentSpanCoverageAccountsForGaps(t *testing.T) {
	span := document.Span{Start: 4, End: 12}
	if !attachmentSpanCovered(span, []AttachmentExcerpt{{StartByte: 8, EndByte: 12}, {StartByte: 2, EndByte: 8}}) {
		t.Fatal("adjacent exact excerpts not covered")
	}
	if attachmentSpanCovered(span, []AttachmentExcerpt{{StartByte: 2, EndByte: 7}, {StartByte: 8, EndByte: 12}}) {
		t.Fatal("unread gap marked covered")
	}
}
