package rag

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/takuphilchan/offgrid-llm/internal/document"
	"github.com/takuphilchan/offgrid-llm/internal/documentimage"
)

var tiffDocumentRevision = "offgrid-tiff-ocr-v1-" + ocrDigest([]byte(documentimage.TIFFRevision + ocrWorkerRevision + ocrEnglishSHA))[:24]

func tiffError(err error) error {
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return err
	}
	if errors.Is(err, documentimage.ErrLimit) {
		return &ParseError{Code: "document_limit", Message: "TIFF documents are limited to 24 MiB, 100 pages, 16 million pixels per page and 200 million pixels total."}
	}
	if errors.Is(err, documentimage.ErrUnsupported) {
		return &ParseError{Code: "document_unsupported_format", Message: "This TIFF encoding or subimage structure is not supported. Export ordinary page images or a PDF."}
	}
	return &ParseError{Code: "document_invalid", Message: "The TIFF page data could not be verified."}
}

func tiffRaster(ctx context.Context, source []byte, page documentimage.Page, index int) (*imageDocumentPage, error) {
	pixels, err := documentimage.Decode(ctx, source, page)
	if err != nil {
		return nil, tiffError(err)
	}
	raster, err := imageDocumentRaster(ctx, pixels, page.Width, page.Height, page.Orientation, "tiff")
	if err == nil {
		raster.raster.Page = index
	}
	return raster, err
}

func (p *DocumentParser) parseTIFF(ctx context.Context, source []byte, recognize bool, run func(context.Context, *pdfPageRaster) (*ocrPageResult, error)) (*ParseResult, error) {
	pages, err := documentimage.Inspect(ctx, source)
	if err != nil {
		return nil, tiffError(err)
	}
	digest := ocrDigest(source)
	revision := p.ocrRevision(tiffDocumentRevision)
	r := &ParseResult{ContentType: "image/tiff", PageCount: len(pages), State: ExtractionPartial, Metadata: map[string]string{"format": "image_document", "image_format": "tiff", "source_sha256": digest, "extractor_revision": tiffDocumentRevision, "extraction_state": "partial", "coordinate_frame": "original_encoded_pixels", "raster_conversion": "upright; alpha=white; channels=8bit"}}
	defer r.bindCoverage()
	r.Metadata["extractor_revision"] = revision
	for i := range pages {
		r.SourceUnits = append(r.SourceUnits, SourceUnit{Kind: "page", Index: i + 1, State: "ocr_required", SourceSHA256: digest, ExtractorRevision: revision})
	}
	graph := &document.Extraction{SourceKind: "original_file", OriginalSHA256: digest, Format: "image_document", Method: "ocr", Parsers: []document.RuntimeIdentity{{Name: "offgrid-tiff-document", Revision: documentimage.TIFFRevision}, {Name: "tesseract", Revision: ocrWorkerRevision}}}
	var text strings.Builder
	complete := true
	var firstProblem error
	for i, page := range pages {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		unit := &r.SourceUnits[i]
		var ocr *ocrPageResult
		var raster *imageDocumentPage
		if recognize && p.ocrSetupErr == nil {
			raster, err = tiffRaster(ctx, source, page, i+1)
			if err == nil {
				ocr, err = run(ctx, raster.raster)
			}
			if ctx.Err() != nil {
				return nil, ctx.Err()
			}
			if err != nil && firstProblem == nil {
				firstProblem = err
			}
			if err == nil && ocr == nil {
				return nil, &ParseError{Code: "document_parser_protocol", Message: "The OCR reader returned no page evidence."}
			}
		}
		var blocks []document.Block
		if ocr != nil && err == nil {
			one := &ParseResult{Content: ocr.Text, Metadata: map[string]string{"source_sha256": digest}}
			p.bindOCRProvenance(one)
			one.Metadata["extractor_revision"] = revision
			part, e := imageDocumentStructure(one, raster, ocr)
			if e != nil {
				return nil, &ParseError{Code: "document_parser_protocol", Message: "The OCR page text and coordinates do not match."}
			}
			blocks = part.Blocks
			unit.Text = ocr.Text
			unit.State = "ready"
			if ocr.State != "complete" || strings.TrimSpace(ocr.Text) == "" {
				unit.State = "ocr_partial"
			}
			r.Metadata[fmt.Sprintf("ocr_page_%d", i+1)] = fmt.Sprintf("%s; rotation=%d; attempts=%d", p.ocrPack().Language, ocr.Rotation, ocr.Attempts)
		}
		if unit.State != "ready" {
			complete = false
			r.Warnings = append(r.Warnings, ExtractionWarning{Code: "document_ocr_required", SourceIndex: i + 1, Message: "This page has unread or uncertain text. Review its original image."})
		}
		if unit.Text != "" {
			r.Warnings = append(r.Warnings, ExtractionWarning{Code: "document_visual_content", SourceIndex: i + 1, Message: "OCR transcribes printed text only. Figures, tables, handwriting, signatures and layout relationships are not interpreted or verified."})
		}
		if text.Len() > 0 && unit.Text != "" {
			text.WriteString("\n\n")
		}
		start := text.Len()
		text.WriteString(unit.Text)
		if int64(text.Len()) > p.maxExtractedSize {
			return nil, &ParseError{Code: "document_limit", Message: "Extracted image text exceeds the supported limit."}
		}
		id := fmt.Sprintf("page-%d", i+1)
		graph.Units = append(graph.Units, document.Unit{ID: id, Kind: "page", Index: i + 1, Span: document.Span{Start: start, End: text.Len()}, State: unit.State, ParserRevision: revision})
		for j, b := range blocks {
			b.ID = fmt.Sprintf("%s-line-%d", id, j+1)
			b.UnitID = id
			b.Span.Start += start
			b.Span.End += start
			graph.Blocks = append(graph.Blocks, b)
		}
	}
	r.Content = text.String()
	r.WordCount = countWords(r.Content)
	if complete {
		r.State = ExtractionComplete
		r.Metadata["extraction_state"] = "complete"
	}
	if r.Content != "" {
		p.bindOCRProvenance(r)
		graph.LanguagePacks = ocrRuntimeIdentities(r)
		graph.Text = r.Content
		graph.Coverage = r.Coverage()
		if err := graph.Seal(); err != nil {
			return nil, err
		}
		r.Structure = graph
	}
	if !complete {
		if p.ocrSetupErr != nil && recognize {
			return r, p.ocrSetupErr
		}
		if r.Content == "" && firstProblem != nil {
			return r, firstProblem
		}
		return r, &ParseError{Code: "document_ocr_required", Message: "Some TIFF pages need local OCR or review. Available text remains incomplete."}
	}
	return r, nil
}
