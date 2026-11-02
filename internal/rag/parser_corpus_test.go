package rag

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

type parserCorpusCase struct {
	Path          string   `json:"path"`
	SHA256        string   `json:"sha256"`
	Bytes         int      `json:"bytes"`
	Family        string   `json:"family"`
	ExpectedText  []string `json:"expected_text"`
	SourceUnits   []string `json:"source_units"`
	ExpectedError string   `json:"expected_error"`
}

func readParserCorpus(t *testing.T) []parserCorpusCase {
	t.Helper()
	data, err := os.ReadFile("testdata/documents/corpus/manifest.json")
	if err != nil {
		t.Fatal(err)
	}
	var corpus struct {
		Version    int                `json:"schema_version"`
		License    string             `json:"license"`
		Provenance string             `json:"provenance"`
		Files      []parserCorpusCase `json:"files"`
	}
	if err := json.Unmarshal(data, &corpus); err != nil {
		t.Fatal(err)
	}
	if corpus.Version != 1 || len(corpus.Files) < 30 || corpus.License == "" || corpus.Provenance == "" {
		t.Fatal("corpus must contain at least 30 authored, licensed fixtures")
	}
	return corpus.Files
}

func TestParserCorpusIntegrity(t *testing.T) {
	seen := map[string]bool{}
	families := map[string]bool{}
	for _, fixture := range readParserCorpus(t) {
		if fixture.Path != filepath.Base(fixture.Path) || seen[fixture.Path] {
			t.Fatalf("unsafe or duplicate corpus path %q", fixture.Path)
		}
		seen[fixture.Path], families[fixture.Family] = true, true
		data, err := os.ReadFile(filepath.Join("testdata/documents/corpus", fixture.Path))
		if err != nil {
			t.Fatal(err)
		}
		digest := sha256.Sum256(data)
		if hex.EncodeToString(digest[:]) != fixture.SHA256 || len(data) != fixture.Bytes {
			t.Fatalf("fixture %s differs from its frozen manifest", fixture.Path)
		}
		if fixture.ExpectedError == "" && (len(fixture.ExpectedText) == 0 || len(fixture.SourceUnits) == 0) {
			t.Fatalf("fixture %s has no independent oracle", fixture.Path)
		}
	}
	for _, family := range []string{"pdf_text", "pdf_scan", "pdf_mixed", "pdf_encrypted", "corrupt", "resource", "office", "text"} {
		if !families[family] {
			t.Errorf("missing corpus family %s", family)
		}
	}
}

func TestParserRejectsInvalidUTF8WithoutReplacingBytes(t *testing.T) {
	data, err := os.ReadFile("testdata/documents/corpus/invalid-utf8.txt")
	if err != nil {
		t.Fatal(err)
	}
	for _, extension := range []string{".txt", ".md", ".markdown"} {
		result, err := NewDocumentParser().Parse(data, "invalid"+extension, extension)
		var problem *ParseError
		if result != nil || !errors.As(err, &problem) || problem.Code != "document_invalid" {
			t.Fatalf("%s accepted invalid text: result=%v error=%v", extension, result, err)
		}
	}
}

func TestParserFileIngestPreservesCancellation(t *testing.T) {
	path := filepath.Join(t.TempDir(), "cancelled.txt")
	if err := os.WriteFile(path, []byte("Never index this cancelled import"), 0600); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	// No store/embedder is configured: cancellation must stop before either is
	// touched, rather than being lost at the filesystem parser entry point.
	_, err := (&Engine{}).IngestFile(ctx, path, nil)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("cancelled file import continued: %v", err)
	}
}

