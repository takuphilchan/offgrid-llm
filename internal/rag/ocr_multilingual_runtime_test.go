package rag

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/takuphilchan/offgrid-llm/internal/documentocr"
)

func TestMultilingualRealPackagedOCRSourceIdentity(t *testing.T) {
	worker, packs := os.Getenv("OFFGRID_TEST_OCR_WORKER"), os.Getenv("OFFGRID_TEST_OCR_PACKS")
	if worker == "" || packs == "" {
		t.Skip("requires explicitly supplied worker and pinned packs")
	}
	for language, want := range map[string]string{"chi_sim": "合同金额为42元", "fra": "Le montant total est de 42 euros", "swa": "Malipo ya jumla ni 42"} {
		t.Run(language, func(t *testing.T) {
			p := NewDocumentParser()
			p.ocrWorker = worker
			model, err := os.ReadFile(filepath.Join(packs, language+".traineddata"))
			if err != nil {
				t.Fatal(err)
			}
			if err = p.UseOCRLanguage(language, model); err != nil {
				t.Fatal(err)
			}
			source, err := os.ReadFile(filepath.Join("testdata/documents/ocr/languages", language+".png"))
			if err != nil {
				t.Fatal(err)
			}
			r, err := p.ParseContext(context.Background(), source, language+".png", ".png")
			if err != nil || r == nil || r.Structure == nil {
				t.Fatal(r, err)
			}
			actual := r.Content
			if language == "chi_sim" {
				actual = strings.ReplaceAll(actual, " ", "")
			}
			if !strings.Contains(actual, want) {
				t.Fatalf("transcription mismatch: %q", r.Content)
			}
			pack, _ := documentocr.LookupLanguage(language)
			if r.Metadata["ocr_language"] != language || len(r.Structure.LanguagePacks) != 1 || r.Structure.LanguagePacks[0].Name != "tesseract-"+language || r.Structure.LanguagePacks[0].SHA256 != pack.SHA256 || r.Metadata["extractor_revision"] == imageDocumentRevision {
				t.Fatal("incorrect language provenance", r.Metadata, r.Structure.LanguagePacks)
			}
			if err = r.Structure.Verify(); err != nil {
				t.Fatal(err)
			}
			assertOCRCitationPersistence(t, r)
			t.Logf("retained %s: %q", language, r.Content)
		})
	}
}
