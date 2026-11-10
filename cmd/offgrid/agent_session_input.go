package main

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"strings"
	"unicode"

	"github.com/charmbracelet/x/ansi"
	"github.com/charmbracelet/x/term"
	"github.com/takuphilchan/offgrid-llm/internal/cli"
)

// This editor reads synchronously from the SAME buffered input as local consent.
// No background reader can eat a grant response or a later prompt. Raw mode is
// restored before returning to approvals, streaming, or process shutdown.
func clearAgentDisplay(out io.Writer) bool {
	file, ok := cli.AgentOutputTarget(out).(*os.File)
	if !ok || !term.IsTerminal(file.Fd()) || os.Getenv("TERM") == "dumb" || os.Getenv("OFFGRID_PLAIN") == "1" || os.Getenv("OFFGRID_TUI") == "0" {
		return false
	}
	_, err := fmt.Fprint(out, "\x1b[3J\x1b[2J\x1b[H")
	return err == nil
}

func readAgentSessionLine(input *bufio.Reader, out io.Writer, style cli.PresentationStyle, commands []cli.AgentCommandSuggestion) (string, error) {
	return readAgentSessionLineWithHistory(input, out, style, commands, nil)
}

func newAgentSessionInput() func(*bufio.Reader, io.Writer, cli.PresentationStyle, []cli.AgentCommandSuggestion) (string, error) {
	line, _ := newAgentSessionEditors()
	return line
}

func newAgentSessionEditors() (func(*bufio.Reader, io.Writer, cli.PresentationStyle, []cli.AgentCommandSuggestion) (string, error), func(*bufio.Reader, io.Writer, cli.PresentationStyle, []cli.AgentCommandSuggestion, string) (string, error)) {
	var history []string
	read := func(input *bufio.Reader, out io.Writer, style cli.PresentationStyle, commands []cli.AgentCommandSuggestion, initial string) (string, error) {
		line, err := readAgentSessionDraft(input, out, style, commands, history, initial)
		if err == nil && strings.TrimSpace(line) != "" {
			history = append(history, strings.TrimSuffix(line, "\n"))
			if len(history) > 50 {
				history = history[len(history)-50:]
			}
		}
		return line, err
	}
	return func(input *bufio.Reader, out io.Writer, style cli.PresentationStyle, commands []cli.AgentCommandSuggestion) (string, error) {
		return read(input, out, style, commands, "")
	}, read
}

func readAgentSessionLineWithHistory(input *bufio.Reader, out io.Writer, style cli.PresentationStyle, commands []cli.AgentCommandSuggestion, history []string) (string, error) {
	return readAgentSessionDraft(input, out, style, commands, history, "")
}

