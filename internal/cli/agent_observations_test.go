package cli

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/takuphilchan/offgrid-llm/internal/agents"
)

func TestObservedDirectoryResultsPreserveNamesAndPartialTruth(t *testing.T) {
	payload := map[string]any{"operation": "file_list", "receipt": map[string]any{"state": "complete", "result": map[string]any{"host": "host", "path": "/selected", "revision": strings.Repeat("a", 64), "complete": false, "next_offset": 128, "entries": []map[string]any{{"name": "café 日本語.md"}, {"name": "reports", "directory": true}, {"name": "\x1b[31mred\u202e"}}}}}
	data, _ := json.Marshal(payload)
	step := agents.Step{ID: 1, Type: "action", ToolName: "workspace_list", ToolResult: string(data)}
	var out bytes.Buffer
	seen := map[string]bool{}
	RenderExecutionObservations(&out, []agents.Step{step}, seen)
	RenderExecutionObservations(&out, []agents.Step{step}, seen)
	if strings.Count(out.String(), "Observed files") != 1 || !strings.Contains(out.String(), "café 日本語.md") || !strings.Contains(out.String(), "reports/") || !strings.Contains(out.String(), "Partial listing") || strings.ContainsAny(out.String(), "\x1b\u202e") {
		t.Fatal(out.String())
	}
	for _, invalid := range []string{`{}`, `{"receipt":{"state":"rejected"}}`, `{"receipt":{"state":"complete","result":{"entries":null}}}`, `{"operation":"file_list","operation":"other"}`} {
		out.Reset()
		step.ToolResult = invalid
		RenderExecutionObservations(&out, []agents.Step{step}, map[string]bool{})
		if out.Len() != 0 {
			t.Fatal("manufactured evidence", out.String())
		}
	}
}

func TestObservedLastPageDoesNotClaimItIncludesEarlierEntries(t *testing.T) {
	for _, receiptOffset := range []bool{false, true} {
		result := map[string]any{"host": "host", "path": "/selected", "revision": strings.Repeat("a", 64), "complete": true, "entries": []map[string]any{{"name": "last.txt"}}}
		if receiptOffset {
			result["offset"] = 2
		}
		data, _ := json.Marshal(map[string]any{"operation": "file_list", "receipt": map[string]any{"state": "complete", "result": result}})
		var out bytes.Buffer
		RenderExecutionObservations(&out, []agents.Step{{Type: "action", ToolName: "workspace_list", ToolArgs: `{"offset":2}`, ToolResult: string(data)}}, map[string]bool{})
		if !strings.Contains(out.String(), "Continuation page from entry 2") || !strings.Contains(out.String(), "earlier entries are not repeated") {
			t.Fatal(out.String())
		}
	}
}
