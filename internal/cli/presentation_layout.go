package cli

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/x/ansi"
)

// Human output shares one hierarchy. Identifiers are never abbreviated or
// rewritten: when a row cannot fit, its detail moves below the exact identifier.
// Very long identifiers soft-wrap in the terminal, keeping them copyable.
func (p Presenter) columns() int {
	if p.Style.Columns > 0 {
		return p.Style.Columns
	}
	return 80
}

func terminalLine(value string) string {
	return strings.NewReplacer("\n", " ", "\t", " ").Replace(TerminalSafe(value))
}

func (p Presenter) Heading(title string) {
	if !p.JSON {
		title = strings.TrimPrefix(title, "OffGrid / ")
		fmt.Fprintf(p.Stdout, "\n  %s%s%s\n", p.Style.Bold, terminalLine(title), p.Style.Reset)
	}
}

func (p Presenter) Note(text string) {
	if p.JSON {
		return
	}
	text = ansi.Wordwrap(terminalLine(text), max(8, p.columns()-4), "")
	fmt.Fprintf(p.Stdout, "  %s\n", strings.ReplaceAll(text, "\n", "\n  "))
}

// Detail gives values the terminal's normal contrast, rather than dimming the
// information users came to read. Labels share a stable, compact column.
func (p Presenter) Detail(label, value string) {
	if p.JSON {
		return
	}
	label, value = terminalLine(label), terminalLine(value)
	width := max(14, ansi.StringWidth(label))
	if width+ansi.StringWidth(value)+6 <= p.columns() {
		fmt.Fprintf(p.Stdout, "  %s%s  %s\n", label, strings.Repeat(" ", width-ansi.StringWidth(label)), value)
	} else {
		fmt.Fprintf(p.Stdout, "  %s\n    %s\n", label, value)
	}
}

type presentationRow struct{ Name, Detail string }

func formatCount(n int) string {
	s := fmt.Sprint(n)
	for i := len(s) - 3; i > 0; i -= 3 {
		s = s[:i] + "," + s[i:]
	}
	return s
}

func (p Presenter) rows(rows []presentationRow) {
	p.rowsWidth(rows, 0)
}

func (p Presenter) rowsWidth(rows []presentationRow, minimumWidth int) {
	if p.JSON {
		return
	}
	width, detailWidth := minimumWidth, 0
	for _, row := range rows {
		width = max(width, ansi.StringWidth(terminalLine(row.Name)))
		detailWidth = max(detailWidth, ansi.StringWidth(terminalLine(row.Detail)))
	}
	stack := width+detailWidth+6 > p.columns()
	for _, row := range rows {
		name, detail := terminalLine(row.Name), terminalLine(row.Detail)
		if stack {
			fmt.Fprintf(p.Stdout, "\n  %s%s%s\n", p.Style.Bold, name, p.Style.Reset)
			if detail != "" {
				detail = ansi.Wordwrap(detail, max(8, p.columns()-6), "")
				fmt.Fprintf(p.Stdout, "    %s\n", strings.ReplaceAll(detail, "\n", "\n    "))
			}
		} else {
			fmt.Fprintf(p.Stdout, "  %s%s%s%s  %s\n", p.Style.Bold, name, p.Style.Reset, strings.Repeat(" ", width-ansi.StringWidth(name)), detail)
		}
	}
}
