package rag

import (
	"crypto/sha256"
	"encoding/hex"
)

type ExtractionState string

const (
	ExtractionComplete ExtractionState = "complete"
	ExtractionPartial  ExtractionState = "partial"
)

type ExtractionWarning struct {
	Code        string `json:"code"`
	Message     string `json:"message"`
	SourceIndex int    `json:"source_index,omitempty"`
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
	if result.Metadata["format"] == "pdf" {
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
	result.Metadata["extractor_revision"] = revision
	if len(result.SourceUnits) == 0 {
		result.SourceUnits = []SourceUnit{{Kind: "section", Index: 1, Text: result.Content, State: "ready"}}
	}
	for i := range result.SourceUnits {
		result.SourceUnits[i].SourceSHA256 = hash
		result.SourceUnits[i].ExtractorRevision = revision
	}
}
