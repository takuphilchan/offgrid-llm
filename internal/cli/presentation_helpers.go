package cli

import (
	"fmt"
	"strings"
)

const (
	itemLabelWidth = 20
	optionWidth    = 26
)

func (p Presenter) HelpfulError(err error, context string) {
	p.Error(fmt.Sprintf("%s failed: %v", context, err))
	_, _ = fmt.Fprintln(p.Stdout)

	errStr := err.Error()
	switch {
	case strings.Contains(errStr, "connection refused"):
		p.Info("Possible causes:")
		p.lines("  • OffGrid server not running", "  • Port 11611 in use by another application", "")
		p.Info("Solutions:")
		p.lines("  • Start server: offgrid serve", "  • Check server: systemctl status llama-server", "  • Check port: lsof -i :11611")
	case strings.Contains(errStr, "timeout") || strings.Contains(errStr, "deadline exceeded"):
		p.Info("Connection timed out - possible causes:")
		p.lines("  • Slow internet connection", "  • HuggingFace servers overloaded", "  • Model is very large", "")
		p.Info("Solutions:")
		p.lines("  • Try again (temporary network issue)", "  • Check internet: ping huggingface.co", "  • Use smaller model")
	case strings.Contains(errStr, "no such host") || strings.Contains(errStr, "DNS"):
		p.Info("Cannot reach server - check internet connection:")
		p.lines("  • Test connection: ping 8.8.8.8", "  • Check DNS: ping huggingface.co", "  • Try offline mode: offgrid list (local models)")
	case strings.Contains(errStr, "permission denied"):
		p.Info("Permission problem:")
		p.lines("  • Check directory permissions", "  • Models directory: ~/.offgrid/models", "  • Fix: chmod 755 ~/.offgrid && chmod 755 ~/.offgrid/models")
	case strings.Contains(errStr, "no space left"):
		p.Info("Disk full:")
		p.lines("  • Check space: df -h", "  • Free space: delete old models with offgrid remove", "  • Models are typically 2-8GB each")
	case strings.Contains(errStr, "out of memory") || strings.Contains(errStr, "OOM"):
		p.Info("Not enough RAM:")
		p.lines("  • Check available RAM: offgrid info", "  • Use smaller model: offgrid search --ram 4", "  • Close other applications", "  • See: docs/4GB_RAM.md")
	case strings.Contains(errStr, "model not found") || strings.Contains(errStr, "404"):
		p.Info("Model not available:")
		p.lines("  • Check model name is correct", "  • Model may be private or removed", "  • Search for alternatives: offgrid search <query>")
	case strings.Contains(errStr, "rate limit") || strings.Contains(errStr, "429"):
		p.Info("HuggingFace rate limit reached:")
		p.lines("  • Wait a few minutes and try again", "  • Consider using local models: offgrid list", "  • Use fewer concurrent downloads")
	}
	_, _ = fmt.Fprintln(p.Stdout)
}

func (p Presenter) lines(lines ...string) {
	for _, line := range lines {
		_, _ = fmt.Fprintln(p.Stdout, line)
	}
}

func (p Presenter) Item(label, value string) {
	_, _ = fmt.Fprintf(p.Stdout, "  %s%-*s%s %s%s%s\n", p.Style.Muted, itemLabelWidth, label+":", p.Style.Reset, p.Style.Bold, value, p.Style.Reset)
}

func (p Presenter) Option(option, description string) {
	_, _ = fmt.Fprintf(p.Stdout, "  %s%-*s%s %s\n", p.Style.Secondary, optionWidth, option, p.Style.Reset, description)
}

func (p Presenter) Example(command string) {
	_, _ = fmt.Fprintf(p.Stdout, "  %s%s%s\n", p.Style.Dim, command, p.Style.Reset)
}

func (p Presenter) Divider() {
	_, _ = fmt.Fprintf(p.Stdout, "%s%s%s\n", p.Style.Muted, strings.Repeat(p.Style.Separator, 60), p.Style.Reset)
}

func (p Presenter) SectionHeader(title string) {
	if p.JSON {
		return
	}
	_, _ = fmt.Fprintf(p.Stdout, "\n%s%s %s%s\n\n", p.Style.Primary, p.Style.Chevron, title, p.Style.Reset)
}

