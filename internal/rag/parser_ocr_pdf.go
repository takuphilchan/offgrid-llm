package rag

import (
	"context"
	"fmt"
	"strings"
	"time"
)

const documentOCRBudget = 2 * time.Minute

// A fixed-size fingerprint binds every component without exceeding existing
// attachment parser-identity bounds. Component provenance remains in metadata.
var ocrExtractionRevision = "offgrid-ocr-v1-" + ocrDigest([]byte(pdfRasterRevision+"\x00"+ocrWorkerRevision+"\x00merge1\x00eng\x00"+ocrEnglishSHA))

// The OCR worker is integrated at the existing typed extraction boundary. This
// does not enable OCR automatically: an explicit verified language is required.
// Native/scanned pages use source coordinates to reconcile overlapping words.
// Ambiguous geometry remains partial instead of losing or duplicating content.
func (p *DocumentParser) extractScannedPDFPages(ctx context.Context, source []byte, result *ParseResult) (*ParseResult, error) {
	ctx, cancel := context.WithTimeout(ctx, documentOCRBudget)
	defer cancel()
	completed := map[int]bool{}
	for i := range result.SourceUnits {
		unit := &result.SourceUnits[i]
		if unit.State != "ocr_required" {
			continue
		}
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		raster, err := p.rasterPDFPage(ctx, source, unit.Index)
		if err != nil {
			return nil, err
		}
		page, err := p.recognizePDFRaster(ctx, raster)
		if err != nil {
			return nil, err
		}
		text := page.Text
		if strings.TrimSpace(unit.Text) != "" {
			text, err = mergeNativeOCR(unit.Text, raster, page)
			if err != nil {
				result.Warnings = append(result.Warnings, ExtractionWarning{Code: "document_ocr_overlap", SourceIndex: unit.Index, Message: "Native and scanned text could not be safely combined. Review this page in the original document."})
				continue
			}
		}
		unit.Text = text
		unit.ExtractorRevision = ocrExtractionRevision
		minimumConfidence := 100
		for _, word := range page.Words {
			minimumConfidence = min(minimumConfidence, word.Confidence)
		}
		if len(page.Words) == 0 {
			minimumConfidence = 0
		}
		result.Metadata[fmt.Sprintf("ocr_page_%d", unit.Index)] = fmt.Sprintf("eng; rotation=%d; attempts=%d; minimum_confidence=%d", page.Rotation, page.Attempts, minimumConfidence)
		if page.State != "complete" || strings.TrimSpace(page.Text) == "" {
			unit.State = "ocr_partial"
			result.Warnings = append(result.Warnings, ExtractionWarning{Code: "document_ocr_low_confidence", SourceIndex: unit.Index, Message: "Some text on this scanned page could not be read confidently. Review the original; the document is not fully extracted."})
			continue
		}
		unit.State = "ready"
		completed[unit.Index] = true
	}
	var text strings.Builder
	partial := false
	for _, unit := range result.SourceUnits {
		if int64(text.Len()+len(unit.Text)+1) > p.maxExtractedSize {
			return nil, &ParseError{Code: "document_limit", Message: "Extracted document text exceeds the supported limit."}
		}
		if text.Len() > 0 {
			text.WriteByte('\n')
		}
		text.WriteString(unit.Text)
		partial = partial || unit.State != "ready" && unit.State != "empty"
	}
	warnings := result.Warnings[:0]
	for _, warning := range result.Warnings {
		if warning.Code != "document_ocr_required" || !completed[warning.SourceIndex] {
			warnings = append(warnings, warning)
		}
	}
	result.Warnings = warnings
	result.Content = strings.TrimSpace(text.String())
	result.WordCount = countWords(result.Content)
	result.Metadata["ocr_revision"], result.Metadata["ocr_language_sha256"] = ocrWorkerRevision, ocrEnglishSHA
	result.Metadata["extractor_revision"] = ocrExtractionRevision
	if partial {
		result.State = ExtractionPartial
		result.Metadata["extraction_state"] = "partial"
		return result, &ParseError{Code: "document_ocr_required", Message: "Some pages need further OCR or mixed-content verification. The document is not fully extracted."}
	}
	result.State = ExtractionComplete
	result.Metadata["extraction_state"] = "complete"
	return result, nil
}
