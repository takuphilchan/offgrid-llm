package main

import (
	"context"
	"os"
	"strings"
	"testing"
)

func TestNativeRegionsUseRasterCoordinates(t *testing.T) {
	t.Setenv("OFFGRID_DOCUMENT_CODE_CACHE", t.TempDir())
	source, err := os.ReadFile("../../internal/rag/testdata/documents/ocr/inputs/mixed-same-page.pdf")
	if err != nil {
		t.Fatal(err)
	}
	var regions []nativeRegion
	_, w, h, code := renderPDFPage(context.Background(), source, 1, &regions)
	if code != "" {
		t.Fatal(code)
	}
	var words []string
	for _, r := range regions {
		words = append(words, r.Text)
		if r.Bounds[0] < 0 || r.Bounds[1] < 0 || r.Bounds[2] > w || r.Bounds[3] > h || r.Bounds[0] >= r.Bounds[2] || r.Bounds[1] >= r.Bounds[3] {
			t.Fatal("invalid native region", r)
		}
		// Authored native heading is above the scanned content at y=180..244.
		if r.Bounds[3] > 400 {
			t.Fatal("native text mapped below heading", r)
		}
	}
	if strings.Join(words, " ") != "Native stock total 12 bags" {
		t.Fatal(words)
	}
}
