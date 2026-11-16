package imageinput

import (
	"image"
	"image/color"
	"testing"
)

func TestOriginalBoundsInvertsEveryExifPixel(t *testing.T) {
	source := image.NewNRGBA(image.Rect(0, 0, 7, 4))
	for y := 0; y < 4; y++ {
		for x := 0; x < 7; x++ {
			source.SetNRGBA(x, y, color.NRGBA{R: uint8(x), G: uint8(y), A: 255})
		}
	}
	for orientation := 1; orientation <= 8; orientation++ {
		upright, err := Orient(source, orientation)
		if err != nil {
			t.Fatal(err)
		}
		for y := 0; y < upright.Bounds().Dy(); y++ {
			for x := 0; x < upright.Bounds().Dx(); x++ {
				box, err := OriginalBounds([4]int{x, y, x + 1, y + 1}, 7, 4, orientation)
				if err != nil || box[2]-box[0] != 1 || box[3]-box[1] != 1 || source.At(box[0], box[1]) != upright.At(x, y) {
					t.Fatalf("orientation %d pixel %d,%d: %v %v", orientation, x, y, box, err)
				}
			}
		}
		full, err := OriginalBounds([4]int{0, 0, upright.Bounds().Dx(), upright.Bounds().Dy()}, 7, 4, orientation)
		if err != nil || full != [4]int{0, 0, 7, 4} {
			t.Fatal(full, err)
		}
	}
	for _, box := range [][4]int{{-1, 0, 1, 1}, {0, 0, 8, 1}, {0, 0, 0, 1}, {0, 0, 1, 5}} {
		if _, err := OriginalBounds(box, 7, 4, 1); err == nil {
			t.Fatal("invalid box accepted", box)
		}
	}
}
