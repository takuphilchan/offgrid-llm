package rag

import (
	"fmt"
	"sort"
	"strings"
)

// Preserve native words where the native and OCR engines describe the same
// location. Equal text elsewhere is not a duplicate. Ambiguous intersections
// fail reconciliation rather than silently removing potentially different text.
func mergeNativeOCR(nativeText string, raster *pdfPageRaster, page *ocrPageResult) (string, error) {
	if len(raster.Native)*len(page.Words) > 2_000_000 {
		return "", fmt.Errorf("mixed page exceeds reconciliation budget")
	}
	var nativeWords []string
	for _, region := range raster.Native {
		nativeWords = append(nativeWords, region.Text)
	}
	if strings.Join(strings.Fields(strings.Join(nativeWords, " ")), " ") != strings.Join(strings.Fields(nativeText), " ") {
		return "", fmt.Errorf("native text and raster evidence disagree")
	}
	type orderedRegion struct {
		pdfNativeRegion
		nativeIndex int
	}
	regions := make([]orderedRegion, 0, len(raster.Native)+len(page.Words))
	for i, r := range raster.Native {
		regions = append(regions, orderedRegion{r, i})
	}
	for _, word := range page.Words {
		area := (word.Bounds[2] - word.Bounds[0]) * (word.Bounds[3] - word.Bounds[1])
		covered := 0
		for _, native := range raster.Native {
			w := min(native.Bounds[2], word.Bounds[2]) - max(native.Bounds[0], word.Bounds[0])
			h := min(native.Bounds[3], word.Bounds[3]) - max(native.Bounds[1], word.Bounds[1])
			if w > 0 && h > 0 {
				covered += w * h
			}
		}
		if covered > 0 {
			// A native glyph box is tighter than an OCR word box, hence the
			// threshold permits small OCR margins but rejects crossing regions.
			if covered*100 < area*60 || covered > area {
				return "", fmt.Errorf("ambiguous native/OCR overlap")
			}
			continue
		}
		regions = append(regions, orderedRegion{pdfNativeRegion{Text: word.Text, Bounds: word.Bounds}, -1})
	}
	// Page order is spatial after OCR; native Unicode is never replaced with an
	// OCR spelling. A row groups vertical overlap, then follows horizontal order.
	sort.SliceStable(regions, func(i, j int) bool { return regions[i].Bounds[1] < regions[j].Bounds[1] })
	var lines []string
	nextNative := 0
	for len(regions) > 0 {
		bottom := regions[0].Bounds[3]
		n := 1
		for n < len(regions) && regions[n].Bounds[1] < bottom {
			bottom = min(bottom, regions[n].Bounds[3])
			n++
		}
		row := regions[:n]
		sort.SliceStable(row, func(i, j int) bool { return row[i].Bounds[0] < row[j].Bounds[0] })
		words := make([]string, len(row))
		for i, r := range row {
			if r.nativeIndex >= 0 {
				if r.nativeIndex != nextNative {
					return "", fmt.Errorf("spatial order would change native reading order")
				}
				nextNative++
			}
			words[i] = r.Text
		}
		lines = append(lines, strings.Join(words, " "))
		regions = regions[n:]
	}
	return strings.Join(lines, "\n"), nil
}
