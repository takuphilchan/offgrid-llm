package rag

import "testing"

func TestReadableNativeTextCanStillNeedVisualReview(t *testing.T) {
	r := &ParseResult{State: ExtractionPartial, Metadata: map[string]string{"format": "pdf"}, SourceUnits: []SourceUnit{
		{Index: 1, State: "native_text_with_visuals", Text: "Native clause"},
		{Index: 2, State: "ocr_required"},
		{Index: 3, State: "empty"},
	}}
	c := r.Coverage()
	if c.Status != "partial" || c.TotalUnits != 3 || c.ReadableUnits != 2 || len(c.ReviewUnits) != 2 || c.ReviewUnits[0] != 1 || c.ReviewUnits[1] != 2 {
		t.Fatal(c)
	}
}

func TestNativeOfficeReviewWarningsRetainReadableUnits(t *testing.T) {
	r := &ParseResult{State: ExtractionComplete, Metadata: map[string]string{"format": "pptx"}, SourceUnits: []SourceUnit{{Index: 1, State: "ready", Text: "Native text"}, {Index: 2, State: "ready", Text: "Speaker notes"}, {Index: 3, State: "empty"}}, Warnings: []ExtractionWarning{{Code: "document_visual_content", SourceIndex: 1}, {Code: "document_table_header_review", SourceIndex: 1}, {Code: "document_speaker_notes", SourceIndex: 2}, {Code: "document_hidden_content", SourceIndex: 3}}}
	c := r.Coverage()
	if c.Status != "text_only" || c.ReadableUnits != 3 || len(c.ReviewUnits) != 2 || c.ReviewUnits[0] != 1 || c.ReviewUnits[1] != 3 {
		t.Fatal("lost review/usable distinction", c)
	}
}
