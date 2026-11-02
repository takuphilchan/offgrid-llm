package rag

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParserReadinessDoesNotConfusePresenceWithQualification(t *testing.T) {
	p := NewDocumentParser()
	p.pdfWorker = filepath.Join(t.TempDir(), "worker")
	check := func(want string) {
		t.Helper()
		formats := p.FormatReadiness()
		seen := map[string]bool{}
		for _, format := range formats {
			if format.Qualification != "unverified" || format.OCRAvailable || format.OCRLanguages == nil {
				t.Fatalf("invented qualification or OCR capability: %+v", format)
			}
			for _, ext := range format.Extensions {
				if seen[ext] {
					t.Fatalf("ambiguous format ownership: %s", ext)
				}
				seen[ext] = true
			}
			if format.ID == "pdf" && (format.RuntimeState != want || format.MaxFileBytes != 50<<20) {
				t.Fatalf("PDF readiness: %+v", format)
			}
		}
		data, err := json.Marshal(formats)
		if err != nil || strings.Contains(string(data), p.pdfWorker) {
			t.Fatal("readiness leaks a local runtime path")
		}
	}
	check("unavailable")
	// Deliberately not executable. Presence alone cannot become runtime-ready.
	if err := os.WriteFile(p.pdfWorker, []byte("not a worker"), 0600); err != nil {
		t.Fatal(err)
	}
	check("installed_unchecked")
	p.pdfWorker = t.TempDir()
	check("unavailable")
	p.maxFileSize = 1024
	for _, format := range p.FormatReadiness() {
		if format.MaxFileBytes > 1024 {
			t.Fatalf("readiness exceeds configured ceiling: %+v", format)
		}
	}
}

func TestParserUnsupportedContainersNeverBecomePlainText(t *testing.T) {
	for _, ext := range []string{".doc", ".xls", ".ppt", ".odt", ".ods", ".odp", ".eml", ".msg", ".mbox", ".png", ".jpg", ".jpeg", ".tif", ".tiff"} {
		result, err := NewDocumentParser().Parse([]byte("valid UTF-8 but not a supported document"), "unsupported"+ext, strings.ToUpper(ext))
		var problem *ParseError
		if result != nil || !errors.As(err, &problem) || problem.Code != "document_unsupported_format" {
			t.Fatalf("%s was silently treated as text: %+v %v", ext, result, err)
		}
	}
}

func TestParserSourceUnitsKeepOfficeOrderAndOriginalIdentity(t *testing.T) {
	entries, err := os.ReadDir("testdata/documents")
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		ext := filepath.Ext(entry.Name())
		if ext != ".xlsx" && ext != ".pptx" {
			continue
		}
		data, err := os.ReadFile(filepath.Join("testdata/documents", entry.Name()))
		if err != nil {
			t.Fatal(err)
		}
		result, err := NewDocumentParser().Parse(data, entry.Name(), ext)
		if err != nil {
			t.Fatal(err)
		}
		digest := sha256.Sum256(data)
		hash := hex.EncodeToString(digest[:])
		if len(result.SourceUnits) != result.PageCount {
			t.Fatalf("lost source units: %+v", result.SourceUnits)
		}
		if result.State != ExtractionComplete || len(result.Warnings) == 0 || result.Warnings[0].Code != "document_text_only" {
			t.Fatalf("missing extraction scope: %+v", result)
		}
		for i, unit := range result.SourceUnits {
			revision := builtinDocumentRevision
			if ext == ".xlsx" {
				revision = xlsxDocumentRevision
			}
			if unit.Index != i+1 || unit.SourceSHA256 != hash || unit.ExtractorRevision != revision {
				t.Fatalf("incorrect source identity: %+v", unit)
			}
			if unit.State == "ready" && !strings.Contains(result.Content, unit.Text) {
				t.Fatal("source text differs from extracted text")
			}
			if ext == ".xlsx" && (unit.Kind != "sheet" || unit.Name == "") {
				t.Fatalf("missing sheet identity: %+v", unit)
			}
			if ext == ".pptx" && unit.Kind != "slide" {
				t.Fatalf("invented slide location: %+v", unit)
			}
		}
	}
}

func TestParserTextSourceHasSectionNotInventedPage(t *testing.T) {
	source := []byte("Hello\nWorld\fStill not a rendered page")
	result, err := NewDocumentParser().Parse(source, "source.txt", ".txt")
	if err != nil {
		t.Fatal(err)
	}
	if len(result.SourceUnits) != 1 || result.SourceUnits[0].Kind != "section" || result.SourceUnits[0].Text != result.Content {
		t.Fatalf("invented page boundaries: %+v", result.SourceUnits)
	}
	changed, err := NewDocumentParser().Parse(append(source, '!'), "source.txt", ".txt")
	if err != nil || changed.SourceUnits[0].SourceSHA256 == result.SourceUnits[0].SourceSHA256 {
		t.Fatal("changed bytes reused old source identity")
	}
}
