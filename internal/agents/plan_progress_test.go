package agents

import (
	"github.com/takuphilchan/offgrid-llm/pkg/api"
	"testing"
)

func TestPlanningRequiresProgressBeforeAnotherSuccessfulUpdate(t *testing.T) {
	for _, tc := range []struct {
		name string
		last api.ChatMessage
		want bool
	}{
		{"initial", api.ChatMessage{Role: "user", Content: "Plan the work"}, true},
		{"saved", api.ChatMessage{Role: "tool", Name: "task_plan", Content: `{"saved":true,"items":[]}`}, false},
		{"invalid", api.ChatMessage{Role: "tool", Name: "task_plan", Content: `{"error":"invalid_task_operation"}`}, true},
		{"observed", api.ChatMessage{Role: "tool", Name: "workspace_read", Content: `{"text":"source"}`}, true},
		{"steered", api.ChatMessage{Role: "user", Content: "Change the plan"}, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			task := &Task{Config: taskConfig(), Checkpoint: &Checkpoint{Messages: []api.ChatMessage{tc.last}}}
			found := false
			for _, tool := range runtimeTools(task) {
				if tool.Function.Name == "task_plan" {
					found = true
				}
			}
			if found != tc.want {
				t.Fatal("incorrect planning availability", found)
			}
			if task.Checkpoint.Messages[0].StringContent() != tc.last.StringContent() {
				t.Fatal("history mutated")
			}
		})
	}
}
