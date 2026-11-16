package rag

import (
	"bytes"
	"context"
	"encoding/binary"
	"github.com/takuphilchan/offgrid-llm/internal/testsupport/tifffixture"
	"image"
	"image/png"
	"os"
	"strings"
	"testing"
)

func TestTIFFRealPackagedOCR(t *testing.T) {
	worker, language, fixture := os.Getenv("OFFGRID_TEST_OCR_WORKER"), os.Getenv("OFFGRID_TEST_OCR_LANGUAGE"), os.Getenv("OFFGRID_TEST_IMAGE_DOCUMENT")
	if worker == "" || language == "" || fixture == "" {
		t.Skip("requires explicit local worker, language pack and printed-image fixture")
	}
	data, err := os.ReadFile(fixture)
	if err != nil {
		t.Fatal(err)
	}
	im, err := png.Decode(bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	source := tifffixture.Encode([]image.Image{im, im}, []uint16{1, 6}, binary.LittleEndian)
	p := NewDocumentParser()
	p.ocrWorker = worker
	pack, err := os.ReadFile(language)
	if err != nil {
		t.Fatal(err)
	}
	if err = p.UseOCRLanguage("eng", pack); err != nil {
		t.Fatal(err)
	}
	r, err := p.ParseContext(context.Background(), source, "record.tiff", ".tiff")
	if err != nil {
		t.Fatal(err)
	}
	if len(r.SourceUnits) != 2 || r.Structure == nil || r.Structure.Verify() != nil {
		t.Fatal(r)
	}
	for _, u := range r.SourceUnits {
		if !strings.Contains(u.Text, "Scanned receipt total 42") {
			t.Fatal(u)
		}
	}
	assertOCRCitationPersistence(t, r)
	// Reopening raster pages needs neither the language pack nor an OCR worker.
	p.ocrWorker = ""
	p.ocrLanguage = nil
	for i := 1; i <= 2; i++ {
		if _, err := p.RenderImagePage(context.Background(), source, ".tiff", i); err != nil {
			t.Fatal(err)
		}
	}
	t.Logf("actual TIFF OCR: %d original bytes, %d pages, %d line blocks", len(source), len(r.SourceUnits), len(r.Structure.Blocks))
}
