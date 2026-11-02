package rag

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func validRasterReceipt(t *testing.T, source []byte) pdfPageRaster {
	t.Helper()
	var pixels bytes.Buffer
	if err := png.Encode(&pixels, image.NewRGBA(image.Rect(0, 0, 2, 3))); err != nil {
		t.Fatal(err)
	}
	digest := sha256.Sum256(source)
	return pdfPageRaster{Protocol: 1, Revision: pdfRasterRevision, SHA256: hex.EncodeToString(digest[:]), Page: 1, Width: 2, Height: 3, PNG: pixels.Bytes()}
}

func TestParserRasterReceiptBoundaries(t *testing.T) {
	source := []byte("source PDF")
	for _, name := range []string{"valid", "protocol", "revision", "digest", "page", "width", "height", "dimension-limit", "pixel-limit", "png-limit", "missing-pixels", "truncated-png", "crc", "unknown-field", "trailing", "truncated-json", "error-with-pixels", "unknown-error"} {
		t.Run(name, func(t *testing.T) {
			r := validRasterReceipt(t, source)
			switch name {
			case "protocol":
				r.Protocol++
			case "revision":
				r.Revision = "unreviewed"
			case "digest":
				r.SHA256 = "different source"
			case "page":
				r.Page++
			case "width":
				r.Width++
			case "height":
				r.Height = 0
			case "dimension-limit":
				r.Width = 8193
			case "pixel-limit":
				r.Width, r.Height = 8192, 8192
			case "png-limit":
				r.PNG = make([]byte, (24<<20)+1)
			case "missing-pixels":
				r.PNG = nil
			case "truncated-png":
				r.PNG = r.PNG[:len(r.PNG)-5]
			case "crc":
				r.PNG[len(r.PNG)-1] ^= 1
			case "error-with-pixels":
				r.Error = "document_timeout"
			case "unknown-error":
				r.Error, r.PNG, r.Width, r.Height = "private content", nil, 0, 0
			}
			raw, err := json.Marshal(r)
			if err != nil {
				t.Fatal(err)
			}
			switch name {
			case "unknown-field":
				raw = append(raw[:len(raw)-1], []byte(`,"command":"run"}`)...)
			case "trailing":
				raw = append(raw, []byte(" {}")...)
			case "truncated-json":
				raw = raw[:len(raw)-1]
			}
			got, err := decodePDFRaster(raw, source, 1)
			if name == "valid" {
				if err != nil || got == nil || !bytes.Equal(got.PNG, r.PNG) {
					t.Fatalf("valid pixels lost: %v", err)
				}
			} else {
				var problem *ParseError
				if got != nil || !errors.As(err, &problem) || problem.Code != "document_parser_protocol" {
					t.Fatalf("accepted %s: %v", name, err)
				}
			}
		})
	}
}

func TestParserRasterNativeEvidenceBoundaries(t *testing.T) {
	source := []byte("source PDF")
	for _, name := range []string{"valid", "empty", "outside", "reversed", "oversize", "error-with-native"} {
		t.Run(name, func(t *testing.T) {
			r := validRasterReceipt(t, source)
			r.Native = []pdfNativeRegion{{Text: "word", Bounds: [4]int{0, 0, 1, 2}}}
			switch name {
			case "empty":
				r.Native[0].Text = " "
			case "outside":
				r.Native[0].Bounds[2] = r.Width + 1
			case "reversed":
				r.Native[0].Bounds[3] = 0
			case "oversize":
				r.Native = make([]pdfNativeRegion, 20001)
			case "error-with-native":
				r.Error = "document_timeout"
				r.PNG = nil
				r.Width = 0
				r.Height = 0
			}
			raw, _ := json.Marshal(r)
			got, err := decodePDFRaster(raw, source, 1)
			if name == "valid" {
				if err != nil || len(got.Native) != 1 {
					t.Fatal(got, err)
				}
			} else if err == nil || got != nil {
				t.Fatal("invalid source regions accepted", got)
			}
		})
	}
}

func TestParserRasterTypedFailures(t *testing.T) {
	source := []byte("PDF")
	for _, code := range []string{"document_invalid", "document_encrypted", "document_empty", "document_limit", "document_timeout", "document_parser_unavailable"} {
		r := validRasterReceipt(t, source)
		r.Error, r.PNG, r.Width, r.Height = code, nil, 0, 0
		raw, _ := json.Marshal(r)
		got, err := decodePDFRaster(raw, source, 1)
		var problem *ParseError
		if got != nil || !errors.As(err, &problem) || problem.Code != code {
			t.Fatalf("failure lost: %s: %v", code, err)
		}
	}
}

func TestParserRasterAdmission(t *testing.T) {
	p := NewDocumentParser()
	p.pdfWorker = "must-not-dispatch"
	for _, page := range []int{0, -1, 1001} {
		_, err := p.rasterPDFPage(context.Background(), []byte("PDF"), page)
		var problem *ParseError
		if !errors.As(err, &problem) || problem.Code != "document_limit" {
			t.Fatalf("invalid page dispatched: %v", err)
		}
	}
	pdfWorkerAdmission <- struct{}{}
	defer func() { <-pdfWorkerAdmission }()
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	if _, err := p.rasterPDFPage(ctx, []byte("PDF"), 1); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("queued cancellation lost: %v", err)
	}
}

func TestParserRasterRealWorker(t *testing.T) {
	worker := os.Getenv("OFFGRID_TEST_DOCUMENT_WORKER")
	if worker == "" {
		t.Skip("build runtimes/pdfium and set OFFGRID_TEST_DOCUMENT_WORKER")
	}
	if !filepath.IsAbs(worker) {
		t.Fatal("worker path must be absolute")
	}
	p := NewDocumentParser()
	p.pdfWorker, p.pdfCacheDir = worker, t.TempDir()
	t.Setenv("PATH", t.TempDir())
	for _, test := range []struct {
		name string
		page int
	}{{"scan.pdf", 1}, {"scan-rotated.pdf", 1}, {"mixed.pdf", 2}} {
		t.Run(test.name, func(t *testing.T) {
			data, err := os.ReadFile(filepath.Join("testdata/documents/corpus", test.name))
			if err != nil {
				t.Fatal(err)
			}
			r, err := p.rasterPDFPage(context.Background(), data, test.page)
			if err != nil {
				t.Fatal(err)
			}
			if r.Width < 100 || r.Height < 100 || r.Page != test.page {
				t.Fatalf("invalid page: %+v", r)
			}
		})
	}
	data, err := os.ReadFile("testdata/documents/corpus/scan.pdf")
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 80*time.Millisecond)
	defer cancel()
	start := time.Now()
	if _, err := p.rasterPDFPage(ctx, data, 1); !errors.Is(err, context.DeadlineExceeded) || time.Since(start) > 3*time.Second {
		t.Fatalf("render process cancellation failed: %v", err)
	}
}
