package rag

import (
	"context"
	"encoding/binary"
	"image"
	"testing"

	"github.com/takuphilchan/offgrid-llm/internal/testsupport/tifffixture"
)

func TestImagePreviewGeometryPreservesEveryEncodedFrame(t *testing.T) {
	for orientation := 1; orientation <= 8; orientation++ {
		data := tifffixture.Encode([]image.Image{image.NewGray(image.Rect(0, 0, 100, 60))}, []uint16{uint16(orientation)}, binary.LittleEndian)
		g := PreviewImageGeometry(context.Background(), data, ".tiff")
		if len(g) != 1 || g[0] != (ImageGeometry{100, 60, orientation}) {
			t.Fatal(orientation, g)
		}
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		if PreviewImageGeometry(ctx, data, ".tiff") != nil {
			t.Fatal("ignored cancellation")
		}
	}
	for _, ext := range []string{".pdf", ".png", ".jpeg", ".tiff"} {
		if PreviewImageGeometry(context.Background(), []byte("invalid"), ext) != nil {
			t.Fatal(ext)
		}
	}
}
