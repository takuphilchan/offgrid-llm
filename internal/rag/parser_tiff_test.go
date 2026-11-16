package rag

import (
	"bytes"
	"context"
	"errors"
	"github.com/takuphilchan/offgrid-llm/internal/documentocr"
	"github.com/takuphilchan/offgrid-llm/internal/testsupport/tifffixture"
	"image/png"
	"strings"
	"testing"
)

func TestTIFFOCRPreservesPageEvidenceAndPartialCoverage(t *testing.T) {
	for _, partial := range []bool{false, true} {
		p := NewDocumentParser()
		calls := 0
		source := tifffixture.TwoPages()
		r, err := p.parseTIFF(context.Background(), source, true, func(_ context.Context, page *pdfPageRaster) (*ocrPageResult, error) {
			calls++
			if page.Page != calls {
				t.Fatal("wrong page order")
			}
			state, text := "complete", "Amount 42"
			if page.Page == 2 {
				text = "Exception 17"
				if partial {
					state = "partial"
				}
			}
			parts := strings.Fields(text)
			return &ocrPageResult{State: state, Text: text, Words: []documentocr.Word{{Text: parts[0], Line: 1, Bounds: [4]int{1, 1, 50, 15}}, {Text: parts[1], Line: 1, Bounds: [4]int{51, 1, 75, 15}}}}, nil
		})
		if calls != 2 || r == nil || r.PageCount != 2 || r.Structure == nil || r.Structure.Verify() != nil {
			t.Fatal(r, err, calls)
		}
		if partial != (err != nil) || r.Content != "Amount 42\n\nException 17" {
			t.Fatal(r.Content, err)
		}
		for i, u := range r.Structure.Units {
			if u.Index != i+1 || r.Content[u.Span.Start:u.Span.End] != r.SourceUnits[i].Text || r.Structure.Blocks[i].UnitID != u.ID {
				t.Fatal("page lineage", r.Structure)
			}
		}
		if partial {
			if r.State != ExtractionPartial || r.Coverage().ReadableUnits != 1 {
				t.Fatal(r.Coverage())
			}
		} else {
			if _, ranges, e := parsedSourceRanges(r); e != nil || len(ranges) != 2 {
				t.Fatal(ranges, e)
			}
		}
	}
}

func TestTIFFPreviewNeverCallsOCRAndRendersSelectedPage(t *testing.T) {
	p := NewDocumentParser()
	data := tifffixture.TwoPages()
	r, err := p.PreviewContext(context.Background(), data, "scan.tif", ".tif")
	if r == nil || r.PageCount != 2 || r.Content != "" || err == nil {
		t.Fatal(r, err)
	}
	for i, want := range []uint32{230 * 257, 80 * 257} {
		pixels, err := p.RenderImagePage(context.Background(), data, ".tif", i+1)
		if err != nil {
			t.Fatal(err)
		}
		img, err := png.Decode(bytes.NewReader(pixels))
		if err != nil {
			t.Fatal(err)
		}
		got, _, _, _ := img.At(0, 0).RGBA()
		if got != want {
			t.Fatal("page pixels", got, want)
		}
	}
	if _, err := p.RenderImagePage(context.Background(), data, ".tif", 3); err == nil {
		t.Fatal("missing page accepted")
	}
	ctx, cancel := context.WithCancel(context.Background())
	calls := 0
	r, err = p.parseTIFF(ctx, data, true, func(context.Context, *pdfPageRaster) (*ocrPageResult, error) {
		calls++
		cancel()
		return nil, ctx.Err()
	})
	if !errors.Is(err, context.Canceled) || r != nil || calls != 1 {
		t.Fatal(r, err, calls)
	}
}
