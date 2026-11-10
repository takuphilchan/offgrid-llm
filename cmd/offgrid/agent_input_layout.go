package main

import (
	"bufio"
	"strings"
	"unicode/utf8"

	"github.com/charmbracelet/x/ansi"
	"github.com/clipperhouse/uax29/v2/graphemes"
	"github.com/takuphilchan/offgrid-llm/internal/cli"
)

type agentInputPosition struct{ offset, row, column int }
type agentInputLayout struct {
	lines       []string
	positions   []agentInputPosition
	row, column int
}

// Layout measures complete graphemes in terminal cells. Leave the final screen
// column unused so a cursor at a wrap boundary never triggers terminal autowrap.
func (e *agentInputEditor) layout(width int) agentInputLayout {
	width = max(1, width)
	l := agentInputLayout{lines: []string{""}}
	offset, row, column := 0, 0, 0
	clusters := graphemes.FromString(string(e.text))
	for clusters.Next() {
		cluster := clusters.Value()
		display := strings.ReplaceAll(cli.TerminalSafe(cluster), "\t", " ")
		cells := ansi.StringWidth(display)
		newline := cluster == "\n" || cluster == "\r\n"
		if !newline && column > 0 && column+cells > width {
			l.lines = append(l.lines, "")
			row++
			column = 0
		}
		l.positions = append(l.positions, agentInputPosition{offset, row, column})
		if newline {
			l.lines = append(l.lines, "")
			row++
			column = 0
		} else {
			// An unusually narrow viewport cannot split a wide grapheme.
			if cells > width {
				display = "?"
				cells = 1
			}
			l.lines[row] += display
			column += cells
			if column >= width {
				l.lines = append(l.lines, "")
				row++
				column = 0
			}
		}
		offset += utf8.RuneCountInString(cluster)
	}
	l.positions = append(l.positions, agentInputPosition{offset, row, column})
	for _, p := range l.positions {
		if p.offset > e.cursor {
			break
		}
		l.row, l.column = p.row, p.column
	}
	return l
}

func (e *agentInputEditor) multilineView(width, height int) agentInputLayout {
	l := e.layout(width)
	height = max(1, height)
	start := max(0, l.row-height+1)
	end := min(len(l.lines), start+height)
	l.lines = l.lines[start:end]
	l.row -= start
	return l
}

// Kept for small selector tests that only need the caret's visible row.
func (e *agentInputEditor) view(width int) (string, int) {
	l := e.multilineView(width, 1)
	return l.lines[0], l.column
}

func (e *agentInputEditor) previousBoundary() int {
	previous := 0
	for _, p := range e.layout(max(1, e.width)).positions {
		if p.offset >= e.cursor {
			break
		}
		previous = p.offset
	}
	return previous
}
func (e *agentInputEditor) snapCursorForward() {
	for _, p := range e.layout(max(1, e.width)).positions {
		if p.offset >= e.cursor {
			e.cursor = p.offset
			return
		}
	}
}
func (e *agentInputEditor) nextBoundary() int {
	for _, p := range e.layout(max(1, e.width)).positions {
		if p.offset > e.cursor {
			return p.offset
		}
	}
	return len(e.text)
}
func (e *agentInputEditor) moveVertical(delta int) {
	l := e.layout(max(1, e.width))
	target := l.row + delta
	if target < 0 || target >= len(l.lines) {
		return
	}
	best := -1
	for _, p := range l.positions {
		if p.row == target && (best < 0 || p.column <= l.column) {
			best = p.offset
		}
	}
	if best >= 0 {
		e.cursor = best
	}
}
func (e *agentInputEditor) recall(delta int) {
	if len(e.history) == 0 {
		return
	}
	if e.historyIndex == len(e.history) {
		e.draft = string(e.text)
	}
	e.historyIndex = max(0, min(len(e.history), e.historyIndex+delta))
	text := e.draft
	if e.historyIndex < len(e.history) {
		text = e.history[e.historyIndex]
	}
	e.text = []rune(text)
	e.cursor = len(e.text)
	e.selected = 0
	e.dismissed = true
}

// Consume exactly one bracketed paste. Once full, retain only the terminator
// window, draining excess so it cannot become the next prompt/approval response.
func readAgentPaste(input *bufio.Reader, limit int) (string, bool, error) {
	const end = "\x1b[201~"
	var content strings.Builder
	pending := ""
	tooLarge := false
	for {
		b, err := input.ReadByte()
		if err != nil {
			return "", false, err
		}
		pending += string([]byte{b})
		if pending == end {
			break
		}
		for pending != "" && !strings.HasPrefix(end, pending) {
			if content.Len() < limit {
				content.WriteByte(pending[0])
			} else {
				tooLarge = true
			}
			pending = pending[1:]
		}
	}
	if tooLarge {
		return "", true, nil
	}
	return strings.ReplaceAll(strings.ReplaceAll(content.String(), "\r\n", "\n"), "\r", "\n"), false, nil
}
