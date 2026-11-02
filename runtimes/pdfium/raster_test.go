package main

import (
	"bytes"
	"context"
	"fmt"
	"image/png"
	"math"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestRasterDimensionsRejectUnboundedPages(t *testing.T) {
	for _, size := range [][2]float64{{0, 1}, {-1, 2}, {math.NaN(), 10}, {math.Inf(1), 1}, {1, math.Inf(-1)}, {3000, 3000}, {100000, 1}} {
		if _, _, ok := rasterDimensions(size[0], size[1]); ok {
			t.Fatalf("unbounded dimensions accepted: %v", size)
		}
	}
	if w, h, ok := rasterDimensions(612, 792); !ok || w != 1700 || h != 2200 {
		t.Fatal(w, h, ok)
	}
	var out boundedPNG
	if _, err := out.Write(make([]byte, maxRasterPNG+1)); err == nil || out.Len() != 0 {
		t.Fatal("PNG output bound bypassed")
	}
}

func TestOCRRotationFixturesAreNotClipped(t *testing.T) {
	t.Setenv("OFFGRID_DOCUMENT_CODE_CACHE", t.TempDir())
	for _, rotation := range []int{0, 90, 180, 270} {
		t.Run(fmt.Sprint(rotation), func(t *testing.T) {
			data, err := os.ReadFile(filepath.Join("../../internal/rag/testdata/documents/ocr/inputs", fmt.Sprintf("visible-rotation-%d.pdf", rotation)))
			if err != nil {
				t.Fatal(err)
			}
			ctx, cancel := context.WithTimeout(context.Background(), 25*time.Second)
			defer cancel()
			raw, width, height, code := renderPDFPage(ctx, data, 1)
			if code != "" {
				t.Fatal(code)
			}
			pixels, err := png.Decode(bytes.NewReader(raw))
			if err != nil {
				t.Fatal(err)
			}
			dark := 0
			for y := 0; y < height; y++ {
				for x := 0; x < width; x++ {
					r, g, b, _ := pixels.At(x, y).RGBA()
					if r < 20000 && g < 20000 && b < 20000 {
						dark++
						if x < 20 || y < 20 || x >= width-20 || y >= height-20 {
							t.Fatal("qualification text touches the crop boundary")
						}
					}
				}
			}
			if dark < 500 {
				t.Fatal("qualification page lost visible text")
			}
		})
	}
}

func TestRealPDFRasterAndRotation(t *testing.T) {
	t.Setenv("OFFGRID_DOCUMENT_CODE_CACHE", t.TempDir())
	for _, rotation := range []int{0, 90, 180, 270} {
		t.Run(fmt.Sprint(rotation), func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 25*time.Second)
			defer cancel()
			data := coloredPDF(72, 144, rotation)
			raw, width, height, code := renderPDFPage(ctx, data, 1)
			if code != "" {
				t.Fatal(code)
			}
			wantW, wantH := 200, 400
			if rotation == 90 || rotation == 270 {
				wantW, wantH = wantH, wantW
			}
			if width != wantW || height != wantH {
				t.Fatal("rotation dimensions", width, height)
			}
			bitmap, err := png.Decode(bytes.NewReader(raw))
			if err != nil || bitmap.Bounds().Dx() != width || bitmap.Bounds().Dy() != height {
				t.Fatal("PNG dimensions", err)
			}
			// Red fills the unrotated lower-left quarter. Verify rotation and BGRA
			// conversion, rather than merely accepting a nonempty PNG receipt.
			x, y := width/4, 3*height/4
			switch rotation {
			case 90:
				x, y = width/4, height/4
			case 180:
				x, y = 3*width/4, height/4
			case 270:
				x, y = 3*width/4, 3*height/4
			}
			r, g, b, a := bitmap.At(x, y).RGBA()
			if r < 65000 || g > 100 || b > 100 || a != 65535 {
				t.Fatalf("wrong raster color at %d,%d: %d,%d,%d,%d", x, y, r, g, b, a)
			}
		})
	}
	for _, name := range []string{"scan.pdf", "scan-rotated.pdf", "mixed.pdf"} {
		t.Run(name, func(t *testing.T) {
			data, err := os.ReadFile(filepath.Join("../../internal/rag/testdata/documents/corpus", name))
			if err != nil {
				t.Fatal(err)
			}
			page := 1
			if name == "mixed.pdf" {
				page = 2
			}
			ctx, cancel := context.WithTimeout(context.Background(), 25*time.Second)
			defer cancel()
			raw, _, _, code := renderPDFPage(ctx, data, page)
			if code != "" {
				t.Fatal(code)
			}
			bitmap, err := png.Decode(bytes.NewReader(raw))
			if err != nil {
				t.Fatal(err)
			}
			dark := 0
			for y := 0; y < bitmap.Bounds().Dy(); y++ {
				for x := 0; x < bitmap.Bounds().Dx(); x++ {
					r, g, b, _ := bitmap.At(x, y).RGBA()
					if r < 30000 && g < 30000 && b < 30000 {
						dark++
					}
				}
			}
			if dark < 100 {
				t.Fatal("scanned content was not rendered", dark)
			}
		})
	}
}

