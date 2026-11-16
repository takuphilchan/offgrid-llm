package imageinput

import (
	"context"
	"image"
	"image/color"
)

// Orient preserves decoded pixel values, including alpha and 16-bit channels.
// It never recompresses JPEG or replaces the caller's original encoded bytes.
func Orient(source image.Image, orientation int) (image.Image, error) {
	return OrientContext(context.Background(), source, orientation)
}

// OrientContext allows document cancellation between pixel rows.
func OrientContext(ctx context.Context, source image.Image, orientation int) (image.Image, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if source == nil || orientation < 1 || orientation > 8 {
		return nil, ErrMetadata
	}
	bounds := source.Bounds()
	w, h := bounds.Dx(), bounds.Dy()
	if w < 1 || h < 1 || w > 8192 || h > 8192 || int64(w)*int64(h) > 32<<20 {
		return nil, ErrMetadata
	}
	if orientation == 1 {
		return source, nil
	}
	ow, oh := w, h
	if orientation >= 5 {
		ow, oh = h, w
	}
	// Callers must bound dimensions before decoding. Preserve the decoded color
	// model rather than converting 16-bit PNGs to 8-bit as an unnoticed side effect.
	var output interface {
		image.Image
		Set(int, int, color.Color)
	}
	switch source.(type) {
	case *image.NRGBA64:
		output = image.NewNRGBA64(image.Rect(0, 0, ow, oh))
	case *image.RGBA64:
		output = image.NewRGBA64(image.Rect(0, 0, ow, oh))
	case *image.Gray16:
		output = image.NewGray16(image.Rect(0, 0, ow, oh))
	case *image.RGBA:
		output = image.NewRGBA(image.Rect(0, 0, ow, oh))
	default:
		output = image.NewNRGBA(image.Rect(0, 0, ow, oh))
	}
	for y := 0; y < h; y++ {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		for x := 0; x < w; x++ {
			dx, dy := x, y
			switch orientation {
			case 2:
				dx = w - 1 - x
			case 3:
				dx, dy = w-1-x, h-1-y
			case 4:
				dy = h - 1 - y
			case 5:
				dx, dy = y, x
			case 6:
				dx, dy = h-1-y, x
			case 7:
				dx, dy = h-1-y, w-1-x
			case 8:
				dx, dy = y, w-1-x
			}
			output.Set(dx, dy, source.At(bounds.Min.X+x, bounds.Min.Y+y))
		}
	}
	return output, nil
}
