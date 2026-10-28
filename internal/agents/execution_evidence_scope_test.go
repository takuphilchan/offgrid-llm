package agents

import (
	"encoding/json"
	"testing"

	"github.com/takuphilchan/offgrid-llm/internal/execution"
)

func TestHistoricalExecutionIdentityRequiresContinuousServiceTransitions(t *testing.T) {
	old := execution.Reference{Host: "host", Session: "old-session", Grant: "old-grant", Revision: 1}
	current := execution.Reference{Host: "host", Session: "new-session", Grant: "new-grant", Revision: 1}
	for _, kind := range []string{"valid", "unrelated", "foreign-host", "missing", "tool-result"} {
		t.Run(kind, func(t *testing.T) {
			prior, next := old, current
			if kind == "unrelated" {
				next.Session = "unrelated"
			}
			if kind == "foreign-host" {
				prior.Host = "another"
			}
			var previous *execution.Reference = &prior
			if kind == "missing" {
				previous = nil
			}
			data, _ := json.Marshal(map[string]any{"status": "workspace_attached", "execution": next, "previous_execution": previous})
			task := &Task{Config: AgentConfig{Execution: &current, ExecutionRequestVersion: 1}, Steps: []Step{{Type: "action"}, {Type: "input_resolved", ToolName: execution.AccessRequestTool, ToolResult: string(data)}}}
			if kind == "tool-result" {
				task.Steps[1].Type = "action"
			}
			ref, ok := executionReferenceAtStep(task, 0)
			if kind == "valid" {
				if !ok || ref != old {
					t.Fatal(ref, ok)
				}
			} else if ok && ref == old {
				t.Fatal("untrusted transition accepted")
			}
			if *task.Config.Execution != current {
				t.Fatal("historical inspection changed active authority")
			}
		})
	}
}
