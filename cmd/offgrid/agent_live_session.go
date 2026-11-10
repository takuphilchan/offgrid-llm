package main

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/charmbracelet/x/ansi"
	"github.com/charmbracelet/x/term"
	"github.com/takuphilchan/offgrid-llm/internal/cli"
)

// The main loop is the only keyboard reader. The stream may write concurrently,
// but one lock owns the output region and the editor. There is no input pump to
// cancel or leave behind when workspace consent takes over the shared reader.
func followAgentLive(ctx context.Context, input *bufio.Reader, out io.Writer, body io.Reader, control func(string) (string, bool)) ([]string, error) {
	file, ok := cli.AgentOutputTarget(out).(*os.File)
	if !ok || !term.IsTerminal(os.Stdin.Fd()) || !term.IsTerminal(file.Fd()) || os.Getenv("TERM") == "dumb" || os.Getenv("OFFGRID_PLAIN") == "1" || os.Getenv("OFFGRID_TUI") == "0" {
		return nil, renderAgentStreamTo(out, out, body)
	}
	w, h, err := term.GetSize(file.Fd())
	if err != nil || w < 40 || h < 16 {
		return nil, renderAgentStreamTo(out, out, body)
	}
	poll, err := newAgentInputPoller()
	if err != nil {
		return nil, renderAgentStreamTo(out, out, body)
	}
	state, err := term.MakeRaw(os.Stdin.Fd())
	if err != nil {
		return nil, renderAgentStreamTo(out, out, body)
	}
	defer term.Restore(os.Stdin.Fd(), state)
	display := &agentLiveEditor{out: out, width: w, height: h, started: time.Now()}
	display.begin()
	var receipts []string
	defer func() {
		display.close()
		// Never insert controls into an in-progress streamed sentence. Keep
		// full receipts after that stream; the editor shows immediate status.
		for _, receipt := range receipts {
			fmt.Fprint(out, cli.TerminalSafe(receipt)+"\r\n")
		}
	}()
	keys := agentLiveKeys{}
	// Consume only already buffered bytes; no speculative read can take a later
	// approval response. This also preserves type-ahead from the previous editor.
	if n := input.Buffered(); n > 0 {
		keys.data = make([]byte, n)
		_, _ = io.ReadFull(input, keys.data)
	}
	done := make(chan error, 1)
	go func() { done <- renderAgentStreamMode(display, display, body, true, false, legacyPresenter().Style) }()
	timer := time.NewTicker(25 * time.Millisecond)
	defer timer.Stop()
	var queued string
	finish := func(result error) ([]string, error) {
		// Take only bytes already waiting in this phase, before cooked consent
		// begins. Treat them as draft text even if they contain Enter or "y".
		for batch := 0; batch < 64; batch++ {
			data, e := poll()
			keys.data = append(keys.data, data...)
			if e != nil {
				result = fmt.Errorf("input handoff could not be verified; inspect saved work: %w", e)
				break
			}
			if len(data) == 0 {
				break
			}
			if batch == 63 {
				result = fmt.Errorf("input remains buffered; access handoff was stopped; inspect saved work")
			}
		}
		display.mu.Lock()
		defer display.mu.Unlock()
		for {
			key, pasted, ready := keys.next(time.Now().Add(time.Second))
			if !ready {
				break
			}
			if pasted {
				if !keys.overflow {
					display.editor.insert(livePaste(key))
				}
				continue
			}
			if key == "\r" {
				display.editor.insert("\n")
				continue
			}
			_, _ = display.editor.key(key)
		}
		// Incomplete input is a draft, never input for the next consent prompt.
		if keys.paste && !keys.overflow {
			display.editor.insert(livePaste(string(keys.pasted) + string(keys.data)))
		}
		var drafts []string
		if queued != "" {
			drafts = append(drafts, queued)
		}
		if text := string(display.editor.text); strings.TrimSpace(text) != "" {
			drafts = append(drafts, text)
		}
		return drafts, result
	}
	for {
		select {
		case result := <-done:
			return finish(result)
		case <-ctx.Done():
			if closer, ok := body.(io.Closer); ok {
				_ = closer.Close()
			}
			<-done
			return finish(ctx.Err())
		case <-timer.C:
		}
		// Prioritize ended streams over type-ahead. No correction can become a
		// new task if completion races the Enter key.
		select {
		case result := <-done:
			return finish(result)
		default:
		}
		data, readErr := poll()
		keys.data = append(keys.data, data...)
		if readErr != nil {
			control("/stop")
			if closer, ok := body.(io.Closer); ok {
				_ = closer.Close()
			}
			<-done
			return finish(readErr)
		}
		for {
			key, paste, ready := keys.next(time.Now())
			if !ready {
				break
			}
			display.mu.Lock()
			action := ""
			if paste {
				if keys.overflow {
					display.notice = "Paste too large; draft kept"
				} else {
					display.editor.insert(livePaste(key))
				}
			} else if key == "\x03" {
				action = "/stop"
			} else {
				complete, keyErr := display.editor.key(key)
				if keyErr != nil {
					display.notice = "Use Ctrl+C or /stop to stop this task; your draft is retained."
				}
				if complete {
					action = strings.TrimSpace(string(display.editor.text))
				}
			}
			display.mu.Unlock()
			if action != "" {
				message, accepted := "", false
				if strings.HasPrefix(action, "/queue ") {
					value := strings.TrimSpace(strings.TrimPrefix(action, "/queue "))
					if queued != "" {
						message = "A follow-up is already queued. Your current draft is retained."
					} else if value != "" {
						queued = value
						message = "Follow-up queued for review after this turn. It will not be sent automatically."
						accepted = true
					}
				} else {
					message, accepted = control(action)
				}
				display.mu.Lock()
				display.notice = message
				// Stop must not erase an unfinished correction.
				if accepted && key != "\x03" {
					display.editor = agentInputEditor{}
				}
				display.mu.Unlock()
				if message != "" {
					receipts = append(receipts, message)
				}
			}
		}
		display.mu.Lock()
		if width, height, e := term.GetSize(file.Fd()); e == nil && width > 4 && height > 4 {
			display.resize(width, height)
		}
		display.draw()
		display.mu.Unlock()
	}
}

