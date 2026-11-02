package rag

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestParserWorkerProcessHelper(t *testing.T) {
	mode := os.Args[len(os.Args)-1]
	if !strings.HasPrefix(mode, "offgrid-parser-helper:") {
		return
	}
	_, _ = io.Copy(io.Discard, os.Stdin)
	switch mode {
	case "offgrid-parser-helper:crash":
		_, _ = os.Stderr.WriteString("private worker diagnostic must not escape")
		os.Exit(7)
	case "offgrid-parser-helper:truncated":
		_, _ = os.Stdout.WriteString(`{"protocol":1`)
	case "offgrid-parser-helper:excessive":
		_, _ = os.Stdout.WriteString(strings.Repeat("x", 21<<20))
	case "offgrid-parser-helper:hang":
		time.Sleep(time.Hour)
	}
	os.Exit(0)
}

func TestParserWorkerProcessFailureBoundaries(t *testing.T) {
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	for _, mode := range []string{"crash", "truncated", "excessive", "hang"} {
		t.Run(mode, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()
			cmd := exec.CommandContext(ctx, executable, "-test.run=^TestParserWorkerProcessHelper$", "--", "offgrid-parser-helper:"+mode)
			result, err := NewDocumentParser().runPDFWorker(ctx, cmd, []byte("%PDF-fixture"))
			if result != nil || err == nil || strings.Contains(err.Error(), "private worker") {
				t.Fatalf("unsafe %s outcome: %v %v", mode, result, err)
			}
			if cmd.ProcessState == nil || !cmd.ProcessState.Exited() && ctx.Err() == nil {
				t.Fatal("owned worker was not waited on")
			}
			if mode == "hang" && !errors.Is(err, context.DeadlineExceeded) {
				t.Fatalf("deadline lost: %v", err)
			}
			// A failed worker does not poison receipt admission for the next file.
			source := []byte("next source")
			raw, _ := json.Marshal(validWorkerReceipt(source))
			if _, err := NewDocumentParser().decodePDFWorker(raw, source); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func validWorkerReceipt(source []byte) map[string]any {
	digest := sha256.Sum256(source)
	return map[string]any{"protocol": 1, "revision": pdfWorkerRevision, "sha256": hex.EncodeToString(digest[:]),
		"pages": []any{map[string]any{"page": 1, "text": "Café 日本語", "state": "ready"}}}
}

func TestParserWorkerRejectsInvalidReceipts(t *testing.T) {
	source := []byte("document bytes")
	for _, name := range []string{"version", "digest", "revision", "extra", "page", "state", "empty-ready", "error-with-pages", "unknown-error", "trailing", "truncated", "oversize"} {
		t.Run(name, func(t *testing.T) {
			r := validWorkerReceipt(source)
			p := NewDocumentParser()
			switch name {
			case "version":
				r["protocol"] = 2
			case "digest":
				r["sha256"] = strings.Repeat("0", 64)
			case "revision":
				r["revision"] = "unreviewed"
			case "extra":
				r["command"] = "execute"
			case "page":
				r["pages"].([]any)[0].(map[string]any)["page"] = 2
			case "state":
				r["pages"].([]any)[0].(map[string]any)["state"] = "maybe"
			case "empty-ready":
				r["pages"].([]any)[0].(map[string]any)["text"] = " "
			case "error-with-pages":
				r["error"] = "document_invalid"
			case "unknown-error":
				r["error"] = "private contents"
				delete(r, "pages")
			case "oversize":
				p.maxExtractedSize = 1
			}
			data, _ := json.Marshal(r)
			if name == "trailing" {
				data = append(data, []byte(" {}")...)
			}
			if name == "truncated" {
				data = data[:len(data)-1]
			}
			result, err := p.decodePDFWorker(data, source)
			if err == nil || result != nil {
				t.Fatalf("accepted %s receipt", name)
			}
		})
	}
}

func TestParserWorkerPreservesPartialPagesWithoutReadyClaim(t *testing.T) {
	source := []byte("document bytes")
	r := validWorkerReceipt(source)
	r["pages"] = append(r["pages"].([]any), map[string]any{"page": 2, "text": "", "state": "ocr_required"})
	data, _ := json.Marshal(r)
	result, err := NewDocumentParser().decodePDFWorker(data, source)
	var problem *ParseError
	if !errors.As(err, &problem) || problem.Code != "document_ocr_required" || result == nil || len(result.SourceUnits) != 2 || result.Content != "Café 日本語" || result.Metadata["extraction_state"] != "partial" {
		t.Fatalf("lost partial state: result=%+v err=%v", result, err)
	}
	if result.State != ExtractionPartial || len(result.Warnings) != 1 || result.Warnings[0].SourceIndex != 2 || result.Warnings[0].Code != "document_ocr_required" {
		t.Fatalf("missing typed partial-page warning: %+v", result)
	}
	for i, unit := range result.SourceUnits {
		if unit.Kind != "page" || unit.Index != i+1 || unit.SourceSHA256 != r["sha256"] || unit.ExtractorRevision != pdfWorkerRevision {
			t.Fatal("unbound page identity")
		}
	}
}

func TestParserWorkerUnavailableDoesNotFallback(t *testing.T) {
	p := NewDocumentParser()
	p.pdfWorker = filepath.Join(t.TempDir(), "missing-worker")
	result, err := p.Parse(parserPDF(t, false), "valid.pdf", ".pdf")
	var problem *ParseError
	if result != nil || !errors.As(err, &problem) || problem.Code != "document_parser_unavailable" {
		t.Fatalf("worker failure fell back: %v %v", result, err)
	}
}

func TestParserWorkerQueuedCancellation(t *testing.T) {
	pdfWorkerAdmission <- struct{}{}
	defer func() { <-pdfWorkerAdmission }()
	p := NewDocumentParser()
	p.pdfWorker = "must-not-dispatch"
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	_, err := p.ParseContext(ctx, []byte("%PDF-"), "test.pdf", ".pdf")
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("cancelled waiter dispatched: %v", err)
	}
}

// Actual compiled, pinned engine; no fake extractor or installed-user state.
func TestParserWorkerRealCorpus(t *testing.T) {
	worker := os.Getenv("OFFGRID_TEST_DOCUMENT_WORKER")
	if worker == "" {
		t.Skip("build runtimes/pdfium and set OFFGRID_TEST_DOCUMENT_WORKER")
	}
	if !filepath.IsAbs(worker) {
		t.Fatal("test worker must have an absolute path")
	}
	p := NewDocumentParser()
	p.pdfWorker = worker
	p.pdfCacheDir = t.TempDir()
	t.Setenv("PATH", t.TempDir())
	for _, fixture := range readParserCorpus(t) {
		if filepath.Ext(fixture.Path) != ".pdf" {
			continue
		}
		t.Run(fixture.Path, func(t *testing.T) {
			data, err := os.ReadFile(filepath.Join("testdata/documents/corpus", fixture.Path))
			if err != nil {
				t.Fatal(err)
			}
			result, err := p.ParseContext(context.Background(), data, fixture.Path, ".pdf")
			if fixture.ExpectedError != "" {
				var problem *ParseError
				if !errors.As(err, &problem) || problem.Code != fixture.ExpectedError {
					t.Fatalf("want %s got %v", fixture.ExpectedError, err)
				}
				if fixture.Family == "pdf_mixed" && (result == nil || len(result.SourceUnits) != 2 || !strings.Contains(result.SourceUnits[0].Text, fixture.ExpectedText[0])) {
					t.Fatal("mixed native page lost")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if len(result.SourceUnits) != len(fixture.SourceUnits) {
				t.Fatalf("wrong page units: %+v", result.SourceUnits)
			}
			text := strings.Join(strings.Fields(result.Content), " ")
			for _, expected := range fixture.ExpectedText {
				index := strings.Index(text, strings.Join(strings.Fields(expected), " "))
				if index < 0 {
					t.Fatalf("missing/out-of-order fixture text %q", expected)
				}
				text = text[index+len(strings.Join(strings.Fields(expected), " ")):]
			}
		})
	}
	t.Run("cancel-running-worker", func(t *testing.T) {
		data, _ := os.ReadFile("testdata/documents/corpus/compressed.pdf")
		ctx, cancel := context.WithTimeout(context.Background(), 80*time.Millisecond)
		defer cancel()
		start := time.Now()
		_, err := p.ParseContext(ctx, data, "test.pdf", ".pdf")
		if !errors.Is(err, context.DeadlineExceeded) || time.Since(start) > 3*time.Second {
			t.Fatalf("worker did not stop promptly: %v %s", err, time.Since(start))
		}
	})

	t.Run("code-cache-excludes-source-text", func(t *testing.T) {
		files := 0
		err := filepath.WalkDir(p.pdfCacheDir, func(path string, entry fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if entry.IsDir() {
				return nil
			}
			data, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			files++
			for _, marker := range []string{"The clinic opens at 08:30.", "Native page total 12", "Object stream page: total 23."} {
				if bytes.Contains(data, []byte(marker)) {
					t.Fatalf("document text retained in compiler cache")
				}
			}
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
		if files == 0 {
			t.Fatal("compiler cache was not exercised")
		}
	})
}
