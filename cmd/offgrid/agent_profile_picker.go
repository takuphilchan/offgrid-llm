package main

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"unicode"

	"github.com/charmbracelet/x/ansi"
	"github.com/charmbracelet/x/term"
	"github.com/takuphilchan/offgrid-llm/internal/agents"
	"github.com/takuphilchan/offgrid-llm/internal/cli"
)

type profilePicker struct {
	profiles []agents.AgentProfile
	query    []rune
	selected int
	note     string
}

func (p *profilePicker) matches() []agents.AgentProfile {
	// Numeric input preserves the familiar numbered-selection shortcut.
	if _, err := strconv.Atoi(string(p.query)); err == nil {
		return p.profiles
	}
	return cli.MatchAgentProfiles(p.profiles, string(p.query))
}

func (p *profilePicker) key(key string) (*agents.AgentProfile, error) {
	switch key {
	case "\x03", "\x1b":
		return nil, cli.ErrAgentSelectionCancelled
	case "\x04":
		if len(p.query) == 0 {
			return nil, io.EOF
		}
	case "\x1b[A", "\x1b[B":
		n := len(p.matches())
		if n > 0 {
			delta := 1
			if key == "\x1b[A" {
				delta = -1
			}
			p.selected = (p.selected + delta + n) % n
		}
	case "\r", "\n":
		matches := p.matches()
		index := p.selected
		if n, err := strconv.Atoi(string(p.query)); err == nil {
			index = n - 1
		}
		if index < 0 || index >= len(matches) {
			p.note = "Choose a listed profile number or change the search."
			return nil, nil
		}
		choice := matches[index]
		return &choice, nil
	case "\x7f", "\b":
		if len(p.query) > 0 {
			p.query = p.query[:len(p.query)-1]
		}
		p.selected = 0
	case "\x15":
		p.query = nil
		p.selected = 0
	default:
		if !strings.HasPrefix(key, "\x1b") {
			for _, r := range key {
				if !unicode.IsControl(r) && len(p.query) < 128 {
					p.query = append(p.query, r)
				}
			}
			p.selected = 0
		}
	}
	if key != "\r" && key != "\n" {
		p.note = ""
	}
	return nil, nil
}

