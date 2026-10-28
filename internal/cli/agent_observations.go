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

// Present committed typed observations separately from assistant prose. This
// never executes a tool, reconstructs missing evidence or rewrites an answer.
func RenderExecutionObservations(w io.Writer, steps []agents.Step, seen map[string]bool) {
	for _, step := range steps {
		if renderProcessObservation(w, step, seen) || renderWriteObservation(w, step, seen) {
			continue
		}
		if step.Type != "action" || step.ToolName != "workspace_list" || len(step.ToolResult) > 256<<10 || jsonwire.Validate([]byte(step.ToolResult)) != nil {
			continue
		}
		digest := sha256.Sum256([]byte(step.ToolResult))
		key := hex.EncodeToString(digest[:])
		if seen[key] {
			continue
		}
		var observation struct {
			Operation string `json:"operation"`
			Receipt   struct {
				State  string `json:"state"`
				Result struct {
					execution.Location
					Offset     *int               `json:"offset"`
					Entries    *[]execution.Entry `json:"entries"`
					Revision   string             `json:"revision"`
					Complete   *bool              `json:"complete"`
					NextOffset *int               `json:"next_offset"`
				} `json:"result"`
			} `json:"receipt"`
		}
		if json.Unmarshal([]byte(step.ToolResult), &observation) != nil {
			continue
		}
		page := observation.Receipt.Result
		digestBytes, err := hex.DecodeString(page.Revision)
		if err != nil || len(digestBytes) != 32 || observation.Operation != "file_list" || observation.Receipt.State != "complete" || page.Host == "" || page.Path == "" || page.Entries == nil || page.Complete == nil || len(*page.Entries) > execution.MaxEntriesPerPage {
			continue
		}
		seen[key] = true
		fmt.Fprintf(w, "Observed files · %s · %s\n", TerminalSafe(page.Host), TerminalSafe(page.Path))
		offset := 0
		if page.Offset != nil {
			offset = *page.Offset
		} else {
			var args struct {
				Offset int `json:"offset"`
			}
			if json.Unmarshal([]byte(step.ToolArgs), &args) == nil {
				offset = args.Offset
			}
		}
		if offset > 0 {
			fmt.Fprintf(w, "  Continuation page from entry %d; earlier entries are not repeated.\n", offset)
		}
		for _, entry := range *page.Entries {
			suffix := ""
			if entry.Directory {
				suffix = "/"
			}
			fmt.Fprintf(w, "  %s%s\n", TerminalSafe(entry.Name), suffix)
		}
		if len(*page.Entries) == 0 {
			fmt.Fprintln(w, "  (No entries in this observed page.)")
		}
		if !*page.Complete || page.NextOffset != nil {
			fmt.Fprintln(w, "  Partial listing — more pages remain; this is not the entire directory.")
		}
	}
}
