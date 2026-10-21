package main

import (
	"fmt"
	"github.com/takuphilchan/offgrid-llm/internal/cli"
	"github.com/takuphilchan/offgrid-llm/internal/output"
	"os"
	"strings"
	"time"
)

func printBanner() {
	legacyPresenter().Banner(getVersion())
}

func printSection(title string) {
	legacyPresenter().Section(title)
}

func printSuccess(message string) {
	legacyPresenter().Success(message)
}

func printError(message string) {
	legacyPresenter().Error(message)
}

func printInfo(message string) {
	legacyPresenter().Info(message)
}

func printWarning(message string) {
	legacyPresenter().Warning(message)
}

func legacyPresenter() cli.Presenter {
	return cli.Presenter{
		Stdout: os.Stdout,
		Stderr: os.Stderr,
		JSON:   output.JSONMode,
		Style: cli.PresentationStyle{
			Primary: brandPrimary, Secondary: brandSecondary, Success: brandSuccess, Error: brandError,
			Accent: brandAccent, Muted: brandMuted, Bold: colorBold, Dim: colorDim,
			Reset: colorReset, Check: iconCheck, Cross: iconCross, Bolt: iconBolt,
			Chevron: iconChevron, Separator: separator,
			Search: iconSearch, Star: iconStar, Circle: iconCircle,
			BoxTL: boxTL, BoxTR: boxTR, BoxBL: boxBL, BoxBR: boxBR,
			BoxH: boxH, BoxV: boxV, BoxVR: boxVR, BoxVL: boxVL,
		},
	}
}

// printProgressBar renders a clean progress bar for downloads
// Format: [=========>          ] 45.2% | 1.8 GB / 4.0 GB | 12.3 MB/s | ETA 2m30s
func printHelpfulError(err error, context string) {
	legacyPresenter().HelpfulError(err, context)
}

// Consistent formatting constants for terminal output
const (
	labelWidth   = 20 // Width for labels in key-value pairs
	optionWidth  = 26 // Width for option/command names
	exampleWidth = 34 // Width for example commands
	tableCol1    = 40 // Width for first table column (model names)
	tableCol2    = 12 // Width for second table column (size)
	tableCol3    = 12 // Width for third table column (quant)
)

func printItem(label, value string) {
	legacyPresenter().Item(label, value)
}

func printOption(option, description string) {
	legacyPresenter().Option(option, description)
}

func printExample(cmd string) {
	legacyPresenter().Example(cmd)
}

func printDivider() {
	legacyPresenter().Divider()
}

func printBox(title, content string) {
	legacyPresenter().Box(title, content)
}

// printModernSection prints a command section with aligned columns
func printModernSection(title string, items [][]string) {
	legacyPresenter().ModernSection(title, items)
}

// ensureOffgridServerRunning checks if OffGrid server is responding
func printSectionHeader(title string) {
	legacyPresenter().SectionHeader(title)
}

// Print a key-value pair with modern styling
func printKeyValue(key, value string) {
	legacyPresenter().KeyValue(key, value)
}

// Print a card-style box with content
func printCard(title string, lines []string) {
	legacyPresenter().Card(title, lines)
}

// Print a subtle divider line
func printSubtleDivider() {
	legacyPresenter().SubtleDivider()
}

func printHelp() { legacyPresenter().Help(getVersion()) }

func formatNumber(n int64) string {
	if n >= 1000000 {
		return fmt.Sprintf("%.1fM", float64(n)/1000000)
	} else if n >= 1000 {
		return fmt.Sprintf("%.1fK", float64(n)/1000)
	}
	return fmt.Sprintf("%d", n)
}

// estimateRAMFromModel estimates RAM needed based on model name and quantization
type diskSpaceInfo struct {
	Total       int64
	Available   int64
	UsedPercent float64
}

func formatBytes(bytes int64) string {
	return cli.FormatBytes(bytes)
}

