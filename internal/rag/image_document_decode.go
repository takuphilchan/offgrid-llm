package rag

import (
	"bytes"
	"context"
	"image"
	_ "image/jpeg"
	"image/png"
	"io"

	"github.com/takuphilchan/offgrid-llm/internal/documentocr"
	"github.com/takuphilchan/offgrid-llm/internal/imageinput"
)

const imageDocumentMaxBytes = 24 << 20

type imageDocumentPage struct {
	raster                     *pdfPageRaster
	width, height, orientation int // Original encoded pixel grid, before EXIF.
	format                     string
}

type imageContextReader struct {
	context.Context
	*bytes.Reader
}

func (r imageContextReader) Read(p []byte) (int, error) {
	if err := r.Err(); err != nil {
		return 0, err
	}
	return r.Reader.Read(p)
}

type imagePNGWriter struct {
	context.Context
	bytes.Buffer
}

func (w *imagePNGWriter) Write(p []byte) (int, error) {
	if err := w.Err(); err != nil {
		return 0, err
	}
	if len(p) > imageDocumentMaxBytes-w.Len() {
		return 0, &ParseError{Code: "document_limit", Message: "The decoded image exceeds the local OCR raster limit."}
	}
	return w.Buffer.Write(p)
}

// DecodeConfig bounds allocation, full Decode verifies pixels, and the shared
// metadata reader rejects ambiguous/animated input. Only a lossless upright
// derivative goes to OCR; the original bytes are never modified or recompressed.
func decodeImageDocument(ctx context.Context, source []byte, ext string) (*imageDocumentPage, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if len(source) > imageDocumentMaxBytes {
		return nil, &ParseError{Code: "document_limit", Message: "Image documents must be 24 MiB or smaller."}
	}
	reader := func() io.Reader { return imageContextReader{ctx, bytes.NewReader(source)} }
	config, format, err := image.DecodeConfig(reader())
	want := "jpeg"
	if ext == ".png" {
		want = "png"
	}
	if err != nil || format != want {
		return nil, &ParseError{Code: "document_invalid", Message: "The image data does not match a valid PNG or JPEG document."}
	}
	if config.Width < 1 || config.Height < 1 || config.Width > 8192 || config.Height > 8192 || int64(config.Width)*int64(config.Height) > 16000000 {
		return nil, &ParseError{Code: "document_limit", Message: "Image documents must be at most 8192 pixels per side and 16 million pixels."}
	}
	orientation, err := imageinput.Orientation(source, format)
	if err != nil {
		return nil, &ParseError{Code: "document_invalid", Message: "The image has ambiguous metadata or multiple frames. Export a single still image before importing."}
	}
	pixels, decodedFormat, err := image.Decode(reader())
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}
	if err != nil || decodedFormat != format || pixels.Bounds().Dx() != config.Width || pixels.Bounds().Dy() != config.Height {
		return nil, &ParseError{Code: "document_invalid", Message: "The complete image pixels could not be decoded."}
	}
	return imageDocumentRaster(ctx, pixels, config.Width, config.Height, orientation, format)
}

func imageDocumentRaster(ctx context.Context, pixels image.Image, width, height, orientation int, format string) (*imageDocumentPage, error) {
	pixels, err := imageinput.OrientContext(ctx, pixels, orientation)
	if err != nil {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		return nil, &ParseError{Code: "document_invalid", Message: "The image orientation could not be verified."}
	}
	// The viewer and OCR must see the same pixels, including transparent PNGs.
	// Use the worker's explicit white compositing and 8-bit conversion for this
	// derivative only; the original remains available unchanged for download.
	rgba, w, h, err := documentocr.Pixels(ctx, pixels, 0)
	if err != nil {
		return nil, err
	}
	pixels = &image.RGBA{Pix: rgba, Stride: w * 4, Rect: image.Rect(0, 0, w, h)}
	encoded := &imagePNGWriter{Context: ctx}
	if err = png.Encode(encoded, pixels); err != nil {
		return nil, err
	}
	return &imageDocumentPage{raster: &pdfPageRaster{Page: 1, Width: pixels.Bounds().Dx(), Height: pixels.Bounds().Dy(), PNG: encoded.Bytes()}, width: width, height: height, orientation: orientation, format: format}, nil
}
