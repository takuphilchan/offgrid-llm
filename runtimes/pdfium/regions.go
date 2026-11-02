package main

import (
	"encoding/binary"
	"math"
	"strings"
	"unicode"
	"unicode/utf8"
)

// Coordinates use the exact device transform used for the returned raster.
// Regions represent native words, not OCR guesses or substring matches.
type nativeRegion struct {
	Text   string `json:"text"`
	Bounds [4]int `json:"bounds"`
}

func nativeRegions(e *pdfEngine, page uint64, width, height int) ([]nativeRegion, string) {
	textPage, err := e.call("FPDFText_LoadPage", page)
	if err != nil || textPage == 0 {
		return nil, "document_invalid"
	}
	defer e.call("FPDFText_ClosePage", textPage)
	count, err := e.call("FPDFText_CountChars", textPage)
	if err != nil || int32(count) < 0 {
		return nil, "document_invalid"
	}
	// Each character requires bounded geometry calls; dense pages must not turn
	// raster preparation into an unbounded per-character worker loop.
	if count > 20000 {
		return nil, "document_limit"
	}
	ptr, err := e.allocate(40)
	if err != nil {
		return nil, "document_limit"
	}
	defer e.call("free", ptr)
	var regions []nativeRegion
	var word strings.Builder
	var box [4]int
	flush := func() {
		if word.Len() != 0 {
			regions = append(regions, nativeRegion{word.String(), box})
			word.Reset()
		}
	}
	for i := uint64(0); i < count; i++ {
		value, err := e.call("FPDFText_GetUnicode", textPage, i)
		if err != nil || value == 0 || value > utf8.MaxRune || !utf8.ValidRune(rune(value)) {
			return nil, "document_invalid"
		}
		char := rune(value)
		if unicode.IsSpace(char) {
			flush()
			continue
		}
		ok, err := e.call("FPDFText_GetCharBox", textPage, i, ptr, ptr+8, ptr+16, ptr+24)
		if err != nil || ok == 0 {
			return nil, "document_invalid"
		}
		raw, valid := e.module.Memory().Read(uint32(ptr), 32)
		if !valid {
			return nil, "document_invalid"
		}
		coords := [4]float64{}
		for j := range coords {
			coords[j] = math.Float64frombits(binary.LittleEndian.Uint64(raw[j*8:]))
			if math.IsInf(coords[j], 0) || math.IsNaN(coords[j]) {
				return nil, "document_invalid"
			}
		}
		// PDFium writes left,right,bottom,top in page coordinates. Pass both
		// corners through PageToDevice so crop boxes and page rotation agree.
		var points [4]int
		for j, point := range [][2]float64{{coords[0], coords[3]}, {coords[1], coords[2]}} {
			ok, err = e.call("FPDF_PageToDevice", page, 0, 0, uint64(width), uint64(height), 0, math.Float64bits(point[0]), math.Float64bits(point[1]), ptr+32, ptr+36)
			if err != nil || ok == 0 {
				return nil, "document_invalid"
			}
			x, xOK := e.module.Memory().ReadUint32Le(uint32(ptr + 32))
			y, yOK := e.module.Memory().ReadUint32Le(uint32(ptr + 36))
			if !xOK || !yOK {
				return nil, "document_invalid"
			}
			points[j*2], points[j*2+1] = int(int32(x)), int(int32(y))
		}
		b := [4]int{min(points[0], points[2]), min(points[1], points[3]), max(points[0], points[2]), max(points[1], points[3])}
		if b[0] < 0 || b[1] < 0 || b[2] > width || b[3] > height || b[0] >= b[2] || b[1] >= b[3] {
			return nil, "document_invalid"
		}
		if word.Len() == 0 {
			box = b
		} else {
			box = [4]int{min(box[0], b[0]), min(box[1], b[1]), max(box[2], b[2]), max(box[3], b[3])}
		}
		word.WriteRune(char)
	}
	flush()
	return regions, ""
}
