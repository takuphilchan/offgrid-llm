package cli

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"

	"github.com/takuphilchan/offgrid-llm/internal/agents"
	"github.com/takuphilchan/offgrid-llm/internal/execution"
	"github.com/takuphilchan/offgrid-llm/internal/jsonwire"
)

// Show the observed destination on its own copyable line, not reconstructed
// from assistant prose (whose punctuation is not part of the path).
func renderWriteObservation(w io.Writer, step agents.Step, seen map[string]bool) bool {
	if step.Type != "action" || execution.Operation(step.ToolName) != "file_write" || len(step.ToolResult) > 256<<10 || jsonwire.Validate([]byte(step.ToolResult)) != nil {
		return false
	}
	var view struct {
		Host, Session, Operation string
		Receipt                  struct {
			State  string
			Result execution.WriteResult
		}
	}
	if json.Unmarshal([]byte(step.ToolResult), &view) != nil || view.Operation != "file_write" || view.Host == "" || view.Session == "" || view.Receipt.State != "complete" {
		return false
	}
	result := view.Receipt.Result
	revision, err := hex.DecodeString(result.Revision)
	if err != nil || len(revision) != 32 || !result.Verified || result.Host != view.Host || result.Path == "" || result.Bytes < 0 {
		return false
	}
	digest := sha256.Sum256([]byte(step.ToolResult))
	key := hex.EncodeToString(digest[:])
	if seen[key] {
		return true
	}
	seen[key] = true
	fmt.Fprintf(w, "\nFile written and read back (%d bytes)\n  %s\n\n", result.Bytes, TerminalSafe(result.Path))
	return true
}
