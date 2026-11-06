package cli

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/x/ansi"
)

func (p Presenter) HelpfulError(err error, context string) {
	p.Error(terminalLine(fmt.Sprintf("%s failed: %v", context, err)))
	_, _ = fmt.Fprintln(p.Stdout)

	errStr := err.Error()
	switch {
	case strings.Contains(errStr, "connection refused"):
		p.Note("The configured endpoint did not accept a connection.")
		p.Note("Run offgrid doctor to check the service address and readiness. Open OffGrid Desktop, or start the intended service with offgrid serve.")
	case strings.Contains(errStr, "timeout") || strings.Contains(errStr, "deadline exceeded"):
		p.Note("The operation did not finish within its deadline. A timeout alone does not identify the cause.")
		p.Note("Check offgrid doctor and offgrid logs. Inspect saved work before retrying an operation that may have changed files or external state.")
	case strings.Contains(errStr, "no such host") || strings.Contains(errStr, "DNS"):
		p.Note("The endpoint name could not be resolved. Check the configured address and its network connection.")
	case strings.Contains(errStr, "permission denied"):
		p.Note("Check access to the reported file or folder. offgrid info shows the locally configured model folder; a remote service may use a different folder.")
	case strings.Contains(errStr, "no space left"):
		p.Note("Check free space on the destination drive before continuing. Existing models are listed by offgrid list.")
	case strings.Contains(errStr, "out of memory") || strings.Contains(errStr, "OOM"):
		p.Note("Check runtime readiness with offgrid doctor. Close unused applications or explicitly choose a smaller supported context or model.")
	case strings.Contains(errStr, "model not found") || strings.Contains(errStr, "404"):
		p.Note("The requested resource was not found. For a model request, check its exact ID with offgrid list.")
	case strings.Contains(errStr, "rate limit") || strings.Contains(errStr, "429"):
		p.Note("The endpoint is limiting requests. Follow its retry delay and avoid starting duplicate requests.")
	}
	_, _ = fmt.Fprintln(p.Stdout)
}

func (p Presenter) lines(lines ...string) {
	for _, line := range lines {
		_, _ = fmt.Fprintln(p.Stdout, line)
	}
}

func (p Presenter) Item(label, value string) {
	p.Detail(label, value)
}

func (p Presenter) Option(option, description string) {
	p.rows([]presentationRow{{option, description}})
}

func (p Presenter) Example(command string) {
	if !p.JSON {
		// Preserve a copyable command; the terminal may soft-wrap it.
		_, _ = fmt.Fprintf(p.Stdout, "  %s\n", terminalLine(command))
	}
}

func (p Presenter) Divider() {
	if !p.JSON {
		_, _ = fmt.Fprintf(p.Stdout, "%s%s%s\n", p.Style.Muted, strings.Repeat(p.Style.Separator, max(1, min(60, p.columns()-2))), p.Style.Reset)
	}
}

func (p Presenter) SectionHeader(title string) {
	if p.JSON {
		return
	}
	p.Heading(title)
}

func (p Presenter) ModernSection(title string, items [][]string) {
	p.Heading(title)
	rows := make([]presentationRow, 0, len(items))
	for _, item := range items {
		if len(item) >= 2 {
			rows = append(rows, presentationRow{item[0], item[1]})
		}
	}
	p.rows(rows)
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
	if p.JSON {
		return
	}
	width := max(8, min(58, p.columns()-2))
	_, _ = fmt.Fprintf(p.Stdout, "%s%s%s%s%s\n", p.Style.Primary, p.Style.BoxTL, strings.Repeat(p.Style.BoxH, width), p.Style.BoxTR, p.Style.Reset)
	for _, line := range strings.Split(ansi.Wrap(terminalLine(title), width-2, ""), "\n") {
		textWidth := ansi.StringWidth(line)
		padding := max(0, (width-textWidth-2)/2)
		_, _ = fmt.Fprintf(p.Stdout, "%s%s%s %s%s%s %s%s%s\n", p.Style.Primary, p.Style.BoxV, p.Style.Reset, strings.Repeat(" ", padding), p.Style.Bold+line+p.Style.Reset, strings.Repeat(" ", max(0, width-textWidth-padding-2)), p.Style.Primary, p.Style.BoxV, p.Style.Reset)
	}
	_, _ = fmt.Fprintf(p.Stdout, "%s%s%s%s%s\n", p.Style.Primary, p.Style.BoxVR, strings.Repeat(p.Style.BoxH, width), p.Style.BoxVL, p.Style.Reset)
	for _, line := range strings.Split(ansi.Wrap(content, width-2, ""), "\n") {
		contentPadding := width - ansi.StringWidth(p.stripStyle(line))
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
	if p.columns() < width+2 || ansi.StringWidth(p.stripStyle(title))+6 > width {
		p.Box(title, strings.Join(lines, "\n"))
		return
	}
	for _, line := range lines {
		if ansi.StringWidth(p.stripStyle(line)) > width-2 {
			p.Box(title, strings.Join(lines, "\n"))
			return
		}
	}
	rightPad := width - ansi.StringWidth(p.stripStyle(title)) - 4
	if rightPad < 0 {
		rightPad = 0
	}
	_, _ = fmt.Fprintf(p.Stdout, "%s%s%s%s %s%s%s %s%s%s\n", p.Style.Muted, p.Style.BoxTL, strings.Repeat(p.Style.BoxH, 2), p.Style.Reset, p.Style.Bold+p.Style.Primary, title, p.Style.Reset, p.Style.Muted, strings.Repeat(p.Style.BoxH, rightPad)+p.Style.BoxTR, p.Style.Reset)
	for _, line := range lines {
		padding := width - ansi.StringWidth(p.stripStyle(line)) - 2
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
	_, _ = fmt.Fprintf(p.Stdout, "%s%s%s\n", p.Style.Muted, strings.Repeat(p.Style.BoxH, max(1, min(50, p.columns()-2))), p.Style.Reset)
}
