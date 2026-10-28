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
func readAgentSessionLine(input *bufio.Reader, out io.Writer, style cli.PresentationStyle, commands []cli.AgentCommandSuggestion) (string, error) {
	return readAgentSessionLineWithHistory(input, out, style, commands, nil)
}

func newAgentSessionInput() func(*bufio.Reader, io.Writer, cli.PresentationStyle, []cli.AgentCommandSuggestion) (string, error) {
	var history []string
	return func(input *bufio.Reader, out io.Writer, style cli.PresentationStyle, commands []cli.AgentCommandSuggestion) (string, error) {
		line, err := readAgentSessionLineWithHistory(input, out, style, commands, history)
		if err == nil && strings.TrimSpace(line) != "" {
			history = append(history, strings.TrimSuffix(line, "\n"))
			if len(history) > 50 {
				history = history[len(history)-50:]
			}
		}
		return line, err
	}
}

func readAgentSessionLineWithHistory(input *bufio.Reader, out io.Writer, style cli.PresentationStyle, commands []cli.AgentCommandSuggestion, history []string) (string, error) {
	file, ok := cli.AgentOutputTarget(out).(*os.File)
	if !ok || !term.IsTerminal(os.Stdin.Fd()) || !term.IsTerminal(file.Fd()) || os.Getenv("TERM") == "dumb" || os.Getenv("OFFGRID_PLAIN") == "1" || os.Getenv("OFFGRID_TUI") == "0" {
		return cli.ReadAgentPlainLine(input, out, style)
	}
	width, height, err := term.GetSize(file.Fd())
	if err != nil || width < 32 || height < 10 {
		return cli.ReadAgentPlainLine(input, out, style)
	}
	state, err := term.MakeRaw(os.Stdin.Fd())
	if err != nil {
		return cli.ReadAgentPlainLine(input, out, style)
	}
	defer term.Restore(os.Stdin.Fd(), state)
	fmt.Fprint(out, "\r\n\x1b[?2004h")
	defer fmt.Fprint(out, "\x1b[?2004l")
	editor := agentInputEditor{commands: commands, history: history, historyIndex: len(history)}
	rows := 0
	draw := func(final bool) {
		// Re-measure between keys so a resized terminal cannot retain stale width.
		if w, _, e := term.GetSize(file.Fd()); e == nil && w > 4 {
			width = w
		}
		fmt.Fprint(out, "\r\x1b[2K")
		for i := 0; i < rows; i++ {
			fmt.Fprint(out, "\r\n\x1b[2K")
		}
		if rows > 0 {
			fmt.Fprintf(out, "\x1b[%dA\r", rows)
		}
		text, cursor := editor.view(width - 4)
		fmt.Fprintf(out, "%s>%s %s", style.Primary, style.Reset, text)
		rows = 0
		if !final && editor.menuOpen() {
			matches := editor.matches()
			start := editor.selected / 4 * 4
			for i := start; i < len(matches) && i < start+4; i++ {
				prefix := "  "
				if i == editor.selected {
					prefix = "> "
				}
				label := prefix + matches[i].Command + "  " + matches[i].Description
				fmt.Fprintf(out, "\r\n%s%s%s", style.Muted, ansi.Truncate(label, width-1, "..."), style.Reset)
				rows++
			}
		}
		if rows > 0 {
			fmt.Fprintf(out, "\x1b[%dA", rows)
		}
		fmt.Fprintf(out, "\r\x1b[%dC", 2+cursor)
		if final {
			fmt.Fprint(out, "\r\n")
		}
	}
	draw(false)
	for {
		r, _, err := input.ReadRune()
		if err != nil {
			draw(true)
			return "", err
		}
		key := string(r)
		if r == '\x1b' {
			// Escape alone closes suggestions immediately. CSI sequences are
			// interpreted only when already available; never wait for another key.
			if input.Buffered() > 0 {
				next, _ := input.Peek(1)
				if next[0] == '[' || next[0] == 'O' {
					b, _ := input.ReadByte()
					key += string(b)
					for len(key) < 16 {
						b, e := input.ReadByte()
						if e != nil {
							draw(true)
							return "", e
						}
						key += string(b)
						if b >= 0x40 && b <= 0x7e {
							break
						}
					}
				}
			}
		}
		if key == "\x1b[200~" {
			// Paste is one editable message, even when it contains newlines.
			var pasted strings.Builder
			for !strings.HasSuffix(pasted.String(), "\x1b[201~") {
				b, e := input.ReadByte()
				if e != nil {
					draw(true)
					return "", e
				}
				pasted.WriteByte(b)
				if pasted.Len() > 32006 {
					draw(true)
					return "", cli.Usage("Message exceeds 32000 bytes")
				}
			}
			editor.insert(strings.ReplaceAll(strings.TrimSuffix(pasted.String(), "\x1b[201~"), "\r\n", "\n"))
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
}

func (e *agentInputEditor) matches() []cli.AgentCommandSuggestion {
	var result []cli.AgentCommandSuggestion
	for _, c := range e.commands {
		if strings.HasPrefix(c.Command, string(e.text)) {
			result = append(result, c)
		}
	}
	return result
}
func (e *agentInputEditor) menuOpen() bool {
	return !e.dismissed && strings.HasPrefix(string(e.text), "/") && len(e.matches()) > 0
}
func (e *agentInputEditor) insert(text string) {
	runes := []rune(text)
	e.text = append(e.text[:e.cursor], append(runes, e.text[e.cursor:]...)...)
	e.cursor += len(runes)
	e.selected = 0
	e.dismissed = false
}
func (e *agentInputEditor) key(key string) (bool, error) {
	switch key {
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
		} else if len(e.history) > 0 {
			if e.historyIndex == len(e.history) {
				e.draft = string(e.text)
			}
			if key == "\x1b[A" && e.historyIndex > 0 {
				e.historyIndex--
			}
			if key == "\x1b[B" && e.historyIndex < len(e.history) {
				e.historyIndex++
			}
			text := e.draft
			if e.historyIndex < len(e.history) {
				text = e.history[e.historyIndex]
			}
			e.text = []rune(text)
			e.cursor = len(e.text)
			e.selected = 0
			e.dismissed = true
		}
	case "\t":
		if e.menuOpen() {
			e.text = []rune(e.matches()[e.selected].Command)
			e.cursor = len(e.text)
			e.dismissed = true
			e.selected = 0
		}
	case "\r", "\n":
		if e.menuOpen() {
			e.text = []rune(e.matches()[e.selected].Command)
			e.cursor = len(e.text)
		}
		return true, nil
	case "\x1b[D":
		if e.cursor > 0 {
			e.cursor--
		}
	case "\x1b[C":
		if e.cursor < len(e.text) {
			e.cursor++
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
			e.text = append(e.text[:e.cursor-1], e.text[e.cursor:]...)
			e.cursor--
			e.selected = 0
		}
	case "\x1b[3~":
		if e.cursor < len(e.text) {
			e.text = append(e.text[:e.cursor], e.text[e.cursor+1:]...)
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

func (e *agentInputEditor) view(width int) (string, int) {
	if width < 1 {
		width = 1
	}
	// Horizontal viewport avoids reflowing input over transcript or menu rows.
	visible := func(runes []rune) string {
		return strings.NewReplacer("\n", "\\n", "\t", " ").Replace(cli.TerminalSafe(string(runes)))
	}
	start := e.cursor
	for start > 0 && ansi.StringWidth(visible(e.text[start-1:e.cursor])) < width {
		start--
	}
	before := visible(e.text[start:e.cursor])
	return ansi.Truncate(visible(e.text[start:]), width, ""), ansi.StringWidth(before)
}