func pickAgentProfile(input *bufio.Reader, out io.Writer, style cli.PresentationStyle, profiles []agents.AgentProfile) (agents.AgentProfile, error) {
	file, ok := cli.AgentOutputTarget(out).(*os.File)
	if !ok || !term.IsTerminal(os.Stdin.Fd()) || !term.IsTerminal(file.Fd()) || os.Getenv("TERM") == "dumb" || os.Getenv("OFFGRID_PLAIN") == "1" || os.Getenv("OFFGRID_TUI") == "0" {
		return cli.PickAgentProfilePlain(input, out, style, profiles)
	}
	width, height, err := term.GetSize(file.Fd())
	if err != nil || width < 32 || height < 10 {
		return cli.PickAgentProfilePlain(input, out, style, profiles)
	}
	state, err := term.MakeRaw(os.Stdin.Fd())
	if err != nil {
		return cli.PickAgentProfilePlain(input, out, style, profiles)
	}
	defer term.Restore(os.Stdin.Fd(), state)
	fmt.Fprint(out, "\x1b[?2004h")
	defer fmt.Fprint(out, "\x1b[?25h\x1b[?2004l")
	picker := profilePicker{profiles: profiles}
	fmt.Fprint(out, "\r\n")
	rows := 0
	draw := func(final string) {
		if w, h, e := term.GetSize(file.Fd()); e == nil {
			width, height = max(w, 4), max(h, 4)
		}
		if rows > 0 {
			fmt.Fprint(out, "\r\x1b[1A") // The caret rests on the search row.
		}
		fmt.Fprint(out, "\x1b[?25l\r\x1b[2K")
		for i := 1; i < rows; i++ {
			fmt.Fprint(out, "\r\n\x1b[2K")
		}
		if rows > 1 {
			fmt.Fprintf(out, "\x1b[%dA\r", rows-1)
		}
		if final != "" {
			fmt.Fprint(out, ansi.Truncate(cli.TerminalSafe(final), width-1, "..."), "\r\n")
			return
		}
		lines := []string{
			"  Choose an agent",
			"  Search or number: " + cli.TerminalSafe(string(picker.query)),
			"",
		}
		matches := picker.matches()
		limit := max(1, min(6, height-8))
		start := picker.selected / limit * limit
		for i := start; i < len(matches) && i < start+limit; i++ {
			prefix := "    "
			if i == picker.selected {
				prefix = "  > "
			}
			p := matches[i]
			lines = append(lines, fmt.Sprintf("%s%d. %s", prefix, i+1, p.Settings.Name))
			if i == picker.selected {
				lines = append(lines, "      "+fmt.Sprintf("%s  |  revision %d", p.Settings.Model, p.Revision))
				if p.Settings.Description != "" && height > 12 {
					lines = append(lines, "      "+p.Settings.Description)
				}
			}
		}
		if len(matches) == 0 {
			lines = append(lines, "    No matching agents. Change the search.")
		}
		hint := "  Up/Down move   Enter open   Esc cancel"
		if picker.note != "" {
			hint = "  " + picker.note
		}
		lines = append(lines, "", hint)
		for i, line := range lines {
			if i > 0 {
				fmt.Fprint(out, "\r\n")
			}
			indent := len(line) - len(strings.TrimLeft(line, " "))
			indent = min(indent, max(0, width-4))
			clean := strings.Repeat(" ", indent) + ansi.Truncate(strings.Join(strings.Fields(cli.TerminalSafe(line)), " "), width-indent-1, "...")
			switch {
			case i == 0:
				fmt.Fprint(out, style.Bold, clean, style.Reset)
			case i == len(lines)-1:
				fmt.Fprint(out, style.Muted, clean, style.Reset)
			case strings.HasPrefix(line, "  > "):
				fmt.Fprint(out, style.Primary, style.Bold, style.Selected, clean, style.Reset)
			default:
				fmt.Fprint(out, clean)
			}
		}
		rows = len(lines)
		if rows > 1 {
			fmt.Fprintf(out, "\x1b[%dA\r", rows-1)
		}
		// Put the visible caret where typing changes the search, not in the title.
		fmt.Fprintf(out, "\x1b[1B\r\x1b[%dC\x1b[?25h", min(width-2, ansi.StringWidth(cli.TerminalSafe(lines[1]))))
	}
	draw("")
	for {
		key, err := readAgentPickerKey(input)
		if err != nil {
			draw("Agent selection closed.")
			return agents.AgentProfile{}, err
		}
		if key == "\x1b[200~" {
			var paste strings.Builder
			for !strings.HasSuffix(paste.String(), "\x1b[201~") {
				b, readErr := input.ReadByte()
				if readErr != nil {
					draw("Agent selection closed.")
					return agents.AgentProfile{}, readErr
				}
				paste.WriteByte(b)
				if paste.Len() > 32006 {
					draw("Paste too large. Agent selection cancelled.")
					return agents.AgentProfile{}, cli.ErrAgentSelectionCancelled
				}
			}
			for _, r := range strings.TrimSuffix(paste.String(), "\x1b[201~") {
				if !unicode.IsControl(r) {
					picker.key(string(r))
				}
			}
			draw("")
			continue
		}
		choice, err := picker.key(key)
		if err != nil {
			draw(" ")
			return agents.AgentProfile{}, err
		}
		if choice != nil {
			// The next screen names the chosen agent; avoid a duplicate receipt.
			draw(" ")
			return *choice, nil
		}
		draw("")
	}
}

func readAgentPickerKey(input *bufio.Reader) (string, error) {
	r, _, err := input.ReadRune()
	if err != nil {
		return "", err
	}
	key := string(r)
	if r == '\x1b' && input.Buffered() > 0 {
		b, _ := input.Peek(1)
		if b[0] == '[' || b[0] == 'O' {
			b, _ := input.ReadByte()
			key += string(b)
			for len(key) < 16 {
				b, e := input.ReadByte()
				if e != nil {
					return "", e
				}
				key += string(b)
				if b >= 0x40 && b <= 0x7e {
					break
				}
			}
		}
	}
	return key, nil
}
