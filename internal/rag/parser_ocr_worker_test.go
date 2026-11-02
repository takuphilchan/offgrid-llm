package rag

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestParserOCRReceiptAuthorityAndQuality(t *testing.T) {
	raster := &pdfPageRaster{PNG: []byte("verified pixels"), Width: 100, Height: 100}
	valid := ocrReceipt{Protocol: 1, Revision: ocrWorkerRevision, PNG_SHA: ocrDigest(raster.PNG), LanguageSHA: ocrEnglishSHA, Width: 100, Height: 100, HOCR: hocrFixture, State: "partial", Attempts: 1}
	encode := func(r ocrReceipt) []byte {
		data, err := json.Marshal(r)
		if err != nil {
			t.Fatal(err)
		}
		return data
	}
	result, err := decodeOCRReceipt(encode(valid), raster)
	if err != nil || result.Text != "café 日本語\nA&B" || result.State != "partial" {
		t.Fatal(result, err)
	}
	for name, change := range map[string]func(*ocrReceipt){
		"source":              func(r *ocrReceipt) { r.PNG_SHA = strings.Repeat("a", 64) },
		"language":            func(r *ocrReceipt) { r.LanguageSHA = strings.Repeat("a", 64) },
		"revision":            func(r *ocrReceipt) { r.Revision = "unreviewed" },
		"width":               func(r *ocrReceipt) { r.Width++ },
		"rotation":            func(r *ocrReceipt) { r.Rotation = 45 },
		"attempts":            func(r *ocrReceipt) { r.Attempts = 5 },
		"false-complete":      func(r *ocrReceipt) { r.State = "complete" },
		"error-with-evidence": func(r *ocrReceipt) { r.Error = "document_timeout" },
		"malformed":           func(r *ocrReceipt) { r.HOCR = "not hOCR" },
	} {
		t.Run(name, func(t *testing.T) {
			bad := valid
			change(&bad)
			if got, err := decodeOCRReceipt(encode(bad), raster); err == nil || got != nil {
				t.Fatal("invalid OCR accepted", got)
			}
		})
	}
	rotated := valid
	rotated.Rotation = 180
	result, err = decodeOCRReceipt(encode(rotated), raster)
	if err != nil || result.Words[0].Bounds != [4]int{70, 90, 99, 98} {
		t.Fatal("source coordinates lost", result, err)
	}
	for _, raw := range [][]byte{append(encode(valid), []byte(`{}`)...), []byte(`{"unknown":true}`), []byte(strings.Repeat("x", (3<<20)+1))} {
		if got, err := decodeOCRReceipt(raw, raster); err == nil || got != nil {
			t.Fatal("invalid envelope accepted")
		}
	}
}

func TestParserOCRMissingAndInvalidLanguageDoesNotDispatch(t *testing.T) {
	code := func(err error) string {
		var problem *ParseError
		if errors.As(err, &problem) {
			return problem.Code
		}
		return ""
	}
	p := NewDocumentParser()
	p.ocrWorker = "must-not-run"
	raster := &pdfPageRaster{PNG: []byte("pixels"), Width: 1, Height: 1}
	if _, err := p.recognizePDFRaster(context.Background(), raster); code(err) != "document_ocr_language_missing" {
		t.Fatal(err)
	}
	if err := p.UseOCRLanguage("eng", []byte("forged language")); code(err) != "document_ocr_language_invalid" || len(p.ocrLanguage) != 0 {
		t.Fatal(err)
	}
}

func TestParserOCRDoesNotExposeUnfinishedExtraction(t *testing.T) {
	p := NewDocumentParser()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	result := &ParseResult{State: ExtractionPartial, Content: "native page", SourceUnits: []SourceUnit{
		{Index: 1, State: "ready", Text: "native page"},
		{Index: 2, State: "ocr_required"},
	}}
	if got, err := p.extractScannedPDFPages(ctx, nil, result); got != nil || !errors.Is(err, context.Canceled) {
		t.Fatal("unfinished evidence escaped cancellation", got, err)
	}
	p.maxExtractedSize = 1
	result.SourceUnits = result.SourceUnits[:1]
	if got, err := p.extractScannedPDFPages(context.Background(), nil, result); got != nil || err == nil {
		t.Fatal("over-limit evidence escaped", got, err)
	}
}

