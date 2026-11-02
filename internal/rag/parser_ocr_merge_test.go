package rag

import (
	"testing"

	"github.com/takuphilchan/offgrid-llm/internal/documentocr"
)

func TestParserOCRMergeUsesLocationNotRepeatedText(t *testing.T) {
	raster := &pdfPageRaster{Width: 100, Height: 100, Native: []pdfNativeRegion{{Text: "café", Bounds: [4]int{10, 10, 40, 20}}}}
	page := &ocrPageResult{Words: []documentocr.Word{
		{Text: "cafe", Bounds: [4]int{9, 9, 41, 21}, Confidence: 99},
		{Text: "café", Bounds: [4]int{10, 40, 40, 50}, Confidence: 99},
	}}
	got, err := mergeNativeOCR("café", raster, page)
	if err != nil || got != "café\ncafé" {
		t.Fatal("native spelling or distinct repeated text lost", got, err)
	}
	if _, err = mergeNativeOCR("different", raster, page); err == nil {
		t.Fatal("mismatched native source accepted")
	}
	page.Words[0].Bounds = [4]int{35, 10, 60, 20}
	if _, err = mergeNativeOCR("café", raster, page); err == nil {
		t.Fatal("ambiguous overlapping text dropped")
	}
	raster.Native = []pdfNativeRegion{{Text: "first", Bounds: [4]int{50, 10, 80, 20}}, {Text: "second", Bounds: [4]int{10, 10, 40, 20}}}
	page.Words = nil
	if _, err = mergeNativeOCR("first second", raster, page); err == nil {
		t.Fatal("native reading order silently reversed")
	}
}