func readAgentSessionDraft(input *bufio.Reader, out io.Writer, style cli.PresentationStyle, commands []cli.AgentCommandSuggestion, history []string, initial string) (string, error) {
	fallback := func() (string, error) {
		if initial != "" {
			fmt.Fprintf(out, "Retained draft:\n%s\nEnter to send it, or type a replacement.\n", cli.TerminalSafe(initial))
		}
		line, err := cli.ReadAgentPlainLine(input, out, style)
		if err == nil && strings.TrimSpace(line) == "" && initial != "" {
			return initial, nil
		}
		return line, err
	}
	file, ok := cli.AgentOutputTarget(out).(*os.File)
	if !ok || !term.IsTerminal(os.Stdin.Fd()) || !term.IsTerminal(file.Fd()) || os.Getenv("TERM") == "dumb" || os.Getenv("OFFGRID_PLAIN") == "1" || os.Getenv("OFFGRID_TUI") == "0" {
		return fallback()
	}
	width, height, err := term.GetSize(file.Fd())
	if err != nil || width < 32 || height < 10 {
		return fallback()
	}
	state, err := term.MakeRaw(os.Stdin.Fd())
	if err != nil {
		return fallback()
	}
	defer term.Restore(os.Stdin.Fd(), state)
	fmt.Fprint(out, "\r\n\x1b[?2004h")
	defer fmt.Fprint(out, "\x1b[?2004l")
	editor := agentInputEditor{commands: commands, history: history, historyIndex: len(history)}
	editor.insert(initial)
	rows, caretRow := 1, 0
	draw := func(final bool) {
		// Re-measure between keys so a resized terminal cannot retain stale width.
		if w, h, e := term.GetSize(file.Fd()); e == nil && w > 4 {
			width, height = w, h
		}
		if caretRow > 0 {
			fmt.Fprintf(out, "\x1b[%dA", caretRow)
		}
		fmt.Fprint(out, "\r\x1b[2K")
		for i := 1; i < rows; i++ {
			fmt.Fprint(out, "\r\n\x1b[2K")
		}
		if rows > 1 {
			fmt.Fprintf(out, "\x1b[%dA\r", rows-1)
		}
		editor.width = max(1, width-4)
		view := editor.multilineView(editor.width, max(1, min(6, height-4)))
		for i, line := range view.lines {
			if i > 0 {
				fmt.Fprint(out, "\r\n")
			}
			prefix := "  "
			if i == 0 {
				prefix = "> "
			}
			fmt.Fprintf(out, "%s%s%s%s", style.Primary, prefix, style.Reset, line)
		}
		rows = len(view.lines)
		if !final && editor.menuOpen() {
			for _, row := range agentCommandMenu(editor.matches(), editor.selected, width, height-rows) {
				color := style.Muted
				if row.selected {
					color = style.Primary + style.Bold + style.Selected
				}
				fmt.Fprintf(out, "\r\n%s%s%s", color, row.text, style.Reset)
				rows++
			}
		} else if !final {
			hint := "Enter send · Ctrl+J newline"
			if editor.searchActive {
				hint = "History: " + cli.TerminalSafe(editor.searchQuery) + " · Enter use · Esc cancel"
			}
			if editor.notice != "" {
				hint = editor.notice
			}
			fmt.Fprintf(out, "\r\n%s%s%s", style.Muted, ansi.Truncate(hint, width-2, ""), style.Reset)
			rows++
		}
		if final {
			fmt.Fprint(out, "\r\n")
		} else {
			caretRow = view.row
			if up := rows - 1 - caretRow; up > 0 {
				fmt.Fprintf(out, "\x1b[%dA", up)
			}
			fmt.Fprintf(out, "\r\x1b[%dC", 2+view.column)
		}
	}
	draw(false)
	var pendingEscape bool
	for {
		r, _, err := input.ReadRune()
		if err != nil {
			draw(true)
			return "", err
		}
		key := string(r)
		editor.notice = ""
		if r == '\x1b' && !pendingEscape {
			// Terminal input can split ESC and [200~ across reads. Buffered()
			// describes only bytes already fetched by bufio, not bytes waiting
			// in the terminal, so it cannot identify bracketed paste reliably.
			pendingEscape = true
			continue
		}
		if pendingEscape {
			pendingEscape = false
			if r == '[' || r == 'O' {
				key = "\x1b" + string(r)
				for len(key) < 16 {
					b, e := input.ReadByte()
					if e != nil { draw(true); return "", e }
					key += string(b)
					if b >= 0x40 && b <= 0x7e { break }
				}
			} else {
				// A standalone Escape dismisses suggestions; the following key
				// remains input rather than being swallowed.
				_, _ = editor.key("\x1b")
			}
		}
		if key == "\x1b[200~" {
			// Paste is one editable message, even when it contains newlines.
			pasted, tooLarge, err := readAgentPaste(input, 32000-len(string(editor.text)))
			if err != nil {
				draw(true)
				return "", err
			}
			if tooLarge {
				editor.notice = "Paste too large; draft kept"
			} else if editor.searchActive {
				editor.searchKey(pasted)
			} else {
				editor.insert(pasted)
			}
		} else {
			done, readErr := editor.key(key)
			if readErr != nil {
				draw(true)
				return "", readErr
			}
			if done {
				draw(true)
				return string(editor.text), nil
			}
		}
		if len(string(editor.text)) > 32000 {
			draw(true)
			return "", cli.Usage("Message exceeds 32000 bytes")
		}
		draw(false)
	}
}

type agentInputEditor struct {
	text             []rune
	cursor, selected int
	dismissed        bool
	commands         []cli.AgentCommandSuggestion
	history          []string
	historyIndex     int
	draft            string
	width            int
	notice           string
	searchActive     bool
	searchQuery      string
	searchOriginal   string
	searchCursor     int
	searchIndex      int
}