func TestParserOCRRealWorker(t *testing.T) {
	ocr, pdf, language := os.Getenv("OFFGRID_TEST_OCR_WORKER"), os.Getenv("OFFGRID_TEST_DOCUMENT_WORKER"), os.Getenv("OFFGRID_TEST_OCR_LANGUAGE")
	if ocr == "" || pdf == "" || language == "" {
		t.Skip("requires isolated actual OCR/PDF workers and pinned language")
	}
	model, err := os.ReadFile(language)
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	if err := ImportOCRLanguage(context.Background(), dir, "eng", strings.NewReader(string(model))); err != nil {
		t.Fatal(err)
	}
	p := NewDocumentParserWithOCR(dir)
	p.pdfWorker = pdf
	p.pdfCacheDir = t.TempDir()
	p.ocrWorker = ocr
	if p.ocrSetupErr != nil {
		t.Fatal(p.ocrSetupErr)
	}
	t.Run("missing language preserves native pages", func(t *testing.T) {
		source, err := os.ReadFile("testdata/documents/corpus/mixed.pdf")
		if err != nil {
			t.Fatal(err)
		}
		missing := *p
		missing.ocrLanguage = nil
		result, err := missing.ParseContext(context.Background(), source, "mixed.pdf", ".pdf")
		var problem *ParseError
		if !errors.As(err, &problem) || problem.Code != "document_ocr_language_missing" || result == nil || result.State != ExtractionPartial || len(result.SourceUnits) != 2 || strings.TrimSpace(result.SourceUnits[0].Text) == "" || result.SourceUnits[1].State != "ocr_required" {
			t.Fatal("missing language lost partial evidence", result, err)
		}
		assertPartialOCRReviewPersistence(t, source, result)
	})
	for _, row := range []struct{ file, want string }{
		{"corpus/scan.pdf", "Scanned receipt total 42"},
		{"ocr/inputs/visible-rotation-180.pdf", "Harvest total 37 bags"},
	} {
		t.Run(row.file, func(t *testing.T) {
			data, err := os.ReadFile(filepath.Join("testdata/documents", row.file))
			if err != nil {
				t.Fatal(err)
			}
			ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
			defer cancel()
			raster, err := p.rasterPDFPage(ctx, data, 1)
			if err != nil {
				t.Fatal(err)
			}
			result, err := p.recognizePDFRaster(ctx, raster)
			if err != nil {
				t.Fatal(err)
			}
			if result.State != "complete" || result.Text != row.want {
				t.Fatalf("incorrect worker receipt: %+v", result)
			}
		})
	}
	t.Run("full typed PDF extraction", func(t *testing.T) {
		source, err := os.ReadFile("testdata/documents/corpus/mixed.pdf")
		if err != nil {
			t.Fatal(err)
		}
		result, err := p.ParseContext(context.Background(), source, "mixed.pdf", ".pdf")
		if err != nil || result == nil {
			t.Fatal(err)
		}
		if result.State != ExtractionComplete || len(result.SourceUnits) != 2 || !strings.Contains(result.Content, "Scanned page total 34") || strings.Count(result.Content, "Scanned page total 34") != 1 {
			t.Fatal("incomplete or duplicated source", result)
		}
		// Source revisions are fixed-size fingerprints; detailed runtime
		// provenance lives in metadata, not an unbounded concatenated ID.
		if result.SourceUnits[1].ExtractorRevision != ocrExtractionRevision || result.Metadata["ocr_revision"] != ocrWorkerRevision || result.Metadata["ocr_language_sha256"] != ocrEnglishSHA {
			t.Fatal("OCR provenance lost", result)
		}
		assertOCRCitationPersistence(t, result)
	})
	for _, name := range []string{"mixed-same-page.pdf", "mixed-repeated-text.pdf"} {
		t.Run(name, func(t *testing.T) {
			source, err := os.ReadFile(filepath.Join("testdata/documents/ocr/inputs", name))
			if err != nil {
				t.Fatal(err)
			}
			result, err := p.ParseContext(context.Background(), source, name, ".pdf")
			if err != nil {
				t.Fatal(err, result)
			}
			want := "Native stock total 12 bags\nHarvest total 37 bags"
			if name == "mixed-repeated-text.pdf" {
				want = "Harvest total 37 bags\nHarvest total 37 bags"
			}
			if result.State != ExtractionComplete || result.Content != want {
				t.Fatalf("mixed source changed: %+v", result)
			}
			assertOCRCitationPersistence(t, result)
		})
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := p.recognizePDFRaster(ctx, &pdfPageRaster{PNG: []byte("not used")}); err != context.Canceled {
		t.Fatal("cancelled OCR dispatched", err)
	}
}

func TestParserOCRActiveWorkerCancellation(t *testing.T) {
	worker := os.Getenv("OFFGRID_TEST_OCR_WORKER")
	if worker == "" {
		t.Skip("requires the actual isolated OCR worker")
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	input, writer := io.Pipe()
	defer input.Close()
	defer writer.Close()
	cmd := exec.CommandContext(ctx, worker)
	configureParserProcess(cmd)
	cmd.Stdin = input
	cmd.WaitDelay = time.Second
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	// Deliberately hold the document stream open: this is a live owned process,
	// not a pre-dispatch cancellation or a fake worker returning an error.
	cancel()
	_ = writer.Close()
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	select {
	case err := <-done:
		if err == nil || cmd.ProcessState == nil {
			t.Fatal("cancelled worker was not reaped", err)
		}
	case <-time.After(5 * time.Second):
		_ = cmd.Process.Kill()
		t.Fatal("owned OCR worker did not stop within the cancellation bound")
	}
}
