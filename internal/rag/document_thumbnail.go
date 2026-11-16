package rag

import (
	"bytes"
	"context"
	"image"
	"image/png"

	"golang.org/x/image/draw"
)

// RenderPreviewThumbnail shares the single extraction admission slot. It never
// runs OCR and never changes originals, extraction revisions or review state.
func (p *DocumentParser) RenderPreviewThumbnail(ctx context.Context, data []byte, ext string, page int) ([]byte, error) {
	release, err := sharedExtractionScheduler.acquire(ctx, extractionQueueTimeout)
	if err != nil {
		return nil, err
	}
	defer release()
	var pixels []byte
	if ext == ".pdf" {
		raster, err := p.renderPDFPageMode(ctx, data, page, true)
		if err != nil {
			return nil, err
		}
		pixels = raster.PNG
	} else {
		pixels, err = p.renderImagePage(ctx, data, ext, page)
		if err != nil {
			return nil, err
		}
	}
	return boundedThumbnail(ctx, pixels)
}

func boundedThumbnail(ctx context.Context, pixels []byte) ([]byte, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	cfg, err := png.DecodeConfig(bytes.NewReader(pixels))
	if err != nil || cfg.Width < 1 || cfg.Height < 1 || cfg.Width > 8192 || cfg.Height > 8192 || int64(cfg.Width)*int64(cfg.Height) > 16_000_000 {
		return nil, &ParseError{Code: "document_invalid", Message: "The source page could not be prepared for preview."}
	}
	source, err := png.Decode(imageContextReader{ctx, bytes.NewReader(pixels)})
	if err != nil {
		return nil, err
	}
	w, h := cfg.Width, cfg.Height
	if w > 192 {
		h = max(1, h*192/w)
		w = 192
	}
	if h > 256 {
		w = max(1, w*256/h)
		h = 256
	}
	thumb := image.NewRGBA(image.Rect(0, 0, w, h))
	draw.CatmullRom.Scale(thumb, thumb.Bounds(), source, source.Bounds(), draw.Src, nil)
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	var output bytes.Buffer
	if err := png.Encode(&output, thumb); err != nil {
		return nil, err
	}
	if output.Len() > 256<<10 {
		return nil, &ParseError{Code: "document_limit", Message: "The page preview exceeds the supported limit."}
	}
	return output.Bytes(), nil
}
