package rag

import (
	"bytes"
	"context"
	"image"

	"github.com/takuphilchan/offgrid-llm/internal/documentimage"
	"github.com/takuphilchan/offgrid-llm/internal/imageinput"
)

// ImageGeometry describes the encoded pixel grid. Preview pixels are upright;
// source bounds must undergo this exact orientation before being displayed.
type ImageGeometry struct {
	Width       int `json:"width"`
	Height      int `json:"height"`
	Orientation int `json:"orientation"`
}

// PreviewImageGeometry inspects an already validated original without decoding
// more pixels or running OCR. Unknown frames produce no overlay capability.
func PreviewImageGeometry(ctx context.Context, data []byte, ext string) []ImageGeometry {
	if ctx.Err() != nil || len(data) > imageDocumentMaxBytes {
		return nil
	}
	if ext == ".tif" || ext == ".tiff" {
		pages, err := documentimage.Inspect(ctx, data)
		if err != nil {
			return nil
		}
		out := make([]ImageGeometry, len(pages))
		for i, p := range pages {
			out[i] = ImageGeometry{p.Width, p.Height, p.Orientation}
		}
		return out
	}
	if ext != ".png" && ext != ".jpg" && ext != ".jpeg" {
		return nil
	}
	c, format, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil || c.Width < 1 || c.Height < 1 || c.Width > 8192 || c.Height > 8192 || int64(c.Width)*int64(c.Height) > 16000000 {
		return nil
	}
	o, err := imageinput.Orientation(data, format)
	if err != nil {
		return nil
	}
	return []ImageGeometry{{c.Width, c.Height, o}}
}
