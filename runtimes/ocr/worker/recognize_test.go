package main

import (
	"context"
	"fmt"
	"image"
	"testing"
)

func hocr(w, h, confidence int) string {
	return fmt.Sprintf(`<div class="ocr_page" title="bbox 0 0 %d %d"><span class="ocr_line"><span class="ocrx_word" title="bbox 0 0 1 1; x_wconf %d">text</span></span></div>`, w, h, confidence)
}

func TestOrientationRecoveryIsBoundedAndVisible(t *testing.T) {
	count := 0
	result, err := recognize(context.Background(), image.NewRGBA(image.Rect(0, 0, 3, 2)), func(pixels []byte, w, h int) (string, error) {
		count++
		if len(pixels) != w*h*4 {
			t.Fatal("invalid pixels")
		}
		confidence := 30
		if count == 2 {
			confidence = 96
		}
		return hocr(w, h, confidence), nil
	})
	if err != nil || count != 2 || result.attempts != 2 || result.rotation != 180 || result.state != "complete" {
		t.Fatal(result, count, err)
	}
}

func TestWeakOrMalformedResultsCannotBecomeComplete(t *testing.T) {
	count := 0
	result, err := recognize(context.Background(), image.NewRGBA(image.Rect(0, 0, 3, 2)), func(_ []byte, w, h int) (string, error) { count++; return hocr(w, h, 10*count), nil })
	if err != nil || count != 4 || result.state != "partial" || result.attempts != 4 || result.rotation != 270 {
		t.Fatal(result, count, err)
	}
	count = 0
	_, err = recognize(context.Background(), image.NewRGBA(image.Rect(0, 0, 3, 2)), func(_ []byte, w, h int) (string, error) { count++; return "malformed", nil })
	if err == nil || count != 1 {
		t.Fatal("malformed output retried", count, err)
	}
}

func TestOrientationCancellationStopsFurtherAttempts(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	count := 0
	_, err := recognize(ctx, image.NewRGBA(image.Rect(0, 0, 3, 2)), func(_ []byte, w, h int) (string, error) { count++; cancel(); return hocr(w, h, 20), nil })
	if err == nil || count != 1 {
		t.Fatal(count, err)
	}
}
