package documentocr

import (
	"context"
	"image"
	"image/color"
	"testing"
)

func TestPixelsAndBoxesPreserveOriginalPositions(t *testing.T) {
	// Nonzero origin and nonsquare dimensions catch transposition/off-by-one bugs.
	source := image.NewNRGBA(image.Rect(7, 9, 10, 11))
	for y := 0; y < 2; y++ {
		for x := 0; x < 3; x++ {
			source.SetNRGBA(7+x, 9+y, color.NRGBA{R: byte(y*3 + x + 1), A: 255})
		}
	}
	for _, row := range []struct {
		angle, w, h int
		want        []byte
	}{
		{0, 3, 2, []byte{1, 2, 3, 4, 5, 6}}, {90, 2, 3, []byte{4, 1, 5, 2, 6, 3}},
		{180, 3, 2, []byte{6, 5, 4, 3, 2, 1}}, {270, 2, 3, []byte{3, 6, 2, 5, 1, 4}},
	} {
		pixels, w, h, err := Pixels(context.Background(), source, row.angle)
		if err != nil || w != row.w || h != row.h {
			t.Fatal(row.angle, w, h, err)
		}
		for i, value := range row.want {
			if pixels[i*4] != value || pixels[i*4+3] != 255 {
				t.Fatal("wrong pixel", row.angle, i, pixels[i*4:i*4+4])
			}
			x, y := i%w, i/w
			box, err := OriginalBounds([4]int{x, y, x + 1, y + 1}, 3, 2, row.angle)
			originalX, originalY := int(value-1)%3, int(value-1)/3
			if err != nil || box != [4]int{originalX, originalY, originalX + 1, originalY + 1} {
				t.Fatal("wrong source locator", row.angle, box, err)
			}
		}
	}
}

func TestPixelsOpacityCancellationAndLimits(t *testing.T) {
	source := image.NewNRGBA(image.Rect(0, 0, 1, 1))
	source.SetNRGBA(0, 0, color.NRGBA{R: 255, A: 128})
	pixels, _, _, err := Pixels(context.Background(), source, 0)
	if err != nil || pixels[0] != 255 || pixels[1] != 127 || pixels[2] != 127 || pixels[3] != 255 {
		t.Fatal(pixels, err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if data, _, _, err := Pixels(ctx, source, 0); err == nil || data != nil {
		t.Fatal("cancelled rotation ran")
	}
	if _, _, _, err := Pixels(context.Background(), source, 45); err == nil {
		t.Fatal("unbounded orientation")
	}
	if _, err := OriginalBounds([4]int{0, 0, 2, 1}, 1, 1, 0); err == nil {
		t.Fatal("out of bounds location")
	}
	if _, err := OriginalBounds([4]int{0, 0, 1, 1}, 1, 1, 45); err == nil {
		t.Fatal("invalid rotation")
	}
}

func TestLowConfidenceIsNotHiddenByStrongWords(t *testing.T) {
	if _, ok := Quality(nil); ok {
		t.Fatal("blank page reliable")
	}
	words := []Word{{Text: "A very long confident sentence", Confidence: 99}, {Text: "3", Confidence: 20}}
	if score, ok := Quality(words); score < 85 || ok {
		t.Fatal("weak amount was concealed", score, ok)
	}
	if _, ok := Quality([]Word{{Text: "café 日本語", Confidence: 96}}); !ok {
		t.Fatal("unicode weighting failed")
	}
}
