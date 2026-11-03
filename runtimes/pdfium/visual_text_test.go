package main

import (
	"context"
	"strings"
	"testing"
)

func TestNativeVisualTextDoesNotAcceptScannedSurfacesOrHeaders(t *testing.T) {
	t.Setenv("OFFGRID_DOCUMENT_CODE_CACHE", t.TempDir())
	text := strings.Repeat("Native document content has readable text. ", 8)
	for _, row := range []struct{ name, transform, text, want string }{
		{"figure", "", text, "native_text_with_visuals"},
		{"scan", "q 6 0 0 6 0 0 cm ", text, "ocr_required"},
		{"header", "", "Invoice heading", "ocr_required"},
		{"scan-only", "", "", "ocr_required"},
	} {
		t.Run(row.name, func(t *testing.T) {
			stream := "BT /F1 3 Tf 2 180 Td (" + row.text + ") Tj ET " + row.transform + "/Fm Do"
			if row.transform != "" {
				stream += " Q"
			}
			pages, code := extract(context.Background(), formPDF(true, true, stream))
			if code != "" || len(pages) != 1 || pages[0].State != row.want {
				t.Fatalf("%s %+v", code, pages)
			}
			if row.text != "" && !strings.Contains(pages[0].Text, strings.TrimSpace(row.text)) {
				t.Fatal("native text changed")
			}
		})
	}
}
