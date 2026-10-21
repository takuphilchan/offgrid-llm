package main

import (
	"fmt"
	"github.com/takuphilchan/offgrid-llm/internal/output"
	"os"
	"strings"
	"time"
)

func printProgressBar(current, total int64, speed float64, width int) {
	if total == 0 {
		return
	}
	percent := float64(current) / float64(total) * 100
	filled := int(float64(width) * percent / 100)
	if filled > width {
		filled = width
	}

	// Build progress bar
	bar := strings.Repeat("=", filled)
	if filled < width {
		bar += ">"
		bar += strings.Repeat(" ", width-filled-1)
	}

	// Calculate ETA
	eta := ""
	if speed > 0 && current < total {
		remaining := float64(total-current) / speed
		if remaining < 60 {
			eta = fmt.Sprintf("%ds", int(remaining))
		} else if remaining < 3600 {
			mins := int(remaining) / 60
			secs := int(remaining) % 60
			eta = fmt.Sprintf("%dm%ds", mins, secs)
		} else {
			hours := int(remaining) / 3600
			mins := (int(remaining) % 3600) / 60
			eta = fmt.Sprintf("%dh%dm", hours, mins)
		}
	} else if current >= total {
		eta = "done"
	}

	// Format sizes
	currentStr := formatBytes(current)
	totalStr := formatBytes(total)
	speedStr := fmt.Sprintf("%.1f MB/s", speed/(1024*1024))

	// Print the line (carriage return to overwrite)
	fmt.Printf("\r  [%s%s%s] %s%.1f%%%s | %s / %s | %s | %s      ",
		brandPrimary, bar, colorReset,
		colorBold, percent, colorReset,
		currentStr, totalStr,
		speedStr, eta)
	os.Stdout.Sync()
}

func stripAnsi(str string) string {
	// Simple ANSI strip for length calculation
	result := str
	for _, code := range []string{colorReset, colorBold, colorDim, colorCyan, colorGreen, colorYellow, colorRed, colorBlue, colorMagenta, brandPrimary, brandSecondary, brandAccent, brandSuccess, brandError, brandMuted} {
		result = strings.ReplaceAll(result, code, "")
	}
	return result
}

// Spinner represents an animated loading indicator
type Spinner struct {
	message string
	frame   int
	running bool
	done    chan bool
}

// NewSpinner creates a new spinner with a message
func NewSpinner(message string) *Spinner {
	return &Spinner{
		message: message,
		frame:   0,
		running: false,
		done:    make(chan bool),
	}
}

// Start begins the spinner animation
func (s *Spinner) Start() {
	if output.JSONMode {
		return
	}
	s.running = true
	go func() {
		for s.running {
			fmt.Printf("\r%s%s%s %s", brandPrimary, spinnerFrames[s.frame], colorReset, s.message)
			s.frame = (s.frame + 1) % len(spinnerFrames)
			time.Sleep(80 * time.Millisecond)
		}
		s.done <- true
	}()
}

// Stop ends the spinner with a final status
func (s *Spinner) Stop(success bool) {
	if output.JSONMode {
		return
	}
	s.running = false
	<-s.done
	if success {
		fmt.Printf("\r%s%s%s %s\n", brandSuccess, iconCheck, colorReset, s.message)
	} else {
		fmt.Printf("\r%s%s%s %s\n", brandError, iconCross, colorReset, s.message)
	}
}

// StopWithMessage ends the spinner with a custom message
func (s *Spinner) StopWithMessage(success bool, message string) {
	if output.JSONMode {
		return
	}
	s.running = false
	<-s.done
	if success {
		fmt.Printf("\r%s%s%s %s\n", brandSuccess, iconCheck, colorReset, message)
	} else {
		fmt.Printf("\r%s%s%s %s\n", brandError, iconCross, colorReset, message)
	}
}

// UpdateMessage changes the spinner message while running
func (s *Spinner) UpdateMessage(message string) {
	s.message = message
}

// ProgressBar renders a visual progress bar
type ProgressBar struct {
	total     int64
	current   int64
	width     int
	label     string
	startTime time.Time
}

// NewProgressBar creates a new progress bar
func NewProgressBar(total int64, label string) *ProgressBar {
	return &ProgressBar{
		total:     total,
		current:   0,
		width:     30,
		label:     label,
		startTime: time.Now(),
	}
}

// Update updates the progress bar with current progress
func (p *ProgressBar) Update(current int64) {
	if output.JSONMode {
		return
	}
	p.current = current
	percent := float64(current) / float64(p.total) * 100
	filled := int(float64(p.width) * percent / 100)
	empty := p.width - filled

	// Calculate speed and ETA
	elapsed := time.Since(p.startTime).Seconds()
	speed := float64(current) / elapsed / (1024 * 1024) // MB/s

	var eta string
	if speed > 0 {
		remaining := float64(p.total-current) / (speed * 1024 * 1024)
		if remaining < 60 {
			eta = fmt.Sprintf("%.0fs", remaining)
		} else {
			eta = fmt.Sprintf("%.1fm", remaining/60)
		}
	}

	// Build progress bar
	bar := strings.Repeat(progressFull, filled) + strings.Repeat(progressEmpty, empty)

	// Render
	fmt.Printf("\r  %s%s%s %s%s%s %s%.1f%%%s %s%.1f MB/s%s %s%s%s",
		brandPrimary, bar, colorReset,
		colorBold, p.label, colorReset,
		brandMuted, percent, colorReset,
		brandMuted, speed, colorReset,
		brandMuted, eta, colorReset)
}

// Complete marks the progress as done
func (p *ProgressBar) Complete() {
	if output.JSONMode {
		return
	}
	bar := strings.Repeat(progressFull, p.width)
	fmt.Printf("\r  %s%s%s %s%s%s %s100.0%%%s\n", brandPrimary, bar, colorReset, colorBold, p.label, colorReset, brandSuccess, colorReset)
}

// Modern section headers with subtle styling
