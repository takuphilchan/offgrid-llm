package rag

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"hash/crc32"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"os"
	"strings"
	"testing"

	"github.com/takuphilchan/offgrid-llm/internal/documentocr"
)

func imageDocumentFixture(t *testing.T, format string) []byte {
	t.Helper()
	pixels := image.NewNRGBA(image.Rect(0, 0, 120, 80))
	for y := 0; y < 80; y++ {
		for x := 0; x < 120; x++ {
			pixels.SetNRGBA(x, y, color.NRGBA{R: 240, G: 240, B: 240, A: 255})
		}
	}
	var out bytes.Buffer
	var err error
	if format == "jpeg" {
		err = jpeg.Encode(&out, pixels, nil)
	} else {
		err = png.Encode(&out, pixels)
	}
	if err != nil {
		t.Fatal(err)
	}
	return out.Bytes()
}

func TestParserImageDocumentRejectsInvalidPixelsBeforeOCR(t *testing.T) {
	p := NewDocumentParser()
	calls := 0
	run := func(context.Context, *pdfPageRaster) (*ocrPageResult, error) {
		calls++
		t.Fatal("invalid image dispatched OCR")
		return nil, nil
	}
	pngData := imageDocumentFixture(t, "png")
	oversized := bytes.Clone(pngData)
	binary.BigEndian.PutUint32(oversized[16:20], 8193)
	binary.BigEndian.PutUint32(oversized[29:33], crc32.ChecksumIEEE(oversized[12:29]))
	for _, tc := range []struct {
		data []byte
		ext  string
	}{{[]byte("text masquerading as an image"), ".png"}, {pngData[:33], ".png"}, {pngData, ".jpg"}, {imageDocumentFixture(t, "jpeg")[:30], ".jpeg"}, {oversized, ".png"}, {make([]byte, imageDocumentMaxBytes+1), ".png"}} {
		result, err := p.extractImageDocument(context.Background(), tc.data, tc.ext, true, run)
		if err == nil || result != nil {
			t.Fatal("invalid pixels accepted", result, err)
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := p.extractImageDocument(ctx, pngData, ".png", true, run); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	if calls != 0 {
		t.Fatal(calls)
	}
}

func imageDocumentPNGChunk(data []byte, kind string, payload []byte) []byte {
	chunk := make([]byte, 12+len(payload))
	binary.BigEndian.PutUint32(chunk, uint32(len(payload)))
	copy(chunk[4:8], kind)
	copy(chunk[8:], payload)
	binary.BigEndian.PutUint32(chunk[8+len(payload):], crc32.ChecksumIEEE(chunk[4:8+len(payload)]))
	out := bytes.Clone(data[:33])
	out = append(out, chunk...)
	return append(out, data[33:]...)
}

func TestParserImageDocumentExifAndTransparentRaster(t *testing.T) {
	source := imageDocumentFixture(t, "png")
	for orientation := 1; orientation <= 8; orientation++ {
		exif := []byte{'I', 'I', 42, 0, 8, 0, 0, 0, 1, 0, 0x12, 1, 3, 0, 1, 0, 0, 0, byte(orientation), 0, 0, 0, 0, 0, 0, 0}
		data := imageDocumentPNGChunk(source, "eXIf", exif)
		page, err := decodeImageDocument(context.Background(), data, ".png")
		if err != nil || page.orientation != orientation || page.width != 120 || page.height != 80 {
			t.Fatal(page, err)
		}
		w, h := 120, 80
		if orientation >= 5 {
			w, h = h, w
		}
		if page.raster.Width != w || page.raster.Height != h {
			t.Fatal("EXIF not applied", orientation, page.raster)
		}
	}
	animated := imageDocumentPNGChunk(source, "acTL", make([]byte, 8))
	if _, err := decodeImageDocument(context.Background(), animated, ".png"); err == nil {
		t.Fatal("animation silently flattened")
	}
	pixels := image.NewNRGBA(image.Rect(0, 0, 2, 1))
	pixels.SetNRGBA(0, 0, color.NRGBA{R: 255, A: 0})
	pixels.SetNRGBA(1, 0, color.NRGBA{B: 255, A: 128})
	var encoded bytes.Buffer
	if err := png.Encode(&encoded, pixels); err != nil {
		t.Fatal(err)
	}
	page, err := decodeImageDocument(context.Background(), encoded.Bytes(), ".png")
	if err != nil {
		t.Fatal(err)
	}
	raster, err := png.Decode(bytes.NewReader(page.raster.PNG))
	if err != nil {
		t.Fatal(err)
	}
	if color.NRGBAModel.Convert(raster.At(0, 0)).(color.NRGBA) != (color.NRGBA{255, 255, 255, 255}) || color.NRGBAModel.Convert(raster.At(1, 0)).(color.NRGBA) != (color.NRGBA{127, 127, 255, 255}) {
		t.Fatal("OCR and preview alpha composition differ")
	}
}

func TestParserImageDocumentCoverageAndSourceGeometry(t *testing.T) {
	p := NewDocumentParser()
	source := imageDocumentFixture(t, "png")
	before := bytes.Clone(source)
	words := []documentocr.Word{{Text: "Grant", Bounds: [4]int{1, 2, 30, 12}, Line: 1, Confidence: 95}, {Text: "42", Bounds: [4]int{31, 2, 45, 12}, Line: 1, Confidence: 96}, {Text: "Except", Bounds: [4]int{1, 20, 40, 30}, Line: 2, Confidence: 93}}
	run := func(_ context.Context, r *pdfPageRaster) (*ocrPageResult, error) {
		if r.Width != 120 || r.Height != 80 {
			t.Fatal(r)
		}
		return &ocrPageResult{Words: words, Text: "Grant 42\nExcept", State: "complete", Attempts: 1}, nil
	}
	result, err := p.extractImageDocument(context.Background(), source, ".png", true, run)
	if err != nil || result.Structure == nil || result.Structure.Verify() != nil {
		t.Fatal(result, err)
	}
	if !bytes.Equal(source, before) || result.Structure.OriginalSHA256 != ocrDigest(source) || result.Structure.Method != "ocr" || result.Coverage().Status != "text_only" || len(result.Coverage().ReviewUnits) != 1 {
		t.Fatal("lost image identity or visual limitations", result)
	}
	blocks := result.Structure.Blocks
	if len(blocks) != 2 || blocks[0].Bounds.Width != 44 || blocks[0].Span.End != 8 || blocks[1].Span.Start != 9 || blocks[0].SourcePart != "original_encoded_pixels" {
		t.Fatal(blocks)
	}
	if err = result.bindStructure(); err != nil {
		t.Fatal(err)
	}
	partial, err := p.extractImageDocument(context.Background(), source, ".png", true, func(context.Context, *pdfPageRaster) (*ocrPageResult, error) {
		return &ocrPageResult{Words: words, Text: "Grant 42\nExcept", State: "partial", Attempts: 4}, nil
	})
	if err == nil || partial.State != ExtractionPartial || partial.SourceUnits[0].State != "ocr_partial" || partial.Content == "" || partial.Structure != nil {
		t.Fatal(partial, err)
	}
	assertPartialOCRReviewPersistence(t, source, partial)
	if _, _, err = parsedSourceRanges(partial); err == nil {
		t.Fatal("partial image became complete indexable text")
	}
}

func TestParserImageDocumentMissingPackAndPreview(t *testing.T) {
	p := NewDocumentParser()
	p.ocrWorker = "must-not-execute"
	source := imageDocumentFixture(t, "png")
	result, err := p.ParseContext(context.Background(), source, "scan.png", ".png")
	var problem *ParseError
	if !errors.As(err, &problem) || problem.Code != "document_ocr_language_missing" || result == nil || result.State != ExtractionPartial {
		t.Fatal(result, err)
	}
	preview, err := p.PreviewContext(context.Background(), source, "scan.png", ".png")
	if !errors.As(err, &problem) || problem.Code != "document_ocr_required" || preview.Content != "" {
		t.Fatal(preview, err)
	}
	p.ocrWorker = ""
	result, err = p.ParseContext(context.Background(), source, "scan.png", ".png")
	if !errors.As(err, &problem) || problem.Code != "document_ocr_unavailable" || result == nil {
		t.Fatal(result, err)
	}
}

func TestParserImageDocumentRealOCR(t *testing.T) {
	worker, language, fixture := os.Getenv("OFFGRID_TEST_OCR_WORKER"), os.Getenv("OFFGRID_TEST_OCR_LANGUAGE"), os.Getenv("OFFGRID_TEST_IMAGE_DOCUMENT")
	if worker == "" || language == "" || fixture == "" {
		t.Skip("requires explicitly supplied packaged worker, verified pack and scan fixture")
	}
	p := NewDocumentParser()
	p.ocrWorker = worker
	model, err := os.ReadFile(language)
	if err != nil {
		t.Fatal(err)
	}
	if err = p.UseOCRLanguage("eng", model); err != nil {
		t.Fatal(err)
	}
	source, err := os.ReadFile(fixture)
	if err != nil {
		t.Fatal(err)
	}
	for _, format := range []string{"png", "jpeg"} {
		t.Run(format, func(t *testing.T) {
			data := source
			if format == "jpeg" {
				pixels, err := png.Decode(bytes.NewReader(source))
				if err != nil {
					t.Fatal(err)
				}
				var encoded bytes.Buffer
				if err = jpeg.Encode(&encoded, pixels, &jpeg.Options{Quality: 95}); err != nil {
					t.Fatal(err)
				}
				data = encoded.Bytes()
			}
			result, err := p.ParseContext(context.Background(), data, "scan."+format, "."+format)
			if err != nil || result.State != ExtractionComplete || !strings.Contains(result.Content, "Scanned receipt total 42") || result.Structure.Method != "ocr" {
				t.Fatal(result, err)
			}
			if err = result.Structure.Verify(); err != nil {
				t.Fatal(err)
			}
			assertOCRCitationPersistence(t, result)
		})
	}
}
