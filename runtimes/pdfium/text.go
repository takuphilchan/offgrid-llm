package main

import (
	"encoding/binary"
	"unicode/utf16"
)

func readPage(engine *pdfEngine, page uint64, remaining int) (string, bool, string) {
	text, code := pageText(engine, page, remaining)
	if code != "" {
		return "", false, code
	}
	needsOCR, code := pageNeedsOCR(engine, page, text)
	return text, needsOCR, code
}

func pageText(engine *pdfEngine, page uint64, remaining int) (string, string) {
	loaded, err := engine.call("FPDFText_LoadPage", page)
	if err != nil || loaded == 0 {
		return "", "document_invalid"
	}
	defer engine.call("FPDFText_ClosePage", loaded)
	count, err := engine.call("FPDFText_CountChars", loaded)
	if err != nil || int32(count) < 0 {
		return "", "document_invalid"
	}
	if count > uint64(remaining/4) {
		return "", "document_limit"
	}
	if count == 0 {
		return "", ""
	}
	// PDFium's UTF-16 buffer includes a terminating NUL. Allocate for the
	// documented count+1 units before dispatch, not a parser-reported byte size.
	ptr, err := engine.allocate((count + 1) * 2)
	if err != nil {
		return "", "document_limit"
	}
	defer engine.call("free", ptr)
	written, err := engine.call("FPDFText_GetText", loaded, 0, count, ptr)
	if err != nil || written < 1 || written > count+1 {
		return "", "document_invalid"
	}
	raw, ok := engine.module.Memory().Read(uint32(ptr), uint32(written*2))
	if !ok || binary.LittleEndian.Uint16(raw[len(raw)-2:]) != 0 {
		return "", "document_invalid"
	}
	units := make([]uint16, written-1)
	for i := range units {
		units[i] = binary.LittleEndian.Uint16(raw[i*2:])
	}
	// Do not silently replace malformed UTF-16 with a replacement character.
	for i := 0; i < len(units); i++ {
		if units[i] >= 0xd800 && units[i] <= 0xdbff {
			if i+1 >= len(units) || units[i+1] < 0xdc00 || units[i+1] > 0xdfff {
				return "", "document_invalid"
			}
			i++
		} else if units[i] >= 0xdc00 && units[i] <= 0xdfff {
			return "", "document_invalid"
		}
	}
	text := string(utf16.Decode(units))
	if len(text) > remaining {
		return "", "document_limit"
	}
	return text, ""
}
