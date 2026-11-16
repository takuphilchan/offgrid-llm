package document

import (
	"strings"
	"testing"
	"time"
)

func sampleExtraction() *Extraction {
	text := "政策\nException 17.25"
	return &Extraction{SourceKind: "original_file", OriginalSHA256: strings.Repeat("a", 64), Method: "native", Text: text,
		Units:         []Unit{{ID: "section-1", Kind: "section", Index: 1, Span: Span{End: len(text)}}},
		Blocks:        []Block{{ID: "clause", UnitID: "section-1", Kind: "paragraph", Method: "native", Span: Span{End: len(text)}}, {ID: "note", UnitID: "section-1", Kind: "footnote", Method: "native", Span: Span{Start: 7, End: len(text)}}},
		Relationships: []Relationship{{From: "note", To: "clause", Kind: "footnote_for"}}}
}

func TestExtractionSealsExactEvidence(t *testing.T) {
	e := sampleExtraction()
	if err := e.Seal(); err != nil {
		t.Fatal(err)
	}
	id := e.ID
	if err := e.Verify(); err != nil {
		t.Fatal(err)
	}
	e.Text = strings.ReplaceAll(e.Text, "17.25", "17.26")
	if e.Verify() == nil {
		t.Fatal("changed critical number accepted")
	}
	if err := e.Seal(); err != nil || e.ID == id {
		t.Fatal("new revision not distinct", err)
	}
}

func TestExtractionRejectsInvalidGraph(t *testing.T) {
	for name, mutate := range map[string]func(*Extraction){
		"split Unicode": func(e *Extraction) { e.Blocks[0].Span.Start = 1 },
		"unknown unit":  func(e *Extraction) { e.Blocks[0].UnitID = "other" },
		"outside unit":  func(e *Extraction) { e.Units[0].Span.End = 7 },
		"duplicate ID":  func(e *Extraction) { e.Blocks[1].ID = "clause" },
		"dangling link": func(e *Extraction) { e.Relationships[0].To = "missing" },
		"cycle": func(e *Extraction) {
			e.Blocks[1].Span = e.Blocks[0].Span
			e.Relationships = []Relationship{{From: "note", To: "clause", Kind: "parent"}, {From: "clause", To: "note", Kind: "parent"}}
		},
		"invented coordinate":     func(e *Extraction) { e.Blocks[0].Bounds = &Bounds{Width: 1, Height: 1} },
		"unattributed correction": func(e *Extraction) { e.ParentID = "old"; e.Method = "review" },
	} {
		t.Run(name, func(t *testing.T) {
			e := sampleExtraction()
			mutate(e)
			if e.Seal() == nil {
				t.Fatal("invalid graph accepted")
			}
		})
	}
}

func TestExtractionCorrectionRequiresProvenance(t *testing.T) {
	e := sampleExtraction()
	if err := e.Seal(); err != nil {
		t.Fatal(err)
	}
	old := e.ID
	e.ParentID = old
	e.Method = "review"
	e.Author = "reviewer"
	now := time.Now().UTC()
	e.CreatedAt = &now
	if err := e.Seal(); err != nil || e.ID == old {
		t.Fatal(err)
	}
}
