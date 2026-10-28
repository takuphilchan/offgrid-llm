package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/takuphilchan/offgrid-llm/internal/cli"
	"io"
	"os"
	"strings"
	"time"

	"github.com/charmbracelet/x/term"
)

// Public response previews are provisional; committed steps/results remain
// separate. Losing this connection does not cancel work or resubmit the prompt.
func renderAgentStream(w io.Writer, body io.Reader) error {
	return renderAgentStreamTo(w, w, body)
}

func renderAgentStreamTo(w, progress io.Writer, body io.Reader) error {
	terminal := false
	style := cli.PresentationStyle{}
	if file, ok := cli.AgentOutputTarget(w).(*os.File); ok {
		if term.IsTerminal(file.Fd()) {
			terminal = true
			style = legacyPresenter().Style
		}
	}
	animate := terminal && os.Getenv("TERM") != "dumb" && os.Getenv("OFFGRID_ANIMATIONS") != "0" && os.Getenv("OFFGRID_PLAIN") != "1"
	if file, ok := cli.AgentOutputTarget(progress).(*os.File); !ok || !term.IsTerminal(file.Fd()) {
		animate = false
	}
	return renderAgentStreamMode(w, progress, body, terminal, animate, style)
}

func renderAgentStreamMode(w, progress io.Writer, body io.Reader, terminal, animate bool, style cli.PresentationStyle) error {
	display := newAgentLiveDisplay(progress, animate)
	defer display.close()
	w, progress = agentDisplayWriter{display, w}, agentDisplayWriter{display, progress}
	draft := &agentDraft{}
	defer func() { draft.end(w, "Stream ended before completion; inspect saved work.") }()
	started := time.Now()
	scanner := bufio.NewScanner(body)
	scanner.Buffer(make([]byte, 64*1024), 8*1024*1024)
	runID := ""
	lastPhase := ""
	lastChildren := ""
	observed := map[string]bool{}
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if !strings.HasPrefix(line, "data:") {
			continue
		}
		var event struct {
			agentRunSnapshot
			Type       string `json:"type"`
			StepType   string `json:"step_type"`
			ToolName   string `json:"tool_name"`
			ToolResult string `json:"tool_result"`
		}
		if err := json.Unmarshal([]byte(strings.TrimSpace(strings.TrimPrefix(line, "data:"))), &event); err != nil {
			return fmt.Errorf("invalid agent progress; inspect the saved run before retrying: %w", err)
		}
		if event.Type == "activity" {
			continue
		}
		if len(event.Steps) > 0 {
			var evidence bytes.Buffer
			cli.RenderExecutionObservations(&evidence, event.Steps, observed)
			if evidence.Len() > 0 {
				draft.end(w, "Draft ended; work continues.")
				fmt.Fprint(w, evidence.String())
			}
		}
		if event.Type == "snapshot" || event.Type == "snapshot_recovery" {
			event.Type = "status"
			switch event.Status {
			case "completed":
				event.Type = "done"
			case "waiting_for_approval":
				event.Type = "approval_required"
			case "waiting_for_input":
				event.Type = "input_required"
			case "interrupted", "uncertain", "failed", "cancelled":
				event.Type = "error"
			}
		}
		if runID == "" && event.RunID != "" {
			runID = event.RunID
		}
		if event.Status == "waiting_for_children" {
			data, _ := json.Marshal(event.Children)
			if string(data) != lastChildren {
				renderAgentSnapshot(progress, event.agentRunSnapshot)
				lastChildren = string(data)
			}
		}
		if p := event.Progress; p != nil && event.Type == "status" {
			if terminal && p.Phase == "generating" && p.Preview != "" {
				display.phase("Responding", false)
				draft.update(w, p.Iteration, p.Preview)
			} else if p.Phase != "generating" && (p.Iteration != draft.iteration || p.Tool != "") {
				draft.end(w, "Draft ended; work continues.")
			}
			phase := fmt.Sprintf("%d:%s:%s", p.Iteration, p.Phase, p.Tool)
			liveLabel := map[string]string{"queued": "Waiting for model", "loading": "Loading model", "processing": "Working", "preparing_tool": "Choosing action", "generating": "Responding", "tool": "Running action"}[p.Phase]
			if liveLabel != "" {
				display.phase(liveLabel, !draft.open)
			}
			labels := map[string]string{"queued": "Waiting for inference", "loading": "Loading model", "tool": "Running tool", "approval": "Awaiting approval"}
			if phase != lastPhase && (labels[p.Phase] != "" || p.Tool != "") {
				label := labels[p.Phase]
				if label == "" {
					label = "Running"
				}
				if p.Tool != "" {
					label = cli.AgentActionLabel(p.Tool)
				}
				display.phase(label, !draft.open)
				fmt.Fprintf(progress, "  %s%s%s\n", style.Muted, label, style.Reset)
				lastPhase = phase
			}
		}
		switch event.Type {
		case "step":
			draft.end(w, "Draft ended; work continues.")
			if event.ToolName != "" && !strings.HasSuffix(lastPhase, ":"+event.ToolName) {
				fmt.Fprintf(progress, "  %s\n", cli.AgentActionLabel(event.ToolName))
			}
		case "done", "approval_required", "input_required", "error":
			display.phase("", false)
			if !draft.complete(w, progress, event.agentRunSnapshot, style, time.Since(started)) {
				draft.end(w, "Draft is not the completed answer; see the saved outcome below.")
				cli.RenderAgentOutcome(w, progress, event.agentRunSnapshot, style, time.Since(started))
			}
			if event.Type == "error" {
				return fmt.Errorf("agent run stopped; use offgrid agent status %s before taking further action", terminalSafe(runID))
			}
			return nil
		}
	}
	if err := scanner.Err(); err != nil {
		return fmt.Errorf("progress disconnected; run %s may still be active: %w", terminalSafe(runID), err)
	}
	return fmt.Errorf("progress ended before a terminal state; inspect with offgrid agent status %s", terminalSafe(runID))
}

func showSpinner(msg string, stop chan bool) {
	i := 0
	for {
		select {
		case <-stop:
			return
		default:
			fmt.Printf("\r%s%s%s %s", brandPrimary, spinnerFrames[i%len(spinnerFrames)], colorReset, msg)
			time.Sleep(100 * time.Millisecond)
			i++
		}
	}
}
