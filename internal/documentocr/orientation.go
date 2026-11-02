package documentocr

import (
	"context"
	"fmt"
	"image"
	"image/draw"
)

// Pixels returns opaque RGBA rotated clockwise by a reviewed right angle.
// Its dimensions and orientation must accompany the receipt: rotated word
// coordinates cannot be presented as coordinates on the original page.
func Pixels(ctx context.Context, source image.Image, rotation int) ([]byte, int, int, error) {
	if source == nil || rotation != 0 && rotation != 90 && rotation != 180 && rotation != 270 {
		return nil, 0, 0, fmt.Errorf("invalid OCR orientation")
	}
	bounds := source.Bounds()
	w, h := bounds.Dx(), bounds.Dy()
	if w < 1 || h < 1 || w > 8192 || h > 8192 || int64(w)*int64(h) > 16_000_000 {
		return nil, 0, 0, fmt.Errorf("OCR image exceeds limits")
	}
	if err := ctx.Err(); err != nil {
		return nil, 0, 0, err
	}
	// Normalize once using image/draw's pixel paths. Calling Image.At and color
	// conversion per pixel creates millions of short-lived interface allocations
	// for ordinary scanned pages and makes cancellation/latency unnecessarily bad.
	straight, ok := source.(*image.NRGBA)
	if !ok {
		straight = image.NewNRGBA(image.Rect(0, 0, w, h))
		draw.Draw(straight, straight.Bounds(), source, bounds.Min, draw.Src)
	}
	outW, outH := w, h
	if rotation == 90 || rotation == 270 {
		outW, outH = h, w
	}
	pixels := make([]byte, outW*outH*4)
	for y := 0; y < h; y++ {
		if err := ctx.Err(); err != nil {
			return nil, 0, 0, err
		}
		for x := 0; x < w; x++ {
			dx, dy := x, y
			switch rotation {
			case 90:
				dx, dy = h-1-y, x
			case 180:
				dx, dy = w-1-x, h-1-y
			case 270:
				dx, dy = y, w-1-x
			}
			sourceIndex := straight.PixOffset(straight.Rect.Min.X+x, straight.Rect.Min.Y+y)
			c := straight.Pix[sourceIndex : sourceIndex+4]
			i, alpha := (dy*outW+dx)*4, uint32(c[3])
			pixels[i] = byte((uint32(c[0])*alpha + 255*(255-alpha) + 127) / 255)
			pixels[i+1] = byte((uint32(c[1])*alpha + 255*(255-alpha) + 127) / 255)
			pixels[i+2] = byte((uint32(c[2])*alpha + 255*(255-alpha) + 127) / 255)
			pixels[i+3] = 255
		}
	}
	return pixels, outW, outH, nil
}

// OriginalBounds maps half-open OCR boxes back to the source raster. The
// caller must first validate hOCR against the rotated image dimensions.
func OriginalBounds(box [4]int, width, height, rotation int) ([4]int, error) {
	w, h := width, height
	if rotation == 90 || rotation == 270 {
		w, h = h, w
	}
	if width < 1 || height < 1 || width > 8192 || height > 8192 ||
		box[0] < 0 || box[1] < 0 || box[2] <= box[0] || box[3] <= box[1] || box[2] > w || box[3] > h {
		return [4]int{}, fmt.Errorf("invalid OCR bounds")
	}
	switch rotation {
	case 0:
		return box, nil
	case 90:
		return [4]int{box[1], height - box[2], box[3], height - box[0]}, nil
	case 180:
		return [4]int{width - box[2], height - box[3], width - box[0], height - box[1]}, nil
	case 270:
		return [4]int{width - box[3], box[0], width - box[1], box[2]}, nil
	default:
		return [4]int{}, fmt.Errorf("invalid OCR orientation")
	}
}

// Quality is a conservative engine-confidence check, not a correctness score.
// Any weak word remains partial, even when other words are highly confident.
func Quality(words []Word) (score int, reliable bool) {
	if len(words) == 0 {
		return 0, false
	}
	total, weight := 0, 0
	reliable = true
	for _, word := range words {
		n := len([]rune(word.Text))
		if n == 0 || word.Confidence < 0 || word.Confidence > 100 {
			return 0, false
		}
		total += word.Confidence * n
		weight += n
		if word.Confidence < 70 {
			reliable = false
		}
	}
	score = total / weight
	return score, reliable && score >= 85
}
