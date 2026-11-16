package server

import (
	"github.com/takuphilchan/offgrid-llm/internal/document"
	"github.com/takuphilchan/offgrid-llm/internal/rag"
	"reflect"
	"unicode/utf8"
)

type retainedImageText struct {
	digest, text string
	coverage     *document.Coverage
	revision     string
	graph        *document.Extraction
	ranges       []rag.SourceRange
}

// Combine source-validated pixels with the saved OCR revision. Opening a reader
// never reruns OCR, changes language packs, or substitutes a fresh transcription.
func applyRetainedImageText(result *rag.ParseResult, sha string, retained []retainedImageText) bool {
	if result == nil || result.Metadata["format"] != "image_document" || len(result.SourceUnits) == 0 || len(retained) != 1 {
		return false
	}
	v := retained[0]
	if v.digest != sha || v.coverage == nil || v.coverage.Format != "image_document" || v.text == "" {
		return false
	}
	if v.graph != nil || len(result.SourceUnits) > 1 || len(v.ranges) > 0 {
		units := append([]rag.SourceUnit(nil), result.SourceUnits...)
		if v.graph != nil {
			g := v.graph
			if g.Verify() != nil || g.OriginalSHA256 != sha || g.Text != v.text || g.Format != "image_document" || len(g.Units) != len(units) || !reflect.DeepEqual(g.Coverage, v.coverage) {
				return false
			}
			for i, u := range g.Units {
				if u.Kind != "page" || u.Index != i+1 {
					return false
				}
				units[i].Text = g.Text[u.Span.Start:u.Span.End]
				units[i].State = u.State
				units[i].ExtractorRevision = u.ParserRevision
			}
		} else {
			if len(v.ranges) == 0 || v.coverage.Status == "partial" || v.coverage.TotalUnits != len(units) {
				return false
			}
			for i := range units {
				units[i].Text = ""
				units[i].State = "empty"
				units[i].ExtractorRevision = v.revision
			}
			end, last := 0, 0
			for _, span := range v.ranges {
				if span.Kind != "page" || span.Index <= last || span.Index > len(units) || span.SHA256 != sha || span.Revision != v.revision || span.Start < end || span.End <= span.Start || span.End > len(v.text) || !utf8.ValidString(v.text[span.Start:span.End]) {
					return false
				}
				units[span.Index-1].Text = v.text[span.Start:span.End]
				units[span.Index-1].State = "ready"
				end, last = span.End, span.Index
			}
		}
		result.SourceUnits = units
		result.Content = v.text
		result.Warnings = v.coverage.Warnings
		result.Structure = v.graph
		result.State = rag.ExtractionComplete
		if v.coverage.Status == "partial" {
			result.State = rag.ExtractionPartial
		}
		return true
	}
	result.Content, result.SourceUnits[0].Text = v.text, v.text
	// This read uses a retained extraction, not the currently installed parser.
	// Failed preparation may have no stable revision yet and remains review-only.
	result.SourceUnits[0].ExtractorRevision = v.revision
	result.Warnings = v.coverage.Warnings
	result.State, result.SourceUnits[0].State = rag.ExtractionComplete, "ready"
	if v.coverage.Status == "partial" {
		result.State, result.SourceUnits[0].State = rag.ExtractionPartial, "ocr_partial"
	}
	return true
}
