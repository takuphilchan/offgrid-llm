package main

import (
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

// Only the terminal adapter enables this. All writes share the lock so a timer
// cannot overwrite an approval, receipt or response. Never hide the cursor.
type agentLiveDisplay struct {
	mu              sync.Mutex
	progress        io.Writer
	active, visible bool
	label           string
	frame           int
	started         time.Time
	stop, done      chan struct{}
}

func newAgentLiveDisplay(progress io.Writer, animate bool) *agentLiveDisplay {
	d := &agentLiveDisplay{progress: progress, active: animate, label: "Working", started: time.Now()}
	if animate {
		d.stop, d.done = make(chan struct{}), make(chan struct{})
		go func() {
			defer close(d.done)
			timer := time.NewTicker(125 * time.Millisecond)
			defer timer.Stop()
			for {
				select {
				case <-d.stop:
					return
				case <-timer.C:
					d.tick()
				}
			}
		}()
	}
	return d
}
func (d *agentLiveDisplay) clearLocked() {
	if d.visible {
		fmt.Fprint(d.progress, "\r\x1b[2K")
		d.visible = false
	}
}
func (d *agentLiveDisplay) tick() {
	d.mu.Lock()
	defer d.mu.Unlock()
	if !d.active {
		return
	}
	columns := 80
	if file, ok := cli.AgentOutputTarget(d.progress).(*os.File); ok {
		if width, _, err := term.GetSize(file.Fd()); err == nil && width > 0 {
			columns = width
		}
	}
	frames := []string{"|", "/", "-", "\\"}
	if terminalSupportsUnicode() {
		frames = []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}
	}
	fmt.Fprint(d.progress, "\r\x1b[2K", agentProgressLine(frames[d.frame%len(frames)], d.label, time.Since(d.started), columns))
	d.frame++
	d.visible = true
}

// Reserve the last terminal cell to avoid wrapping into the next prompt row.
// Measure display cells, not bytes or runes, and retain useful action labels.
func agentProgressLine(frame, label string, elapsed time.Duration, columns int) string {
	label = strings.Join(strings.Fields(cli.TerminalSafe(label)), " ")
	suffix := fmt.Sprintf(" %ds", int(elapsed.Seconds()))
	budget := columns - ansi.StringWidth(frame) - ansi.StringWidth(suffix) - 2
	if budget < 4 {
		return ansi.Truncate(frame+suffix, max(0, columns-1), "")
	}
	return frame + " " + ansi.Truncate(label, budget, "...") + suffix
}
func (d *agentLiveDisplay) phase(label string, active bool) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.clearLocked()
	d.label = label
	d.active = active && d.stop != nil
}
func (d *agentLiveDisplay) close() {
	if d.stop != nil {
		close(d.stop)
		<-d.done
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	d.active = false
	d.clearLocked()
}

type agentDisplayWriter struct {
	display *agentLiveDisplay
	writer  io.Writer
}

func (w agentDisplayWriter) AgentSessionOutput() bool { return cli.AgentSessionOutput(w.writer) }

func (w agentDisplayWriter) Write(p []byte) (int, error) {
	w.display.mu.Lock()
	defer w.display.mu.Unlock()
	w.display.clearLocked()
	return w.writer.Write(p)
}

type agentDraft struct {
	text      string
	iteration int
	open      bool
}

func (d *agentDraft) end(w io.Writer, reason string) {
	if d.open {
		fmt.Fprintf(w, "\n\n[%s]\n", reason)
		d.open = false
		d.text = ""
	}
}
func (d *agentDraft) update(w io.Writer, iteration int, text string) {
	text = cli.TerminalSafe(text)
	if text == "" {
		return
	}
	if d.open && (d.iteration != iteration || !strings.HasPrefix(text, d.text)) {
		d.end(w, "Earlier draft replaced; not a completed answer.")
	}
	if !d.open {
		fmt.Fprint(w, "\nReply in progress\n\n")
		d.open = true
		d.iteration = iteration
		d.text = ""
	}
	fmt.Fprint(w, strings.TrimPrefix(text, d.text))
	d.text = text
}
func (d *agentDraft) complete(w, progress io.Writer, snapshot cli.AgentRunSnapshot, style cli.PresentationStyle, elapsed time.Duration) bool {
	final := cli.TerminalSafe(snapshot.Output)
	if !d.open || snapshot.Status != "completed" || snapshot.Error != "" || snapshot.PendingApproval != nil || snapshot.PendingInput != nil || !strings.HasPrefix(final, d.text) {
		return false
	}
	fmt.Fprint(w, strings.TrimPrefix(final, d.text)+"\n\n")
	fmt.Fprintf(progress, "%sCompleted in %.1fs%s\n", style.Muted, elapsed.Seconds(), style.Reset)
	if !cli.AgentSessionOutput(w) {
		fmt.Fprintf(progress, "%sInspect: offgrid agent status %s%s\n", style.Muted, cli.TerminalSafe(snapshot.RunID), style.Reset)
	}
	d.open = false
	d.text = ""
	return true
}
