package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"time"
)

// Public response previews are provisional; committed steps/results remain
// separate. Losing this connection does not cancel work or resubmit the prompt.
func renderAgentStream(w io.Writer, body io.Reader) error {
	return renderAgentStreamTo(w, w, body)
}

func renderAgentStreamTo(w, progress io.Writer, body io.Reader) error {
	scanner := bufio.NewScanner(body)
	scanner.Buffer(make([]byte, 64*1024), 8*1024*1024)
	runID := ""
	lastPhase, lastPreview, iteration := "", "", 0
	lastChildren := ""
	previewOpen := false
	endPreview := func() {
		if previewOpen {
			fmt.Fprintln(progress)
			previewOpen = false
		}
	}
	defer endPreview()
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
			fmt.Fprintf(progress, "Run: %s\n", terminalSafe(runID))
		}
		if event.Status == "waiting_for_children" {
			data, _ := json.Marshal(event.Children)
			if string(data) != lastChildren {
				endPreview()
				renderAgentSnapshot(progress, event.agentRunSnapshot)
				lastChildren = string(data)
			}
		}
		if p := event.Progress; p != nil && event.Type == "status" {
			phase := fmt.Sprintf("%d:%s:%s", p.Iteration, p.Phase, p.Tool)
			if phase != lastPhase {
				endPreview()
				labels := map[string]string{"queued": "Waiting for inference", "loading": "Loading model", "processing": "Preparing prompt", "generating": "Generating response", "preparing_tool": "Preparing tool call", "tool": "Running tool", "approval": "Awaiting approval"}
				label := labels[p.Phase]
				if label == "" {
					label = "Running"
				}
				fmt.Fprintf(progress, "Step %d · %s", p.Iteration, label)
				if p.Tool != "" {
					fmt.Fprintf(progress, " · %s", terminalSafe(p.Tool))
				}
				fmt.Fprintln(progress)
				lastPhase = phase
			}
			if p.Iteration != iteration {
				lastPreview = ""
				iteration = p.Iteration
			}
			if p.Preview != "" && p.Preview != lastPreview {
				if !previewOpen {
					fmt.Fprintln(progress, "Response preview (not complete):")
					previewOpen = true
				}
				if strings.HasPrefix(p.Preview, lastPreview) {
					fmt.Fprint(progress, terminalSafe(strings.TrimPrefix(p.Preview, lastPreview)))
				} else {
					fmt.Fprint(progress, "\n", terminalSafe(p.Preview))
				}
				lastPreview = p.Preview
			}
		}
		switch event.Type {
		case "step":
			endPreview()
			if event.ToolName != "" {
				fmt.Fprintf(progress, "  %s · %s\n", terminalSafe(event.ToolName), truncateTerminalText(terminalSafe(event.ToolResult), 180))
			}
		case "done", "approval_required", "input_required", "error":
			endPreview()
			renderAgentSnapshot(w, event.agentRunSnapshot)
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