func formatSizeGB(gb float64) string {
	if gb < 1.0 {
		return fmt.Sprintf("%.0f MB", gb*1024)
	}
	return fmt.Sprintf("%.1f GB", gb)
}

// wrapText wraps text to a specified width
func wrapText(text string, width int) string {
	if len(text) <= width {
		return text
	}
	var lines []string
	words := strings.Fields(text)
	var line string
	for _, word := range words {
		if len(line)+len(word)+1 > width {
			if line != "" {
				lines = append(lines, line)
			}
			line = word
		} else {
			if line != "" {
				line += " "
			}
			line += word
		}
	}
	if line != "" {
		lines = append(lines, line)
	}
	return strings.Join(lines, "\n")
}

// formatDuration formats a duration in human-readable form
func formatDuration(d time.Duration) string {
	if d < time.Microsecond {
		return fmt.Sprintf("%dns", d.Nanoseconds())
	} else if d < time.Millisecond {
		return fmt.Sprintf("%.2fµs", float64(d.Microseconds()))
	} else if d < time.Second {
		return fmt.Sprintf("%.2fms", float64(d.Milliseconds()))
	}
	return fmt.Sprintf("%.2fs", d.Seconds())
}

// formatModelSize formats model size in GB with appropriate precision
func formatModelSize(sizeGB float64) string {
	if sizeGB < 0.1 {
		// For sizes less than 0.1 GB, show in MB
		return fmt.Sprintf("%d MB", int(sizeGB*1024))
	} else if sizeGB < 1.0 {
		// For sizes less than 1 GB, use 2 decimal places
		return fmt.Sprintf("%.2f GB", sizeGB)
	}
	// For larger sizes, use 1 decimal place
	return fmt.Sprintf("%.1f GB", sizeGB)
}

// isServerHealthy checks if the server is responding
func formatTimeAgo(t time.Time) string {
	duration := time.Since(t)

	if duration < time.Minute {
		return "just now"
	} else if duration < time.Hour {
		minutes := int(duration.Minutes())
		if minutes == 1 {
			return "1 minute ago"
		}
		return fmt.Sprintf("%d minutes ago", minutes)
	} else if duration < 24*time.Hour {
		hours := int(duration.Hours())
		if hours == 1 {
			return "1 hour ago"
		}
		return fmt.Sprintf("%d hours ago", hours)
	} else if duration < 7*24*time.Hour {
		days := int(duration.Hours() / 24)
		if days == 1 {
			return "1 day ago"
		}
		return fmt.Sprintf("%d days ago", days)
	} else {
		return t.Format("2006-01-02")
	}
}

// handleCompletions generates shell completion scripts
func handleShellCompletion(args []string) {
	if len(args) < 1 {
		fmt.Println()
		fmt.Printf("%sShell Completions%s\n", brandPrimary+colorBold, colorReset)
		fmt.Println("")
		fmt.Printf("%sUsage:%s offgrid shell-completion <shell>\n", colorDim, colorReset)
		fmt.Println("")
		fmt.Printf("%sSupported shells:%s bash, zsh, fish\n", colorDim, colorReset)
		fmt.Println()
		fmt.Printf("%sInstallation%s\n", brandPrimary+colorBold, colorReset)
		fmt.Println()
		fmt.Println("  Bash:")
		fmt.Printf("   %s$ offgrid shell-completion bash > /etc/bash_completion.d/offgrid%s\n", colorDim, colorReset)
		fmt.Println()
		fmt.Println("  Zsh:")
		fmt.Printf("   %s$ offgrid shell-completion zsh > ~/.zsh/completions/_offgrid%s\n", colorDim, colorReset)
		fmt.Println()
		fmt.Println("  Fish:")
		fmt.Printf("   %s$ offgrid shell-completion fish > ~/.config/fish/completions/offgrid.fish%s\n", colorDim, colorReset)
		fmt.Println()
		return
	}

	shell := strings.ToLower(args[0])

	// Call the existing completions handler
	handleCompletions([]string{shell})
}