func (p Presenter) ModernSection(title string, items [][]string) {
	_, _ = fmt.Fprintf(p.Stdout, "  %s%s%s\n", p.Style.Primary, title, p.Style.Reset)
	for _, item := range items {
		if len(item) >= 2 {
			_, _ = fmt.Fprintf(p.Stdout, "    %s%s%s %-22s %s%s%s\n", p.Style.Primary, p.Style.Chevron, p.Style.Reset, item[0], p.Style.Dim, item[1], p.Style.Reset)
		}
	}
	_, _ = fmt.Fprintln(p.Stdout)
}

func (p Presenter) stripStyle(value string) string {
	for _, code := range []string{p.Style.Reset, p.Style.Bold, p.Style.Dim, p.Style.Primary, p.Style.Secondary, p.Style.Accent, p.Style.Success, p.Style.Error, p.Style.Muted} {
		if code != "" {
			value = strings.ReplaceAll(value, code, "")
		}
	}
	return value
}

func (p Presenter) Box(title, content string) {
	const width = 58
	_, _ = fmt.Fprintf(p.Stdout, "%s%s%s%s%s\n", p.Style.Primary, p.Style.BoxTL, strings.Repeat(p.Style.BoxH, width), p.Style.BoxTR, p.Style.Reset)
	padding := (width - len(title) - 2) / 2
	_, _ = fmt.Fprintf(p.Stdout, "%s%s%s %s%s%s %s%s%s\n", p.Style.Primary, p.Style.BoxV, p.Style.Reset, strings.Repeat(" ", padding), p.Style.Bold+title+p.Style.Reset, strings.Repeat(" ", width-len(title)-padding-2), p.Style.Primary, p.Style.BoxV, p.Style.Reset)
	_, _ = fmt.Fprintf(p.Stdout, "%s%s%s%s%s\n", p.Style.Primary, p.Style.BoxVR, strings.Repeat(p.Style.BoxH, width), p.Style.BoxVL, p.Style.Reset)
	for _, line := range strings.Split(content, "\n") {
		contentPadding := width - len(p.stripStyle(line))
		if contentPadding < 2 {
			contentPadding = 2
		}
		_, _ = fmt.Fprintf(p.Stdout, "%s%s%s %s%s %s%s%s\n", p.Style.Primary, p.Style.BoxV, p.Style.Reset, line, strings.Repeat(" ", contentPadding-2), p.Style.Primary, p.Style.BoxV, p.Style.Reset)
	}
	_, _ = fmt.Fprintf(p.Stdout, "%s%s%s%s%s\n", p.Style.Primary, p.Style.BoxBL, strings.Repeat(p.Style.BoxH, width), p.Style.BoxBR, p.Style.Reset)
}

func (p Presenter) Card(title string, lines []string) {
	if p.JSON {
		return
	}
	const width = 56
	rightPad := width - len(p.stripStyle(title)) - 4
	if rightPad < 0 {
		rightPad = 0
	}
	_, _ = fmt.Fprintf(p.Stdout, "%s%s%s%s %s%s%s %s%s%s\n", p.Style.Muted, p.Style.BoxTL, strings.Repeat(p.Style.BoxH, 2), p.Style.Reset, p.Style.Bold+p.Style.Primary, title, p.Style.Reset, p.Style.Muted, strings.Repeat(p.Style.BoxH, rightPad)+p.Style.BoxTR, p.Style.Reset)
	for _, line := range lines {
		padding := width - len(p.stripStyle(line)) - 2
		if padding < 0 {
			padding = 0
		}
		_, _ = fmt.Fprintf(p.Stdout, "%s%s%s %s%s %s%s%s\n", p.Style.Muted, p.Style.BoxV, p.Style.Reset, line, strings.Repeat(" ", padding), p.Style.Muted, p.Style.BoxV, p.Style.Reset)
	}
	_, _ = fmt.Fprintf(p.Stdout, "%s%s%s%s%s\n", p.Style.Muted, p.Style.BoxBL, strings.Repeat(p.Style.BoxH, width), p.Style.BoxBR, p.Style.Reset)
}

func (p Presenter) SubtleDivider() {
	if p.JSON {
		return
	}
	_, _ = fmt.Fprintf(p.Stdout, "%s%s%s\n", p.Style.Muted, strings.Repeat(p.Style.BoxH, 50), p.Style.Reset)
}
