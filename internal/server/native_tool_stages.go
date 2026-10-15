package server

import (
	"encoding/json"

	"github.com/takuphilchan/offgrid-llm/internal/agents"
)

// Model discovery follows durable host evidence. These are planning hints, not
// grants: dispatch and the host still independently validate every operation.
func desktopPlanningState(task *agents.Task) (catalog, windows, selected bool) {
	for i, step := range task.Steps {
		if i < task.Config.ComputerStepOffset || step.Type != "action" {
			continue
		}
		switch step.ToolName {
		case "computer_applications":
			var view struct {
				Applications []json.RawMessage `json:"applications"`
			}
			catalog = json.Unmarshal([]byte(step.ToolResult), &view) == nil && len(view.Applications) > 0
		case "computer_windows":
			var view struct {
				Windows []json.RawMessage `json:"windows"`
			}
			windows = json.Unmarshal([]byte(step.ToolResult), &view) == nil && len(view.Windows) > 0
		case "computer_open_application":
			selected = false
		case "computer_focus_window":
			selected = true // acknowledgment permits a fresh observation, never input
		case "computer_verify_application":
			var view struct {
				Verified bool `json:"verified"`
				Target   *struct {
					ID string `json:"id"`
				} `json:"target"`
			}
			selected = json.Unmarshal([]byte(step.ToolResult), &view) == nil && view.Verified && view.Target != nil && view.Target.ID != ""
		case "computer_observe", "computer_documents":
			var view struct {
				Observation struct {
					ID string `json:"id"`
				} `json:"observation"`
			}
			selected = json.Unmarshal([]byte(step.ToolResult), &view) == nil && view.Observation.ID != ""
		}
	}
	return
}

func desktopPlanningTool(task *agents.Task, name string) bool {
	catalog, windows, selected := desktopPlanningState(task)
	pending := len(agents.PendingApplicationLaunches(task)) > 0
	switch name {
	case applicationSearchTool:
		return catalog
	case "computer_applications", "computer_windows":
		return true
	case "computer_open_application":
		_, err := resolvedApplicationLookup(task)
		return catalog && !pending && err == nil
	case "computer_verify_application":
		return pending || (task.Checkpoint != nil && task.Checkpoint.NativeReadRepair != nil)
	case "computer_focus_window":
		return windows
	default:
		return selected && !pending
	}
}

// A negotiated document host must inspect its current editor before proposing
// generic control input. Otherwise small models can choose Enter/whole-value
// replacement instead of creating an independent document. Ordinary forms keep
// legacy controls after the trusted provider explicitly reports no document
// support. A failed/incomplete/stale read is not evidence of that limitation.
func nativeDocumentControlFallback(task *agents.Task) bool {
	for i := len(task.Steps) - 1; i >= task.Config.ComputerStepOffset; i-- {
		step := task.Steps[i]
		if step.Type != "action" && !(step.Type == "tool_error" && step.ToolName == "computer_documents") {
			continue
		}
		switch step.ToolName {
		case "computer_open_application", "computer_focus_window", "computer_windows":
			return false
		case "computer_documents":
			var result struct {
				Code string `json:"error"`
			}
			return json.Unmarshal([]byte(step.ToolResult), &result) == nil && result.Code == "computer_document_unavailable"
		}
	}
	return false
}
