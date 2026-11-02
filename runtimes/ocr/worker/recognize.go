package main

import (
	"context"
	"fmt"
	"image"
	"time"

	"github.com/takuphilchan/offgrid-llm/internal/documentocr"
)

type recognition struct {
	hocr, state        string
	rotation, attempts int
}

// Retry only perception, never a document mutation. Try at most four reviewed
// orientations under the same deadline; uncertain content remains partial.
func recognize(ctx context.Context, bitmap image.Image, run func([]byte, int, int) (string, error)) (recognition, error) {
	best := recognition{state: "partial"}
	bestScore := -1
	for _, angle := range []int{0, 180, 90, 270} {
		started := time.Now()
		pixels, w, h, err := documentocr.Pixels(ctx, bitmap, angle)
		if err != nil {
			return recognition{}, err
		}
		text, err := run(pixels, w, h)
		if err != nil {
			return recognition{}, err
		}
		if ctx.Err() != nil {
			return recognition{}, ctx.Err()
		}
		words, err := documentocr.ParseHOCR(text, w, h)
		if err != nil {
			return recognition{}, fmt.Errorf("invalid OCR result")
		}
		score, reliable := documentocr.Quality(words)
		best.attempts++
		if score > bestScore {
			best.hocr, best.rotation, bestScore = text, angle, score
		}
		if reliable {
			best.hocr, best.rotation, best.state = text, angle, "complete"
			return best, nil
		}
		// Preserve a validated partial result rather than starting another pass
		// that the measured previous pass says is unlikely to fit the deadline.
		// This never turns weak evidence into a complete extraction.
		if deadline, ok := ctx.Deadline(); ok && time.Until(deadline) < time.Since(started)+time.Second {
			return best, nil
		}
	}
	return best, nil
}
