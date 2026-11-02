package rag

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestOCRLanguageImportRejectsUnknownDataBeforeWriting(t *testing.T) {
	dir := t.TempDir()
	if got := OCRLanguages(dir); len(got) != 1 || got[0].State != "missing" {
		t.Fatal(got)
	}
	for _, language := range []string{"eng", "../eng", "deu"} {
		if err := ImportOCRLanguage(context.Background(), dir, language, strings.NewReader("not a verified pack")); err == nil {
			t.Fatal("unreviewed pack accepted")
		}
	}
	entries, _ := os.ReadDir(dir)
	if len(entries) != 0 {
		t.Fatal("rejected import changed storage")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := ImportOCRLanguage(ctx, dir, "eng", strings.NewReader("never read")); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(dir, ocrLanguageDirectory), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, ocrLanguageFile), []byte("corrupt"), 0600); err != nil {
		t.Fatal(err)
	}
	p := NewDocumentParserWithOCR(dir)
	if p.ocrSetupErr == nil || len(p.ocrLanguage) != 0 || OCRLanguages(dir)[0].State != "invalid" {
		t.Fatal("corrupt pack became ready")
	}
	if _, err := p.Parse([]byte("text still works"), "note.txt", ".txt"); err != nil {
		t.Fatal("OCR setup blocked text", err)
	}
}

func TestOCRLanguageActualOfflineImport(t *testing.T) {
	path := os.Getenv("OFFGRID_TEST_OCR_LANGUAGE")
	if path == "" {
		t.Skip("requires explicitly acquired pinned language fixture")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	for i := 0; i < 2; i++ {
		if err = ImportOCRLanguage(context.Background(), dir, "eng", bytes.NewReader(data)); err != nil {
			t.Fatal(err)
		}
		p := NewDocumentParserWithOCR(dir)
		if p.ocrSetupErr != nil || !bytes.Equal(p.ocrLanguage, data) || OCRLanguages(dir)[0].State != "installed_verified" {
			t.Fatal("offline import/reload failed", p.ocrSetupErr)
		}
		p.ocrWorker = filepath.Join(dir, ocrLanguageFile) // Presence is deliberately not execution qualification.
		for _, format := range p.FormatReadiness() {
			if format.ID == "pdf" && (format.OCRState != "installed_unchecked" || format.Qualification != "unverified" || len(format.OCRLanguages) != 1) {
				t.Fatal("language readiness lost or qualification invented", format)
			}
		}
	}
	entries, err := os.ReadDir(filepath.Join(dir, ocrLanguageDirectory))
	if err != nil || len(entries) != 1 {
		t.Fatal("staging leftovers", entries, err)
	}
	if err = ImportOCRLanguage(context.Background(), dir, "eng", strings.NewReader("invalid replacement")); err == nil {
		t.Fatal("bad replacement accepted")
	}
	if got, err := readOCRLanguage(dir); err != nil || !bytes.Equal(got, data) {
		t.Fatal("rejected replacement damaged installed pack", err)
	}
	// An import cannot escape the service-owned root even if its reserved
	// directory was replaced by a link. Windows may require link privilege.
	outside := t.TempDir()
	other := t.TempDir()
	if err = os.Symlink(outside, filepath.Join(other, ocrLanguageDirectory)); err == nil {
		if err = ImportOCRLanguage(context.Background(), other, "eng", bytes.NewReader(data)); err == nil {
			t.Fatal("linked language directory accepted")
		}
		entries, _ = os.ReadDir(outside)
		if len(entries) != 0 {
			t.Fatal("import escaped workspace")
		}
	} else {
		t.Log("symlink creation unavailable on this host; rooted path enforcement still active")
	}
}
