package rag

import (
	"context"
	"fmt"
	"strings"

	"github.com/takuphilchan/offgrid-llm/internal/document"
	"github.com/takuphilchan/offgrid-llm/internal/imageinput"
)

var imageDocumentRevision = "offgrid-image-ocr-v1-" + ocrDigest([]byte(ocrWorkerRevision + ocrEnglishSHA))[:24]

func (p *DocumentParser) parseImageDocument(ctx context.Context, source []byte, ext string, recognize bool) (*ParseResult, error) {
	return p.extractImageDocument(ctx, source, ext, recognize, p.recognizePDFRaster)
}

func (p *DocumentParser) extractImageDocument(ctx context.Context, source []byte, ext string, recognize bool, run func(context.Context, *pdfPageRaster) (*ocrPageResult, error)) (*ParseResult, error) {
	page, err := decodeImageDocument(ctx, source, ext)
	if err != nil {
		return nil, err
	}
	digest := ocrDigest(source)
	revision := p.ocrRevision(imageDocumentRevision)
	result := &ParseResult{ContentType: "image/" + page.format, PageCount: 1, State: ExtractionPartial,
		Metadata: map[string]string{"format": "image_document", "image_format": page.format, "source_sha256": digest, "extractor_revision": revision, "extraction_state": "partial",
			"image_width": fmt.Sprint(page.width), "image_height": fmt.Sprint(page.height), "image_orientation": fmt.Sprint(page.orientation), "coordinate_frame": "original_encoded_pixels", "raster_conversion": "upright; alpha=white; channels=8bit"},
		SourceUnits: []SourceUnit{{Kind: "page", Index: 1, State: "ocr_required", SourceSHA256: digest, ExtractorRevision: revision}},
		Warnings:    []ExtractionWarning{{Code: "document_ocr_required", SourceIndex: 1, Message: "This image requires local text recognition. It has no native document text."}},
	}
	defer result.bindCoverage()
	if !recognize {
		return result, &ParseError{Code: "document_ocr_required", Message: "This image needs local OCR before its text can be used."}
	}
	if p.ocrSetupErr != nil {
		return result, p.ocrSetupErr
	}
	ocr, err := run(ctx, page.raster)
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}
	if err != nil {
		return result, err
	}
	if ocr == nil {
		return nil, &ParseError{Code: "document_parser_protocol", Message: "The local OCR reader returned no source evidence."}
	}
	result.Content, result.WordCount = ocr.Text, countWords(ocr.Text)
	if int64(len(ocr.Text)) > p.maxExtractedSize {
		return nil, &ParseError{Code: "document_limit", Message: "Extracted image text exceeds the supported limit."}
	}
	result.SourceUnits[0].Text = ocr.Text
	p.bindOCRProvenance(result)
	result.Metadata["ocr_page_1"] = fmt.Sprintf("%s; rotation=%d; attempts=%d", p.ocrPack().Language, ocr.Rotation, ocr.Attempts)
	result.Warnings = []ExtractionWarning{{Code: "document_visual_content", SourceIndex: 1, Message: "OCR transcribes printed text only. Figures, charts, handwriting, signatures and layout relationships are not interpreted or verified."}}
	if ocr.State != "complete" || strings.TrimSpace(ocr.Text) == "" {
		result.SourceUnits[0].State = "ocr_partial"
		result.Warnings = append(result.Warnings, ExtractionWarning{Code: "document_ocr_low_confidence", SourceIndex: 1, Message: "Some image text could not be read confidently. Review the original before using the available text."})
		return result, &ParseError{Code: "document_ocr_required", Message: "Image text recognition is incomplete. Review the original and available text."}
	}
	result.State, result.SourceUnits[0].State = ExtractionComplete, "ready"
	result.Metadata["extraction_state"] = "complete"
	result.Structure, err = imageDocumentStructure(result, page, ocr)
	if err != nil {
		return nil, err
	}
	return result, nil
}

func imageDocumentStructure(result *ParseResult, page *imageDocumentPage, ocr *ocrPageResult) (*document.Extraction, error) {
	revision := result.Metadata["extractor_revision"]
	if revision == "" {
		revision = imageDocumentRevision
	}
	e := &document.Extraction{SourceKind: "original_file", OriginalSHA256: result.Metadata["source_sha256"], Format: "image_document", Method: "ocr", Text: ocr.Text, Coverage: result.Coverage(),
		Parsers:       []document.RuntimeIdentity{{Name: "offgrid-image-document", Revision: revision}, {Name: "tesseract", Revision: ocrWorkerRevision}},
		LanguagePacks: ocrRuntimeIdentities(result),
		Units:         []document.Unit{{ID: "page-1", Kind: "page", Index: 1, Span: document.Span{End: len(ocr.Text)}, State: "ready", ParserRevision: revision}},
	}
	// Word positions are checked against the exact worker transcription. Line
	// blocks retain its reading order; geometry addresses original pixels, not
	// the EXIF/upright derivative sent to OCR. No headings or tables are guessed.
	cursor, lastLine := 0, 0
	for i, word := range ocr.Words {
		if i > 0 {
			separator := byte(' ')
			if word.Line != lastLine {
				separator = '\n'
			}
			if cursor >= len(e.Text) || e.Text[cursor] != separator {
				return nil, fmt.Errorf("OCR word/text mismatch")
			}
			cursor++
		}
		end := cursor + len(word.Text)
		if word.Text == "" || end > len(e.Text) || e.Text[cursor:end] != word.Text {
			return nil, fmt.Errorf("OCR word/text mismatch")
		}
		box, err := imageinput.OriginalBounds(word.Bounds, page.width, page.height, page.orientation)
		if err != nil {
			return nil, err
		}
		if i == 0 || word.Line != lastLine {
			e.Blocks = append(e.Blocks, document.Block{ID: fmt.Sprintf("page-1-line-%d", len(e.Blocks)+1), UnitID: "page-1", Kind: "paragraph", Method: "ocr", Span: document.Span{Start: cursor, End: end}, SourcePart: "original_encoded_pixels",
				Bounds: &document.Bounds{X: float64(box[0]), Y: float64(box[1]), Width: float64(box[2] - box[0]), Height: float64(box[3] - box[1]), Unit: "pixels"}})
		} else {
			b := &e.Blocks[len(e.Blocks)-1]
			b.Span.End = end
			x, y := min(b.Bounds.X, float64(box[0])), min(b.Bounds.Y, float64(box[1]))
			r, bottom := max(b.Bounds.X+b.Bounds.Width, float64(box[2])), max(b.Bounds.Y+b.Bounds.Height, float64(box[3]))
			b.Bounds.X, b.Bounds.Y, b.Bounds.Width, b.Bounds.Height = x, y, r-x, bottom-y
		}
		cursor, lastLine = end, word.Line
	}
	if cursor != len(e.Text) {
		return nil, fmt.Errorf("OCR word/text mismatch")
	}
	return e, e.Seal()
}
