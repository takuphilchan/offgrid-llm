package main

import (
	"bytes"
	"math"
	"strings"
	"testing"
)

func TestTransferProgressPlainAndInvalidMeasurements(t *testing.T) {
	var b bytes.Buffer
	renderTransferProgress(&b, 50, 100, 10, 30, false)
	if b.Len() != 0 {
		t.Fatalf("plain intermediate output: %q", b.String())
	}
	renderTransferProgress(&b, 100, 100, 10, 30, false)
	if !strings.Contains(b.String(), "Transferred") || strings.ContainsAny(b.String(), "\r\x1b") {
		t.Fatalf("plain output: %q", b.String())
	}
	for _, test := range []struct {
		current, total int64
		width          int
		speed          float64
	}{
		{-1, 100, -5, -1}, {200, 100, 99999, math.NaN()}, {1, 0, 0, 0}, {1, -1, 10, math.Inf(1)},
	} {
		b.Reset()
		renderTransferProgress(&b, test.current, test.total, test.speed, test.width, true)
		if strings.Contains(b.String(), "NaN") || strings.Contains(b.String(), "Inf") || b.Len() > 200 {
			t.Fatalf("invalid progress: %q", b.String())
		}
	}
}

func TestTransferProgressFitsTerminalWidth(t *testing.T) {
	for _, columns := range []int{1, 8, 20, 40, 60, 80, 120} {
		var b bytes.Buffer
		renderTransferProgress(&b, 123456789, 987654321, 1234567, 60, true, columns)
		line := strings.TrimPrefix(b.String(), "\r\x1b[2K")
		if len(line) >= columns {
			t.Fatalf("%d columns: overflowing %q", columns, line)
		}
		if columns >= 8 && !strings.Contains(line, "%") {
			t.Fatalf("missing progress: %q", line)
		}
	}
}
