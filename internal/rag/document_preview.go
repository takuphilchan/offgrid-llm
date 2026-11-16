package rag

import (
	"context"
	"github.com/takuphilchan/offgrid-llm/internal/documentimage"
	"strings"
)

// NativeTextUsable is deliberately narrower than "some text exists". A scan
// anywhere in the document keeps automatic text admission closed.
func NativeTextUsable(result *ParseResult) bool {
	if result == nil || result.State != ExtractionPartial || len(result.SourceUnits) == 0 || strings.TrimSpace(result.Content) == "" {
		return false
	}
	visual := false
	for _, unit := range result.SourceUnits {
		switch unit.State {
		case "ready", "empty":
		case "native_text_with_visuals":
			visual = true
		default:
			return false
		}
	}
	return visual
}

// PreviewContext extracts native source units without starting OCR or changing
// import state. Partial results remain partial, and cannot authorize inference.
func (p *DocumentParser) PreviewContext(ctx context.Context, data []byte, name, ext string) (*ParseResult, error) {
	preview := *p
	preview.previewOnly = true
	preview.ocrWorker = ""
	preview.ocrLanguage = nil
	return preview.ParseContext(ctx, data, name, ext)
}

// RenderImagePreview returns the same verified upright raster used by OCR, with
// no OCR/model work. Source authorization belongs to the caller, as for PDFs.
func (p *DocumentParser) RenderImagePreview(ctx context.Context, data []byte, ext string) ([]byte, error) {
	return p.RenderImagePage(ctx, data, ext, 1)
}

func (p *DocumentParser) RenderImagePage(ctx context.Context, data []byte, ext string, index int) ([]byte, error) {
	release, err := sharedExtractionScheduler.acquire(ctx, extractionQueueTimeout)
	if err != nil {
		return nil, err
	}
	defer release()
	return p.renderImagePage(ctx, data, ext, index)
}

func (p *DocumentParser) renderImagePage(ctx context.Context, data []byte, ext string, index int) ([]byte, error) {
	if ext == ".tif" || ext == ".tiff" {
		pages, err := documentimage.Inspect(ctx, data)
		if err != nil {
			return nil, tiffError(err)
		}
		if index < 1 || index > len(pages) {
			return nil, tiffError(documentimage.ErrInvalid)
		}
		page, err := tiffRaster(ctx, data, pages[index-1], index)
		if err != nil {
			return nil, err
		}
		return page.raster.PNG, nil
	}
	if index != 1 {
		return nil, tiffError(documentimage.ErrInvalid)
	}
	page, err := decodeImageDocument(ctx, data, ext)
	if err != nil {
		return nil, err
	}
	return page.raster.PNG, nil
}

// RenderPreviewPage shares ingestion admission and the pinned raster protocol.
// Callers must check source access before and after this cancellable operation.
func (p *DocumentParser) RenderPreviewPage(ctx context.Context, data []byte, page int) ([]byte, error) {
	release, err := sharedExtractionScheduler.acquire(ctx, extractionQueueTimeout)
	if err != nil {
		return nil, err
	}
	defer release()
	raster, err := p.renderPDFPageMode(ctx, data, page, true)
	if err != nil {
		return nil, err
	}
	return raster.PNG, nil
}