func (e *agentInputEditor) matches() []cli.AgentCommandSuggestion {
	var result []cli.AgentCommandSuggestion
	for _, c := range e.commands {
		if strings.HasPrefix(c.Command, string(e.text)) {
			if c.Command == string(e.text) {
				result = append([]cli.AgentCommandSuggestion{c}, result...)
			} else {
				result = append(result, c)
			}
		}
	}
	return result
}
func (e *agentInputEditor) menuOpen() bool {
	return !e.dismissed && strings.HasPrefix(string(e.text), "/") && len(e.matches()) > 0
}
func (e *agentInputEditor) insert(text string) {
	if len(string(e.text))+len(text) > 32000 {
		e.notice = "Message limit: 32000 bytes"
		return
	}
	runes := []rune(text)
	e.text = append(e.text[:e.cursor], append(runes, e.text[e.cursor:]...)...)
	e.cursor += len(runes)
	// Inserting a joiner/base before an existing mark can merge clusters across
	// the insertion point. Keep the caret at the merged cluster's far boundary.
	e.snapCursorForward()
	e.selected = 0
	e.dismissed = false
}
func (e *agentInputEditor) key(key string) (bool, error) {
	if e.searchActive && key != "\x03" && key != "\x04" {
		e.searchKey(key)
		return false, nil
	}
	switch key {
	case "\x12":
		e.searchActive = true
		e.searchOriginal, e.searchCursor = string(e.text), e.cursor
		e.searchQuery, e.searchIndex = "", len(e.history)
		e.dismissed = true
		e.findHistory(true)
	case "\x03":
		return false, context.Canceled
	case "\x04":
		if len(e.text) == 0 {
			return false, io.EOF
		}
	case "\x1b":
		e.dismissed = true
	case "\x1b[A", "\x1b[B":
		if e.menuOpen() {
			delta := 1
			if key == "\x1b[A" {
				delta = -1
			}
			n := len(e.matches())
			e.selected = (e.selected + delta + n) % n
		} else {
			delta := 1
			if key == "\x1b[A" {
				delta = -1
			}
			if len(e.layout(max(1, e.width)).lines) > 1 && e.width > 0 || strings.ContainsRune(string(e.text), '\n') {
				e.moveVertical(delta)
			} else {
				e.recall(delta)
			}
		}
	case "\x10", "\x0e":
		delta := 1
		if key == "\x10" {
			delta = -1
		}
		e.recall(delta)
	case "\t":
		if e.menuOpen() {
			e.text = []rune(e.matches()[e.selected].Command)
			e.cursor = len(e.text)
			e.dismissed = true
			e.selected = 0
		}
	case "\n", "\x1b\r", "\x1b\n", "\x1b[13;2u", "\x1b[27;2;13~":
		e.insert("\n")
	case "\r":
		if e.menuOpen() {
			e.text = []rune(e.matches()[e.selected].Command)
			e.cursor = len(e.text)
		}
		return true, nil
	case "\x1b[D":
		if e.cursor > 0 {
			e.cursor = e.previousBoundary()
		}
	case "\x1b[C":
		if e.cursor < len(e.text) {
			e.cursor = e.nextBoundary()
		}
	case "\x01", "\x1b[H", "\x1bOH":
		e.cursor = 0
	case "\x05", "\x1b[F", "\x1bOF":
		e.cursor = len(e.text)
	case "\x15":
		e.text = append([]rune{}, e.text[e.cursor:]...)
		e.cursor = 0
		e.selected = 0
	case "\x7f", "\b":
		if e.cursor > 0 {
			previous := e.previousBoundary()
			e.text = append(e.text[:previous], e.text[e.cursor:]...)
			e.cursor = previous
			e.selected = 0
		}
	case "\x1b[3~":
		if e.cursor < len(e.text) {
			e.text = append(e.text[:e.cursor], e.text[e.nextBoundary():]...)
			e.selected = 0
		}
	default:
		if !strings.HasPrefix(key, "\x1b") {
			for _, r := range key {
				if !unicode.IsControl(r) {
					e.insert(string(r))
				}
			}
		}
	}
	return false, nil
}
