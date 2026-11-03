package main

import (
	"bytes"
	"context"
	"fmt"
	"strings"
	"testing"
)

func formPDF(raster bool, nested bool, textOverride ...string) []byte {
	stream := "BT /F1 14 Tf 20 100 Td (Native text) Tj ET /Fm Do"
	if len(textOverride) > 0 {
		stream = textOverride[0]
	}
	form := "0 0 1 rg 10 10 20 20 re f"
	resources := "<<>>"
	if raster {
		form = "q 30 0 0 30 0 0 cm /Im Do Q"
		resources = "<< /XObject << /Im 7 0 R >> >>"
	}
	if nested {
		form = "/Nested Do"
		resources = "<< /XObject << /Nested 8 0 R >> >>"
	}
	objectStream := func(dict, body string) string {
		return fmt.Sprintf("<< %s /Length %d >>\nstream\n%s\nendstream", dict, len(body), body)
	}
	objects := []string{
		"<< /Type /Catalog /Pages 2 0 R >>", "<< /Type /Pages /Kids [3 0 R] /Count 1 >>",
		"<< /Type /Page /Parent 2 0 R /MediaBox [0 0 200 200] /Resources << /Font << /F1 5 0 R >> /XObject << /Fm 6 0 R >> >> /Contents 4 0 R >>",
		objectStream("", stream), "<< /Type /Font /Subtype /Type1 /BaseFont /Helvetica >>",
		objectStream("/Type /XObject /Subtype /Form /BBox [0 0 200 200] /Resources "+resources, form),
		objectStream("/Type /XObject /Subtype /Image /Width 1 /Height 1 /ColorSpace /DeviceRGB /BitsPerComponent 8", "abc"),
		objectStream("/Type /XObject /Subtype /Form /BBox [0 0 200 200] /Resources << /XObject << /Im 7 0 R >> >>", func() string {
			if raster {
				return "q 30 0 0 30 0 0 cm /Im Do Q"
			}
			return "0 0 1 rg 10 10 20 20 re f"
		}()),
	}
	var b bytes.Buffer
	b.WriteString("%PDF-1.7\n")
	offsets := []int{0}
	for i, obj := range objects {
		offsets = append(offsets, b.Len())
		fmt.Fprintf(&b, "%d 0 obj\n%s\nendobj\n", i+1, obj)
	}
	start := b.Len()
	fmt.Fprintf(&b, "xref\n0 %d\n0000000000 65535 f \n", len(offsets))
	for _, offset := range offsets[1:] {
		fmt.Fprintf(&b, "%010d 00000 n \n", offset)
	}
	fmt.Fprintf(&b, "trailer\n<< /Size %d /Root 1 0 R >>\nstartxref\n%d\n%%%%EOF\n", len(offsets), start)
	return b.Bytes()
}

func TestFormsAreNotScans(t *testing.T) {
	t.Setenv("OFFGRID_DOCUMENT_CODE_CACHE", t.TempDir())
	for _, nested := range []bool{false, true} {
		for _, raster := range []bool{false, true} {
			t.Run(fmt.Sprintf("nested=%v/raster=%v", nested, raster), func(t *testing.T) {
				pages, code := extract(context.Background(), formPDF(raster, nested))
				if code != "" || len(pages) != 1 || !strings.Contains(pages[0].Text, "Native text") {
					t.Fatalf("%s %+v", code, pages)
				}
				want := "ready"
				if raster {
					want = "ocr_required"
				}
				if pages[0].State != want {
					t.Fatalf("got %s want %s", pages[0].State, want)
				}
			})
		}
	}
}

func TestFormInspectionBoundPreservesUncertainty(t *testing.T) {
	remaining := 0
	if needs, code := formNeedsOCR(nil, 0, 0, &remaining); !needs || code != "" {
		t.Fatal("bounded structure must remain partial", needs, code)
	}
	remaining = 100
	if needs, code := formNeedsOCR(nil, 0, 32, &remaining); !needs || code != "" {
		t.Fatal("deep structure must remain partial", needs, code)
	}
}
