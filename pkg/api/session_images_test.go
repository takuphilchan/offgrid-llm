package api

import (
	"bytes"
	"encoding/base64"
	"image"
	"image/jpeg"
	"image/png"
	"strings"
	"testing"
)

func TestSessionImagesValidatePixelsAndLimits(t *testing.T) {
	var b bytes.Buffer
	if err := png.Encode(&b, image.NewRGBA(image.Rect(0, 0, 2, 2))); err != nil {
		t.Fatal(err)
	}
	valid := SessionImage{Name: "test.png", DataURL: "data:image/png;base64," + base64.StdEncoding.EncodeToString(b.Bytes())}
	if err := ValidateSessionImages([]SessionImage{valid}); err != nil {
		t.Fatal(err)
	}
	var jpegBytes bytes.Buffer
	if err := jpeg.Encode(&jpegBytes, image.NewRGBA(image.Rect(0, 0, 2, 2)), nil); err != nil {
		t.Fatal(err)
	}
	if err := ValidateSessionImages([]SessionImage{{Name: "photo.jpg", DataURL: "data:image/jpeg;base64," + base64.StdEncoding.EncodeToString(jpegBytes.Bytes())}}); err != nil {
		t.Fatal("valid JPEG rejected", err)
	}
	for name, images := range map[string][]SessionImage{
		"too many":      {valid, valid, valid, valid, valid},
		"remote":        {{Name: "test.png", DataURL: "https://example.com/test.png"}},
		"file":          {{Name: "test.png", DataURL: "file:///private/test.png"}},
		"svg":           {{Name: "test.svg", DataURL: "data:image/svg+xml;base64,PHN2Zy8+"}},
		"spoofed":       {{Name: "test.jpg", DataURL: strings.Replace(valid.DataURL, "image/png", "image/jpeg", 1)}},
		"broken pixels": {{Name: "test.png", DataURL: "data:image/png;base64," + base64.StdEncoding.EncodeToString(b.Bytes()[:33])}},
		"oversized":     {{Name: "test.png", DataURL: valid.DataURL + strings.Repeat("A", 3<<20)}},
		"bad name":      {{Name: "bad\x00.png", DataURL: valid.DataURL}},
	} {
		t.Run(name, func(t *testing.T) {
			if ValidateSessionImages(images) == nil {
				t.Fatal("accepted invalid image")
			}
		})
	}
	content := SessionContent("describe", []SessionImage{valid})
	if present, err := ValidateChatMessages([]ChatMessage{{Role: "user", Content: content}}); err != nil || !present {
		t.Fatal(present, err)
	}
	if SessionContent("legacy", nil) != "legacy" {
		t.Fatal("text compatibility changed")
	}
}
