package main

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/charmbracelet/x/ansi"
	"github.com/charmbracelet/x/term"
	"github.com/takuphilchan/offgrid-llm/internal/cli"
)

type accessPicker struct {
	selected int
	remember bool
}

func (p *accessPicker) key(key string) (string, bool) {
	count := 2
	if p.remember {
		count = 3
	}
	switch key {
	case "\x03", "\x04", "\x1b":
		return "n", true
	case "\x1b[A":
		p.selected = (p.selected + count - 1) % count
	case "\x1b[B":
		p.selected = (p.selected + 1) % count
	case "0", "n", "N":
		p.selected = 0
	case "1", "y", "Y":
		p.selected = 1
	case "2":
		if p.remember {
			p.selected = 2
		}
	case "\r", "\n":
		return []string{"n", "y", "remember"}[p.selected], true
	}
	return "", false
}

// This picker owns stdin only while no generation editor is active. It does
// not create authority; the caller verifies and persists the exact grant.
func chooseWorkspaceAccess(input *bufio.Reader, out io.Writer, style cli.PresentationStyle, remember bool) (string, error) {
	file, ok := cli.AgentOutputTarget(out).(*os.File)
	if !ok || !term.IsTerminal(os.Stdin.Fd()) || !term.IsTerminal(file.Fd()) || os.Getenv("TERM") == "dumb" || os.Getenv("OFFGRID_PLAIN") == "1" || os.Getenv("OFFGRID_TUI") == "0" {
		return cli.ChooseWorkspaceAccessPlain(input, out, style, remember)
	}
	width, height, err := term.GetSize(file.Fd())
	if err != nil || width < 40 || height < 10 {
		return cli.ChooseWorkspaceAccessPlain(input, out, style, remember)
	}
	state, err := term.MakeRaw(os.Stdin.Fd())
	if err != nil {
		return cli.ChooseWorkspaceAccessPlain(input, out, style, remember)
	}
	defer term.Restore(os.Stdin.Fd(), state)
	fmt.Fprint(out, "\x1b[?2004h\x1b[?25l")
	defer fmt.Fprint(out, "\x1b[?2004l\x1b[?25h")
	picker := accessPicker{remember: remember}
	labels := []string{"Cancel", "Allow once"}
	if remember {
		labels = append(labels, "Remember this folder for this account and service")
	}
	rows := len(labels) + 1
	drawn := false
	draw := func(final string) {
		if drawn {
			fmt.Fprintf(out, "\x1b[%dA\r", rows)
		}
		if w, _, e := term.GetSize(file.Fd()); e == nil {
			width = max(w, 4)
		}
		for i := 0; i < rows; i++ {
			fmt.Fprint(out, "\x1b[2K")
			line := ""
			if final != "" {
				if i == 0 {
					line = "  " + final
				}
			} else if i < len(labels) {
				prefix := "    "
				if i == picker.selected {
					prefix = "  > "
				}
				line = fmt.Sprintf("%s%d  %s", prefix, i, labels[i])
			} else {
				line = "  Up/Down choose · Enter confirm · Esc cancel"
			}
			if final == "" && i == picker.selected {
				fmt.Fprint(out, style.Bold)
			}
			fmt.Fprint(out, ansi.Truncate(line, width-1, "…"), style.Reset, "\r\n")
		}
		drawn = true
	}
	draw("")
	for {
		key, err := readAgentPickerKey(input)
		if err != nil {
			draw("Access cancelled")
			return "", err
		}
		if key == "\x1b[200~" {
			// A pasted command must never become an approval. Consume the whole
			// paste, including embedded newlines, before accepting keyboard input.
			var tail string
			for !strings.HasSuffix(tail, "\x1b[201~") {
				b, err := input.ReadByte()
				if err != nil {
					draw("Access cancelled")
					return "", err
				}
				tail += string(b)
				if len(tail) > 6 {
					tail = tail[len(tail)-6:]
				}
			}
			continue
		}
		if answer, done := picker.key(key); done {
			label := labels[picker.selected]
			if answer == "n" {
				label = "Access cancelled"
			}
			draw(label)
			return answer, nil
		}
		draw("")
	}
}
