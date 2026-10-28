package cli

import (
	"fmt"
	"io"
	"time"
)

// AgentHeader uses the existing invocation's terminal tokens. No cursor control,
// fixed-width frame or truncated model identity is needed for narrow terminals.
func AgentHeader(w io.Writer, style PresentationStyle, model, profile string) {
	fmt.Fprintf(w, "\n%sOffGrid Agent%s\n", style.Bold, style.Reset)
	if model != "" {
		fmt.Fprintf(w, "  %sModel%s  %s\n", style.Muted, style.Reset, TerminalSafe(model))
	} else if profile != "" {
		fmt.Fprintf(w, "  %sProfile%s  %s\n", style.Muted, style.Reset, TerminalSafe(profile))
	}
	fmt.Fprintf(w, "  %sWorking | Ctrl+C to stop%s\n\n", style.Muted, style.Reset)
}

// A friendly label describes the proposed operation, never its success. Unknown
// tools retain their identity rather than being misclassified by name matching.
func AgentActionLabel(tool string) string {
	labels := map[string]string{
		"workspace_list": "Listing files", "workspace_read": "Reading a file",
		"workspace_write": "Writing a file", "workspace_write_from_input": "Writing exact text",
		"workspace_process_start": "Starting a command", "workspace_process_wait": "Waiting for a command",
		"workspace_process_output": "Reading command output", "workspace_process_cancel": "Stopping a command",
		"request_workspace_access": "Requesting folder access", "request_computer_access": "Requesting computer access",
	}
	if label := labels[tool]; label != "" {
		return label
	}
	return "Using " + TerminalSafe(tool)
}

// Keep answer bytes (including code, Unicode and line breaks) distinct from
// progress. Failed/blocked runs retain the canonical recovery/approval display.
func RenderAgentOutcome(w, progress io.Writer, snapshot AgentRunSnapshot, style PresentationStyle, elapsed time.Duration) {
	if snapshot.Status == "completed" && snapshot.PendingApproval == nil && snapshot.PendingInput == nil && snapshot.Error == "" {
		fmt.Fprintf(w, "\n%sOffGrid%s\n\n%s\n\n", style.Bold, style.Reset, TerminalSafe(snapshot.Output))
		fmt.Fprintf(progress, "%sCompleted in %.1fs%s\n", style.Muted, elapsed.Seconds(), style.Reset)
	} else {
		fmt.Fprintln(w)
		RenderAgentSnapshot(w, snapshot)
		fmt.Fprintln(w)
	}
	if snapshot.RunID != "" && !AgentSessionOutput(w) {
		fmt.Fprintf(progress, "%sInspect: offgrid agent status %s%s\n", style.Muted, TerminalSafe(snapshot.RunID), style.Reset)
	}
}