// The opt-in report records known failures, rather than normalizing them into
// expected successful behavior. It is explicitly NOT a release qualification
// test. Strict mode is for a candidate claiming to satisfy the full corpus.
func TestParserCorpusScore(t *testing.T) {
	output := os.Getenv("OFFGRID_DOCUMENT_CORPUS_REPORT")
	if output == "" {
		t.Skip("set OFFGRID_DOCUMENT_CORPUS_REPORT to write an isolated corpus assessment")
	}
	// Score the service independently of developer-installed programs. The frozen
	// pre-worker baseline remains evidence; no PATH/regex fallback is now allowed.
	t.Setenv("PATH", t.TempDir())
	type score struct {
		Path              string `json:"path"`
		Family            string `json:"family"`
		Pass              bool   `json:"text_or_error_pass"`
		Error             string `json:"actual_error,omitempty"`
		Missing           string `json:"missing_text,omitempty"`
		PageCount         int    `json:"page_count,omitempty"`
		PageMatch         bool   `json:"page_count_match"`
		DurationMS        int64  `json:"duration_ms"`
		SourceRangesValid bool   `json:"source_ranges_valid"`
	}
	report := struct {
		Engine               string  `json:"engine"`
		Passed               int     `json:"text_or_error_passed"`
		Total                int     `json:"total"`
		SourceUnitsQualified bool    `json:"source_units_qualified"`
		Cases                []score `json:"cases"`
	}{Engine: "current-parser-without-PATH-extractor"}
	parser := NewDocumentParser()
	if worker := os.Getenv("OFFGRID_TEST_DOCUMENT_WORKER"); worker != "" {
		if !filepath.IsAbs(worker) {
			t.Fatal("test worker must have an absolute path")
		}
		parser.pdfWorker, parser.pdfCacheDir = worker, t.TempDir()
		report.Engine = "builtin-with-pinned-pdfium-worker-no-OCR"
	}
	for _, fixture := range readParserCorpus(t) {
		data, err := os.ReadFile(filepath.Join("testdata/documents/corpus", fixture.Path))
		if err != nil {
			t.Fatal(err)
		}
		start := time.Now()
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		result, err := parser.ParseContext(ctx, data, fixture.Path, filepath.Ext(fixture.Path))
		cancel()
		s := score{Path: fixture.Path, Family: fixture.Family, DurationMS: time.Since(start).Milliseconds()}
		if err != nil {
			var problem *ParseError
			if errors.As(err, &problem) {
				s.Error = problem.Code
			} else {
				s.Error = "unclassified_error"
			}
		}
		if fixture.ExpectedError != "" {
			s.Pass = s.Error == fixture.ExpectedError
		} else if result != nil && err == nil {
			s.Pass = true
			remaining := strings.Join(strings.Fields(result.Content), " ")
			for _, expected := range fixture.ExpectedText {
				expected = strings.Join(strings.Fields(expected), " ")
				index := strings.Index(remaining, expected)
				if index < 0 {
					s.Pass, s.Missing = false, expected
					break
				}
				remaining = remaining[index+len(expected):]
			}
		}
		if result != nil {
			s.PageCount = result.PageCount
			s.PageMatch = s.PageCount == len(fixture.SourceUnits)
			if result.State == ExtractionComplete && err == nil {
				_, _, rangeErr := parsedSourceRanges(result)
				s.SourceRangesValid = rangeErr == nil
				if rangeErr != nil {
					s.Pass = false
				}
			}
		}
		if s.Pass {
			report.Passed++
		} else {
			t.Logf("UNQUALIFIED %s: error=%s missing=%q", s.Path, s.Error, s.Missing)
		}
		report.Cases = append(report.Cases, s)
	}
	report.Total = len(report.Cases)
	encoded, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(output, append(encoded, '\n'), 0600); err != nil {
		t.Fatal(err)
	}
	t.Logf("text/error matches: %d/%d; source-unit contract not qualified", report.Passed, report.Total)
	if os.Getenv("OFFGRID_DOCUMENT_CORPUS_STRICT") == "1" && report.Passed != report.Total {
		t.Fatal("parser has unresolved corpus failures")
	}
}
