package server

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/takuphilchan/offgrid-llm/internal/agents"
	"github.com/takuphilchan/offgrid-llm/internal/computer"
)

func TestSelectedWindowSchemaPinsObservedReferencesWithoutMutatingTools(t *testing.T) {
	view := desktopContextFixture(computer.TargetIdentity{ID: "surface", OSSession: "os", ProcessGeneration: "process", Surface: "window", Driver: "windows-uia"}, "Registered app")
	raw, _ := json.Marshal(view)
	task := &agents.Task{Config: agents.AgentConfig{TaskFirst: true, ComputerSession: "session", ComputerDriver: "windows-uia"}, Steps: []agents.Step{{ID: 1, Type: "action", ToolName: "computer_observe", ToolResult: string(raw)}}}
	tool := nativeComputerTools()[1]
	before, _ := json.Marshal(tool)
	got := selectedWindowToolReferences(task, tool)
	encoded, _ := json.Marshal(got)
	if !strings.Contains(string(encoded), `"enum":["search-field"]`) || !strings.Contains(string(encoded), `"enum":["view"]`) {
		t.Fatalf("missing reference constraints: %s", encoded)
	}
	after, _ := json.Marshal(tool)
	if string(before) != string(after) || task.Steps[0].ToolResult != string(raw) {
		t.Fatal("mutated shared schema or evidence")
	}
	task.Steps = append(task.Steps, agents.Step{ID: 2, Type: "action", ToolName: "computer_replace_text", ToolArgs: `{"observation_id":"view","element":"search-field","text":"tourism"}`})
	stale, _ := json.Marshal(selectedWindowToolReferences(task, tool))
	if string(stale) != string(before) {
		t.Fatal("pinned invalidated references")
	}
	task.Steps = task.Steps[:1]
	task.Config.ComputerScope = "desktop"
	desktop, _ := json.Marshal(selectedWindowToolReferences(task, tool))
	if string(desktop) != string(before) {
		t.Fatal("canonical references mixed with desktop aliases")
	}
	task.Config.ComputerScope = ""
	task.Config.ComputerStepOffset = 1
	previous, _ := json.Marshal(selectedWindowToolReferences(task, tool))
	if string(previous) != string(before) {
		t.Fatal("reused prior session")
	}
}
