package rag

import (
	"bytes"
	"context"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/takuphilchan/offgrid-llm/internal/documentocr"
)

func TestMixedRealPackagedOCRPreservesBothScriptsAndPackIdentity(t *testing.T) {
	worker, root := os.Getenv("OFFGRID_TEST_OCR_WORKER"), os.Getenv("OFFGRID_TEST_OCR_PACKS")
	if worker == "" || root == "" {
		t.Skip("requires actual packaged worker and pinned packs")
	}
	var models [][]byte
	var sources []image.Image
	width, height := 0, 0
	dir := t.TempDir()
	for _, id := range []string{"chi_sim", "fra"} {
		model, err := os.ReadFile(filepath.Join(root, id+".traineddata"))
		if err != nil {
			t.Fatal(err)
		}
		models = append(models, model)
		raw, err := os.ReadFile(filepath.Join("testdata/documents/ocr/languages", id+".png"))
		if err != nil {
			t.Fatal(err)
		}
		bitmap, err := png.Decode(bytes.NewReader(raw))
		if err != nil {
			t.Fatal(err)
		}
		sources = append(sources, bitmap)
		width = max(width, bitmap.Bounds().Dx())
		height += bitmap.Bounds().Dy()
		if err = ImportOCRLanguage(context.Background(), dir, id, bytes.NewReader(model)); err != nil {
			t.Fatal(err)
		}
	}
	canvas := image.NewRGBA(image.Rect(0, 0, width, height))
	draw.Draw(canvas, canvas.Bounds(), image.NewUniform(color.White), image.Point{}, draw.Src)
	y := 0
	for _, part := range sources {
		draw.Draw(canvas, image.Rect(0, y, part.Bounds().Dx(), y+part.Bounds().Dy()), part, part.Bounds().Min, draw.Src)
		y += part.Bounds().Dy()
	}
	var encoded bytes.Buffer
	if err := png.Encode(&encoded, canvas); err != nil {
		t.Fatal(err)
	}
	p := NewDocumentParserForOCRLanguage(dir, "chi_sim+fra")
	p.ocrWorker = worker
	if p.ocrSetupErr != nil || !documentocr.ValidateModels("chi_sim+fra", p.ocrLanguage) {
		t.Fatal(p.ocrSetupErr)
	}
	r, err := p.ParseContext(context.Background(), encoded.Bytes(), "mixed.png", ".png")
	if err != nil || r == nil || r.Structure == nil {
		t.Fatal(r, err)
	}
	if !strings.Contains(strings.ReplaceAll(r.Content, " ", ""), "合同金额为42元") || !strings.Contains(r.Content, "Le montant total est de 42 euros") {
		t.Fatalf("mixed OCR lost source text: %q", r.Content)
	}
	if r.Metadata["ocr_language"] != "chi_sim+fra" || len(r.Structure.LanguagePacks) != 2 {
		t.Fatal("lost selection", r.Metadata, r.Structure.LanguagePacks)
	}
	for i, id := range []string{"chi_sim", "fra"} {
		pack, _ := documentocr.LookupLanguage(id)
		if r.Structure.LanguagePacks[i].SHA256 != pack.SHA256 || r.Structure.LanguagePacks[i].Name != "tesseract-"+id {
			t.Fatal("wrong pack identity")
		}
	}
	if err = r.Structure.Verify(); err != nil {
		t.Fatal(err)
	}
	assertOCRCitationPersistence(t, r)
	t.Logf("retained mixed script source: %q", r.Content)
	missing := NewDocumentParserForOCRLanguage(dir, "chi_sim+eng")
	if len(missing.ocrLanguage) != 0 {
		t.Fatal("silently used only the installed pack")
	}
}
