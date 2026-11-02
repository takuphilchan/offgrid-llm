package rag

import "github.com/takuphilchan/offgrid-llm/internal/documentocr"

type ocrWord = documentocr.Word

func parseHOCR(content string, width, height int) ([]ocrWord, error) {
	return documentocr.ParseHOCR(content, width, height)
}
