package cli

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"unicode/utf8"

	"github.com/takuphilchan/offgrid-llm/internal/agents"
	"github.com/takuphilchan/offgrid-llm/internal/execution"
	"github.com/takuphilchan/offgrid-llm/internal/jsonwire"
)

// These are committed observations, separate from the generated explanation.
func renderProcessObservation(w io.Writer, step agents.Step, seen map[string]bool) bool {
	operation := execution.Operation(step.ToolName)
	if step.Type != "action" || !strings.HasPrefix(operation, "process_") || len(step.ToolResult) > 256<<10 || jsonwire.Validate([]byte(step.ToolResult)) != nil {
		return false
	}
	var view struct {
		Host, Session, Operation string
		Receipt                  struct {
			State  string
			Result json.RawMessage
		}
	}
	var process execution.ProcessPage
	if json.Unmarshal([]byte(step.ToolResult), &view) != nil || view.Host == "" || view.Session == "" || view.Operation != operation || view.Receipt.State != "complete" && view.Receipt.State != "dispatched" || json.Unmarshal(view.Receipt.Result, &process) != nil || process.Handle == "" || len(process.Output) > execution.MaxPageBytes {
		return false
	}
	switch process.State {
	case "running":
	case "exited", "failed", "cancelled", "timed_out":
		if process.ExitCode == nil {
			return false
		}
	default:
		return false
	}
	digest := sha256.Sum256([]byte(step.ToolResult))
	key := hex.EncodeToString(digest[:])
	if seen[key] {
		return true
	}
	seen[key] = true
	fmt.Fprintf(w, "Observed command · %s · %s", TerminalSafe(view.Host), process.State)
	if process.ExitCode != nil {
		fmt.Fprintf(w, " · exit code %d", *process.ExitCode)
	}
	fmt.Fprintln(w)
	if process.Directory != "" {
		fmt.Fprintf(w, "  Directory: %s\n", TerminalSafe(process.Directory))
	}
	if len(process.Output) > 0 {
		if utf8.Valid(process.Output) {
			for _, line := range strings.Split(strings.TrimSuffix(string(process.Output), "\n"), "\n") {
				fmt.Fprintf(w, "  %s\n", TerminalSafe(line))
			}
		} else {
			fmt.Fprintf(w, "  %d raw output bytes; inspect the saved receipt for binary data.\n", len(process.Output))
		}
	}
	if process.OutputTruncated || process.NextOffset != nil {
		fmt.Fprintln(w, "  Partial output; inspect the saved process handle for remaining captured bytes.")
	}
	return true
}
