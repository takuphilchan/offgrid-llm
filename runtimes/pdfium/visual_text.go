package main

import (
	"encoding/binary"
	"math"
	"unicode"
)

// This is admission for native text with an explicit visual-coverage limit,
// never a claim that images have been read. Sparse native headers and large
// scanned surfaces must still enter OCR. Bounds include an object's own matrix;
// using the enclosing form's bounds avoids guessing nested transforms.
func nativeTextWithVisuals(e *pdfEngine, page uint64, text string) bool {
	letters := 0
	for _, r := range text {
		if unicode.IsLetter(r) || unicode.IsNumber(r) {
			letters++
		}
	}
	if letters < 200 {
		return false
	}
	w, ew := e.call("FPDF_GetPageWidthF", page)
	h, eh := e.call("FPDF_GetPageHeightF", page)
	width, height := float64(math.Float32frombits(uint32(w))), float64(math.Float32frombits(uint32(h)))
	if ew != nil || eh != nil || !finitePositive(width) || !finitePositive(height) {
		return false
	}
	count, err := e.call("FPDFPage_CountObjects", page)
	if err != nil || count > 10000 {
		return false
	}
	ptr, err := e.allocate(16)
	if err != nil {
		return false
	}
	defer e.call("free", ptr)
	area := 0.0
	remaining := 10000 - int(count)
	for i := uint64(0); i < count; i++ {
		obj, err := e.call("FPDFPage_GetObject", page, i)
		if err != nil || obj == 0 {
			return false
		}
		kind, err := e.call("FPDFPageObj_GetType", obj)
		if err != nil {
			return false
		}
		if kind != 3 && kind != 5 {
			continue
		}
		if kind == 5 {
			needs, code := formNeedsOCR(e, obj, 0, &remaining)
			if code != "" {
				return false
			}
			if !needs {
				continue
			}
		}
		ok, err := e.call("FPDFPageObj_GetBounds", obj, ptr, ptr+4, ptr+8, ptr+12)
		if err != nil || ok == 0 {
			return false
		}
		raw, valid := e.module.Memory().Read(uint32(ptr), 16)
		if !valid {
			return false
		}
		var b [4]float64
		for j := range b {
			b[j] = float64(math.Float32frombits(binary.LittleEndian.Uint32(raw[j*4:])))
		}
		bw, bh := b[2]-b[0], b[3]-b[1]
		if !finitePositive(bw) || !finitePositive(bh) {
			return false
		}
		// Sum enclosing bounds conservatively (overlap is not subtracted).
		// A full-page scan with an added native heading cannot pass this gate.
		area += bw * bh
		if area >= width*height*0.60 {
			return false
		}
	}
	return true
}

func finitePositive(v float64) bool { return v > 0 && !math.IsNaN(v) && !math.IsInf(v, 0) }
