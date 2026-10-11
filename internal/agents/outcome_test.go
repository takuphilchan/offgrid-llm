package agents

import "testing"

func TestTaskOutcome(t *testing.T) {
	for _, tt := range []struct {
		name string
		task Task
		want string
	}{
		{"poem or instructions without work", Task{Status: TaskCompleted, Config: AgentConfig{TaskFirst: true}, Result: "Here is an answer"}, "answer_only"},
		{"rejected access then historical prose", Task{Status: TaskCompleted, Config: AgentConfig{TaskFirst: true}, Steps: []Step{{Type: "rejected"}}}, "answer_only"},
		{"plan is not execution", Task{Status: TaskCompleted, Config: AgentConfig{TaskFirst: true}, Steps: []Step{{Type: "task_metadata"}, {Type: "input_resolved"}}}, "answer_only"},
		{"executed read", Task{Status: TaskCompleted, Config: AgentConfig{TaskFirst: true}, Steps: []Step{{Type: "action", ToolName: "read_file"}}}, ""},
		{"unknown old evidence", Task{Status: TaskCompleted}, ""},
		{"interrupted", Task{Status: TaskInterrupted, Config: AgentConfig{TaskFirst: true}}, ""},
		{"children", Task{Status: TaskCompleted, Config: AgentConfig{TaskFirst: true}, ChildHistory: []ChildLink{{}}}, ""},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if got := TaskOutcome(&tt.task); got != tt.want {
				t.Fatalf("got %q want %q", got, tt.want)
			}
		})
	}
}