func TestRasterRejectsUnavailablePageEncryptedAndOversize(t *testing.T) {
	t.Setenv("OFFGRID_DOCUMENT_CODE_CACHE", t.TempDir())
	for _, c := range []struct {
		data []byte
		page int
		code string
	}{
		{coloredPDF(72, 144, 0), 0, "document_invalid"},
		{coloredPDF(72, 144, 0), 2, "document_invalid"},
		{coloredPDF(100000, 100000, 0), 1, "document_limit"},
		{[]byte("not PDF"), 1, "document_invalid"},
	} {
		raw, _, _, code := renderPDFPage(context.Background(), c.data, c.page)
		if code != c.code || len(raw) != 0 {
			t.Fatal(code, c.code)
		}
	}
	data, err := os.ReadFile("../../internal/rag/testdata/documents/corpus/encrypted.pdf")
	if err != nil {
		t.Fatal(err)
	}
	if raw, _, _, code := renderPDFPage(context.Background(), data, 1); code != "document_encrypted" || len(raw) != 0 {
		t.Fatal(code)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	start := time.Now()
	raw, _, _, code := renderPDFPage(ctx, coloredPDF(72, 144, 0), 1)
	if code == "" || len(raw) != 0 || time.Since(start) > time.Second {
		t.Fatal("cancelled raster started")
	}
}

// Authored vector-only geometry fixture; no third-party document or font assets.
func coloredPDF(width, height, rotate int) []byte {
	stream := "1 0 0 rg 0 0 36 72 re f"
	objects := []string{"<< /Type /Catalog /Pages 2 0 R >>", "<< /Type /Pages /Kids [3 0 R] /Count 1 >>",
		fmt.Sprintf("<< /Type /Page /Parent 2 0 R /MediaBox [0 0 %d %d] /Rotate %d /Resources <<>> /Contents 4 0 R >>", width, height, rotate),
		fmt.Sprintf("<< /Length %d >>\nstream\n%s\nendstream", len(stream), stream)}
	var b bytes.Buffer
	b.WriteString("%PDF-1.7\n")
	offsets := []int{0}
	for i, obj := range objects {
		offsets = append(offsets, b.Len())
		fmt.Fprintf(&b, "%d 0 obj\n%s\nendobj\n", i+1, obj)
	}
	xref := b.Len()
	fmt.Fprintf(&b, "xref\n0 %d\n0000000000 65535 f \n", len(offsets))
	for _, offset := range offsets[1:] {
		fmt.Fprintf(&b, "%010d 00000 n \n", offset)
	}
	fmt.Fprintf(&b, "trailer\n<< /Size %d /Root 1 0 R >>\nstartxref\n%d\n%%%%EOF\n", len(offsets), xref)
	return b.Bytes()
}
