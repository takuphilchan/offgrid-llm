package main

import (
	"os"
	"runtime"
	"strings"
)

// Terminal colors are deliberately semantic and monochrome. Normal terminal
// foreground/background colors remain authoritative, so the CLI is readable
// in dark, light, high-contrast, SSH, and redirected-output environments.
const (
	terminalLightText  = "#18181B"
	terminalDarkText   = "#F4F4F5"
	terminalLightMuted = "#52525B"
	terminalDarkMuted  = "#A1A1AA"
	terminalLightFaint = "#71717A"
	terminalDarkFaint  = "#71717A"
	terminalLightLine  = "#D4D4D8"
	terminalDarkLine   = "#3F3F46"
)

var (
	colorReset = "\033[0m"
	colorBold  = "\033[1m"
	colorDim   = "\033[2m"

	// Legacy semantic names remain aliases while command output is migrated to
	// higher-level renderers. None of them introduce a hue.
	colorCyan    = "\033[1m"
	colorGreen   = "\033[1m"
	colorYellow  = "\033[1m"
	colorRed     = "\033[1m"
	colorBlue    = "\033[1m"
	colorMagenta = "\033[1m"

	brandPrimary   = "\033[1m"
	brandSecondary = ""
	brandAccent    = "\033[1m"
	brandSuccess   = "\033[1m"
	brandError     = "\033[1m"
	brandMuted     = "\033[2m"
)

var (
	boxTL     = "+"
	boxTR     = "+"
	boxBL     = "+"
	boxBR     = "+"
	boxH      = "-"
	boxV      = "|"
	boxVR     = "+"
	boxVL     = "+"
	boxHD     = "+"
	boxHU     = "+"
	boxCross  = "+"
	separator = "-"

	iconBolt      = "*"
	iconCheck     = "OK"
	iconCross     = "ERR"
	iconArrow     = "->"
	iconDot       = "-"
	iconStar      = "*"
	iconBox       = "-"
	iconCircle    = "o"
	iconDiamond   = "*"
	iconChevron   = ">"
	iconDownload  = "down"
	iconUpload    = "up"
	iconSearch    = "search"
	iconModel     = "model"
	iconCpu       = "CPU"
	iconGpu       = "GPU"
	spinnerFrames = []string{"-", "\\", "|", "/"}
	progressFull  = "="
	progressEmpty = "-"
)

func init() {
	if !terminalSupportsColor() {
		disableColors()
	}
	if terminalSupportsUnicode() {
		enableUnicodeSymbols()
	}
}

func terminalSupportsColor() bool {
	if os.Getenv("NO_COLOR") != "" || os.Getenv("TERM") == "dumb" {
		return false
	}
	if os.Getenv("FORCE_COLOR") != "" {
		return true
	}
	info, err := os.Stdout.Stat()
	return err == nil && info.Mode()&os.ModeCharDevice != 0
}

func terminalSupportsUnicode() bool {
	if value, ok := os.LookupEnv("OFFGRID_UNICODE"); ok {
		return value == "1" || strings.EqualFold(value, "true")
	}
	info, err := os.Stdout.Stat()
	if err != nil || info.Mode()&os.ModeCharDevice == 0 || os.Getenv("TERM") == "dumb" {
		return false
	}
	if runtime.GOOS == "windows" {
		return os.Getenv("WT_SESSION") != "" || os.Getenv("TERM_PROGRAM") != ""
	}
	locale := os.Getenv("LC_ALL") + os.Getenv("LC_CTYPE") + os.Getenv("LANG")
	return strings.Contains(strings.ToUpper(locale), "UTF-8") || strings.Contains(strings.ToUpper(locale), "UTF8")
}

func enableUnicodeSymbols() {
	boxTL, boxTR, boxBL, boxBR = "╭", "╮", "╰", "╯"
	boxH, boxV, boxVR, boxVL = "─", "│", "├", "┤"
	boxHD, boxHU, boxCross, separator = "┬", "┴", "┼", "━"

	iconBolt, iconCheck, iconCross, iconArrow = "◆", "✓", "×", "→"
	iconDot, iconStar, iconBox, iconCircle = "•", "★", "▪", "●"
	iconDiamond, iconChevron = "◆", "›"
	iconDownload, iconUpload, iconSearch, iconModel = "↓", "↑", "⌕", "◇"
	iconCpu, iconGpu = "CPU", "GPU"
	spinnerFrames = []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}
	progressFull, progressEmpty = "█", "░"
}

func disableColors() {
	colorReset, colorBold, colorDim = "", "", ""
	colorCyan, colorGreen, colorYellow = "", "", ""
	colorRed, colorBlue, colorMagenta = "", "", ""
	brandPrimary, brandSecondary, brandAccent = "", "", ""
	brandSuccess, brandError, brandMuted = "", "", ""
}

func truncateTerminalText(value string, limit int) string {
	characters := []rune(value)
	if limit < 4 || len(characters) <= limit {
		return value
	}
	return string(characters[:limit-3]) + "..."
}
