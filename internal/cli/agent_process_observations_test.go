package cli

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/takuphilchan/offgrid-llm/internal/agents"
	"github.com/takuphilchan/offgrid-llm/internal/execution"
)

func TestObservedCommandReportsRealExitAndSafeTextOnce(t *testing.T) {
	exit := 7
	process := execution.ProcessPage{ProcessResult: execution.ProcessResult{Handle: "owned", Directory: "/selected", State: "failed", ExitCode: &exit, Output: []byte("FAILED café 日本語\n\x1b[31mred\u202e"), OutputTruncated: true}}
	result, _ := json.Marshal(process)
	view, _ := json.Marshal(map[string]any{"host": "host", "session": "session", "operation": "process_inspect", "receipt": execution.Receipt{State: "complete", Result: result}})
	step := agents.Step{Type: "action", ToolName: "workspace_process_output", ToolResult: string(view)}
	var out bytes.Buffer
	seen := map[string]bool{}
	RenderExecutionObservations(&out, []agents.Step{step}, seen)
	RenderExecutionObservations(&out, []agents.Step{step}, seen)
	if strings.Count(out.String(), "Observed command") != 1 || !strings.Contains(out.String(), "exit code 7") || !strings.Contains(out.String(), "FAILED café 日本語") || !strings.Contains(out.String(), "Partial output") || strings.ContainsAny(out.String(), "\x1b\u202e") {
		t.Fatal(out.String())
	}
	process.ExitCode = nil
	result, _ = json.Marshal(process)
	bad, _ := json.Marshal(map[string]any{"host": "host", "session": "session", "operation": "process_inspect", "receipt": execution.Receipt{State: "complete", Result: result}})
	out.Reset()
	step.ToolResult = string(bad)
	RenderExecutionObservations(&out, []agents.Step{step}, map[string]bool{})
	if out.Len() != 0 {
		t.Fatal("terminal state without exit evidence was presented", out.String())
	}
}
