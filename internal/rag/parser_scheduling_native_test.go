package rag

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// This measures actual bundled reader execution, not model answer quality.
// Cold means an empty PDF compilation cache; warm still starts a fresh worker.
func TestParserSchedulingNativeMeasurements(t *testing.T) {
	pdf, ocr, language := os.Getenv("OFFGRID_TEST_DOCUMENT_WORKER"), os.Getenv("OFFGRID_TEST_OCR_WORKER"), os.Getenv("OFFGRID_TEST_OCR_LANGUAGE")
	if pdf == "" || ocr == "" || language == "" {
		t.Skip("requires actual pinned PDF/OCR workers and offline language pack")
	}
	pack, err := os.ReadFile(language)
	if err != nil {
		t.Fatal(err)
	}
	for _, sample := range []struct{ file, want string }{
		{"compressed.pdf", "The library closes at 17:00."},
		{"scan.pdf", "Scanned receipt total 42"},
	} {
		t.Run(sample.file, func(t *testing.T) {
			parser := NewDocumentParser()
			parser.pdfWorker, parser.ocrWorker, parser.pdfCacheDir = pdf, ocr, t.TempDir()
			if err := parser.UseOCRLanguage("eng", pack); err != nil {
				t.Fatal(err)
			}
			data, err := os.ReadFile(filepath.Join("testdata/documents/corpus", sample.file))
			if err != nil {
				t.Fatal(err)
			}
			for _, phase := range []string{"cold", "warm"} {
				ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
				started := time.Now()
				result, err := parser.ParseContext(ctx, data, sample.file, ".pdf")
				elapsed := time.Since(started)
				cancel()
				if err != nil || result == nil || result.State != ExtractionComplete || !strings.Contains(result.Content, sample.want) {
					t.Fatalf("%s extraction failed: result=%+v error=%v", phase, result, err)
				}
				t.Logf("%s %s elapsed=%s input_bytes=%d extracted_bytes=%d", sample.file, phase, elapsed, len(data), len(result.Content))
			}
		})
	}
}
