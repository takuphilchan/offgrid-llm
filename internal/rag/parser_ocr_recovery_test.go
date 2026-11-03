package rag

import (
	"context"
	"errors"
	"strings"
	"testing"
)

func TestOCRFailurePreservesVerifiedTextWithoutCompleting(t *testing.T) {
	for _, failure := range []error{context.DeadlineExceeded, &ParseError{Code: "document_timeout"}, &ParseError{Code: "document_ocr_failed"}} {
		p := NewDocumentParser()
		result := &ParseResult{Metadata: map[string]string{}, SourceUnits: []SourceUnit{
			{Index: 1, State: "ready", Text: "Native café 日本語", ExtractorRevision: pdfWorkerRevision},
			{Index: 2, State: "ocr_required"}, {Index: 3, State: "ocr_required", Text: "Unchanged native text"},
			{Index: 4, State: "ocr_required", Text: "Unattempted"},
		}}
		calls := 0
		render := func(context.Context, []byte, int) (*pdfPageRaster, error) { return &pdfPageRaster{}, nil }
		recognize := func(context.Context, *pdfPageRaster) (*ocrPageResult, error) {
			calls++
			if calls == 2 {
				return nil, failure
			}
			return &ocrPageResult{Text: "Verified scan", State: "complete"}, nil
		}
		got, err := p.extractPDFPagesWithOCR(context.Background(), nil, result, render, recognize)
		var problem *ParseError
		if got == nil || !errors.As(err, &problem) || problem.Code != "document_ocr_required" || got.State != ExtractionPartial || calls != 2 {
			t.Fatalf("operational failure lost evidence or replayed: %+v %v calls=%d", got, err, calls)
		}
		if got.Content != "Native café 日本語\nVerified scan\nUnchanged native text\nUnattempted" || got.SourceUnits[1].State != "ready" || got.SourceUnits[2].State != "ocr_required" || got.SourceUnits[0].ExtractorRevision != pdfWorkerRevision {
			t.Fatalf("source evidence changed: %+v", got)
		}
		if len(got.Warnings) != 1 || got.Warnings[0].SourceIndex != 3 || got.Warnings[0].Code != "document_ocr_incomplete" {
			t.Fatal(got.Warnings)
		}
	}
}

func TestOCRRecoveryStillRejectsInvalidEvidenceAndCancellation(t *testing.T) {
	for _, code := range []string{"document_parser_protocol", "document_limit", "document_ocr_language_invalid", "cancel"} {
		ctx, cancel := context.WithCancel(context.Background())
		result := &ParseResult{Metadata: map[string]string{}, SourceUnits: []SourceUnit{{Index: 1, State: "ocr_required", Text: "native"}}}
		render := func(context.Context, []byte, int) (*pdfPageRaster, error) {
			if code == "cancel" {
				cancel()
				return nil, context.Canceled
			}
			return nil, &ParseError{Code: code}
		}
		got, err := NewDocumentParser().extractPDFPagesWithOCR(ctx, nil, result, render, nil)
		cancel()
		if got != nil || err == nil || strings.Contains(err.Error(), "Some pages need") {
			t.Fatalf("invalid evidence accepted: %+v %v", got, err)
		}
	}
}