type agentLiveEditor struct {
	mu            sync.Mutex
	out           io.Writer
	width, height int
	editor        agentInputEditor
	notice        string
	phase         string
	started       time.Time
	lastFrame     string
}

func (d *agentLiveEditor) AgentSessionOutput() bool { return true }

// Progress comes from the same validated event stream as the response. The
// editor owns its status row; a second spinner must not write over the draft.
func (d *agentLiveEditor) AgentProgressPhase(label string) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if label != "" {
		d.phase = cli.TerminalSafe(label)
		d.draw()
	}
}
func (d *agentLiveEditor) begin() {
	// Save the response cursor. The status and correction editor follow the
	// streamed answer instead of occupying the bottom of an empty viewport.
	fmt.Fprint(d.out, "\x1b7\x1b[?2004h")
	d.draw()
}
func (d *agentLiveEditor) resize(w, h int) {
	if d.width == w && d.height == h {
		return
	}
	d.width, d.height = w, h
	d.lastFrame = ""
}
func (d *agentLiveEditor) draw() {
	var frame strings.Builder
	frame.WriteString("\x1b8\x1b[J\r\n")
	d.editor.width = max(1, d.width-4)
	view := d.editor.multilineView(d.editor.width, 2)
	frames := []string{"|", "/", "-", "\\"}
	label := d.phase
	if label == "" {
		label = "Working"
	}
	status := fmt.Sprintf("%s %s · %ds", frames[int(time.Since(d.started)/(125*time.Millisecond))%len(frames)], label, int(time.Since(d.started).Seconds()))
	if os.Getenv("OFFGRID_ANIMATIONS") == "0" {
		status = fmt.Sprintf("%s · %ds", label, int(time.Since(d.started).Seconds()))
	}
	fmt.Fprintf(&frame, "%s\r\n", ansi.Truncate(status, max(1, d.width-1), "…"))
	for i, line := range view.lines {
		_ = i
		prefix := "  "
		if len(d.editor.text) == 0 {
			line = "Type a correction while the agent works"
		}
		fmt.Fprintf(&frame, "%s%s\r\n", prefix, ansi.Truncate(line, d.editor.width, "…"))
	}
	hint := "Enter steer · /queue MESSAGE · Ctrl+C stop"
	if d.notice != "" {
		hint = cli.TerminalSafe(d.notice)
	}
	fmt.Fprintf(&frame, "%s", ansi.Truncate(hint, max(1, d.width-1), "…"))
	fmt.Fprintf(&frame, "\x1b[%dA\x1b[%dG", len(view.lines)-view.row, view.column+3)
	if next := frame.String(); next != d.lastFrame {
		fmt.Fprint(d.out, next)
		d.lastFrame = next
	}
}
func (d *agentLiveEditor) Write(data []byte) (int, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	fmt.Fprint(d.out, "\x1b8\x1b[J")
	// Raw mode does not translate LF. The renderer's trusted formatting uses LF;
	// preserve its CR/ANSI while ensuring text starts at column one after LF.
	text := strings.ReplaceAll(strings.ReplaceAll(string(data), "\r\n", "\n"), "\n", "\r\n")
	_, err := io.WriteString(d.out, text)
	fmt.Fprint(d.out, "\x1b7")
	d.lastFrame = ""
	d.draw()
	if err != nil {
		return 0, err
	}
	return len(data), nil
}
func (d *agentLiveEditor) close() {
	d.mu.Lock()
	defer d.mu.Unlock()
	fmt.Fprint(d.out, "\x1b8\x1b[J")
	fmt.Fprintf(d.out, "\x1b[r\x1b8\r\n\x1b[?2004l")
}
