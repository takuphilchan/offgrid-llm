package main

import (
	"fmt"
	"io"
	"math"
	"os"
	"strings"

	"github.com/charmbracelet/x/term"
	"github.com/takuphilchan/offgrid-llm/internal/output"
)

// Download progress belongs to a real terminal. Redirected output retains a
// single completion line, with no carriage returns or animation control bytes.
func printProgressBar(current, total int64, speed float64, width int) {
	if output.JSONMode {
		return
	}
	animated := term.IsTerminal(os.Stdout.Fd()) && os.Getenv("TERM") != "dumb"
	columns, _, err := term.GetSize(os.Stdout.Fd())
	if err != nil {
		columns = 80
	}
	renderTransferProgress(os.Stdout, current, total, speed, width, animated, columns)
}

func renderTransferProgress(w io.Writer, current, total int64, speed float64, width int, animated bool, columns ...int) {
	if total <= 0 {
		return
	}
	current = max(0, min(current, total))
	width = max(1, min(width, 60))
	if !animated {
		if current == total {
			fmt.Fprintf(w, "  Transferred %s\n", formatBytes(total))
		}
		return
	}
	if math.IsNaN(speed) || math.IsInf(speed, 0) || speed < 0 {
		speed = 0
	}
	fraction := float64(current) / float64(total)
	eta := ""
	if speed > 0 && current < total {
		seconds := float64(total-current) / speed
		if seconds < 3600 {
			eta = fmt.Sprintf(" | ~%.0fs remaining", seconds)
		}
	}
	limit := 79 // Leave the final column unused to avoid terminal auto-wrapping.
	if len(columns) > 0 {
		limit = max(0, columns[0]-1)
	}
	percent := fmt.Sprintf("%.1f%%", fraction*100)
	sizes := fmt.Sprintf("%s / %s", formatBytes(current), formatBytes(total))
	suffix := fmt.Sprintf(" %s | %s | %.1f MB/s%s", percent, sizes, speed/(1024*1024), eta)
	width = min(width, limit-len(suffix)-4)
	line := ""
	if width >= 8 {
		filled := min(width, int(float64(width)*fraction))
		line = "  [" + strings.Repeat("=", filled) + strings.Repeat(" ", width-filled) + "]" + suffix
	} else {
		for _, compact := range []string{percent + " | " + sizes, percent + " | " + formatBytes(total), percent} {
			if len(compact) <= limit {
				line = compact
				break
			}
		}
	}
	fmt.Fprintf(w, "\r\x1b[2K%s", line)
}
