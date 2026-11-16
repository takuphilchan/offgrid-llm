package rag

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"

	"github.com/takuphilchan/offgrid-llm/internal/document"
)

type ExtractionState string

const (
	ExtractionComplete ExtractionState = "complete"
	ExtractionPartial  ExtractionState = "partial"
)

type ExtractionWarning = document.Warning

func (r *ParseResult) Coverage() *document.Coverage {
	if r == nil {
		return nil
	}
	c := &document.Coverage{Format: r.Metadata["format"], Status: "complete", TotalUnits: len(r.SourceUnits), Warnings: append([]document.Warning(nil), r.Warnings...)}
	if r.State == ExtractionPartial {
		c.Status = "partial"
	} else if len(r.Warnings) > 0 {
		c.Status = "text_only"
	}
	review := map[int]bool{}
	for _, w := range r.Warnings {
		switch w.Code {
		case "document_form_review", "document_visual_content", "document_alternate_content", "document_table_header_review", "document_table_merge_review", "document_table_header_missing", "document_number_format", "document_excel_leap_day", "document_hidden_content", "document_formula_cache", "document_conditional_format", "document_margin_layout", "document_tracked_changes":
			review[w.SourceIndex] = true
		}
	}
	for _, u := range r.SourceUnits {
		if u.State == "ready" || u.State == "empty" || u.State == "native_text_with_visuals" {
			c.ReadableUnits++
		}
		if (u.State != "ready" && u.State != "empty") || review[u.Index] {
			c.ReviewUnits = append(c.ReviewUnits, u.Index)
		}
	}
	return c
}

func (r *ParseResult) bindCoverage() {
	if r == nil {
		return
	}
	if r.Metadata == nil {
		r.Metadata = map[string]string{}
	}
	data, _ := json.Marshal(r.Coverage())
	r.Metadata["extraction_coverage"] = string(data)
}

// bindSourceUnits binds actual extractor positions, not inferred page breaks, to
// original bytes. Flat formats receive one section, never invented page numbers.
func bindSourceUnits(result *ParseResult, source []byte) {
	if result == nil {
		return
	}
	if result.Metadata == nil {
		result.Metadata = make(map[string]string)
	}
	if result.Metadata["format"] == "pdf" || result.Metadata["format"] == "image_document" {
		// PDF worker receipts are independently checked against the source.
		return
	}
	result.State = ExtractionComplete
	result.Metadata["extraction_state"] = string(result.State)
	if limitation := result.Metadata["limitations"]; limitation != "" && len(result.Warnings) == 0 {
		result.Warnings = []ExtractionWarning{{Code: "document_text_only", Message: limitation}}
	}
	digest := sha256.Sum256(source)
	hash := hex.EncodeToString(digest[:])
	result.Metadata["source_sha256"] = hash
	revision := builtinDocumentRevision
	if result.Metadata["format"] == "xlsx" {
		revision = xlsxDocumentRevision
	}
	if result.Metadata["format"] == "docx" {
		revision = docxDocumentRevision
	}
	if result.Metadata["format"] == "pptx" {
		revision = pptxDocumentRevision
	}
	result.Metadata["extractor_revision"] = revision
	if len(result.SourceUnits) == 0 {
		result.SourceUnits = []SourceUnit{{Kind: "section", Index: 1, Text: result.Content, State: "ready"}}
	}
	for i := range result.SourceUnits {
		result.SourceUnits[i].SourceSHA256 = hash
		result.SourceUnits[i].ExtractorRevision = revision
	}
}
