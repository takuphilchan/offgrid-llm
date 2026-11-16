package rag

import (
	"bytes"
	"context"
	"errors"
	"image"
	"image/color"
	"image/png"
	"os"
	"testing"
)

func TestDocumentThumbnailBoundsAndSourcePreservation(t *testing.T) {
	for _, size := range []image.Point{{1200, 1800}, {1800, 1200}, {60, 40}} {
		source := image.NewRGBA(image.Rect(0, 0, size.X, size.Y))
		for y := 0; y < size.Y; y++ {
			for x := 0; x < size.X; x++ {
				source.SetRGBA(x, y, color.RGBA{uint8(x), uint8(y), 127, 255})
			}
		}
		var encoded bytes.Buffer
		if err := png.Encode(&encoded, source); err != nil {
			t.Fatal(err)
		}
		original := bytes.Clone(encoded.Bytes())
		parser := NewDocumentParser()
		parser.ocrWorker = "must-not-launch"
		thumb, err := parser.RenderPreviewThumbnail(context.Background(), original, ".png", 1)
		if err != nil {
			t.Fatal(err)
		}
		config, err := png.DecodeConfig(bytes.NewReader(thumb))
		if err != nil || config.Width > 192 || config.Height > 256 || config.Width < 1 || config.Height < 1 || len(thumb) > 256<<10 {
			t.Fatal(config, len(thumb), err)
		}
		if !bytes.Equal(original, encoded.Bytes()) {
			t.Fatal("original modified")
		}
		if _, err := parser.RenderPreviewThumbnail(context.Background(), original, ".png", 2); err == nil {
			t.Fatal("wrong page accepted")
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := boundedThumbnail(ctx, nil); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	if _, err := boundedThumbnail(context.Background(), []byte("untrusted")); err == nil {
		t.Fatal("invalid raster accepted")
	}
}

func TestDocumentThumbnailRealPackagedPDF(t *testing.T) {
	worker := os.Getenv("OFFGRID_TEST_DOCUMENT_WORKER")
	if worker == "" {
		t.Skip("requires explicit packaged PDF worker")
	}
	parser := NewDocumentParser()
	parser.pdfWorker = worker
	parser.pdfCacheDir = t.TempDir()
	parser.ocrWorker = "must-not-launch"
	for _, name := range []string{"compressed.pdf", "scan.pdf"} {
		original, err := os.ReadFile("testdata/documents/corpus/" + name)
		if err != nil {
			t.Fatal(err)
		}
		raw, err := parser.RenderPreviewThumbnail(context.Background(), original, ".pdf", 1)
		if err != nil {
			t.Fatal(name, err)
		}
		cfg, err := png.DecodeConfig(bytes.NewReader(raw))
		if err != nil || cfg.Width > 192 || cfg.Height > 256 || cfg.Width < 1 || cfg.Height < 1 {
			t.Fatal(name, cfg, err)
		}
		t.Logf("%s: bounded %dx%d original-page thumbnail without OCR", name, cfg.Width, cfg.Height)
	}
}
