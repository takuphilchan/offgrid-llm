package server

import (
	"encoding/json"

	"github.com/takuphilchan/offgrid-llm/internal/agents"
)

// Stored state flags do not establish historical verification. Rebuild it from
// the successful source call and the later independent observation on each
// assessment. Source IDs come from the runner, never model-supplied evidence.
func derivedActionStep(task *agents.Task, check *agents.OutcomeCheck) int {
	if !agents.UsesNativeActionContract(task) || check.ActionDerivedCallID == "" || task.Config.ComputerStepOffset < 0 || task.Config.ComputerStepOffset > len(task.Steps) {
		return 0
	}
	for index := task.Config.ComputerStepOffset; index < len(task.Steps); index++ {
		s := task.Steps[index]
		if s.CallID != check.ActionDerivedCallID {
			continue
		}
		if s.ID != index+1 || s.Type != "action" {
			return 0
		}
		e := check.Expectation
		switch e.Kind {
		case "application_open":
			var a struct {
				Application string `json:"application_id"`
			}
			if s.ToolName == "computer_open_application" && json.Unmarshal([]byte(s.ToolArgs), &a) == nil && a.Application == e.ApplicationID {
				return s.ID
			}
		case "document_blank", "document_edit_matches":
			a, valid := decodeDocumentEditArguments(s.ToolName, []byte(s.ToolArgs))
			if valid && e.Kind == "document_blank" && s.ToolName == "computer_create_document" && a.Command == e.Command {
				return s.ID
			}
			if valid && e.Kind == "document_edit_matches" && documentEditModeForTool(s.ToolName) == e.EditMode && a.Document == e.Document && a.Snapshot == e.Snapshot && a.Text != nil && e.Text != nil && *a.Text == *e.Text {
				return s.ID
			}
		case "control_text_equals", "control_checked_equals":
			var a struct {
				Observation string  `json:"observation_id"`
				Element     string  `json:"element"`
				Text        *string `json:"text"`
				Checked     *bool   `json:"checked"`
			}
			if json.Unmarshal([]byte(s.ToolArgs), &a) != nil || check.Binding == nil || a.Observation != check.Binding.Observation || a.Element != e.Element {
				return 0
			}
			if s.ToolName == "computer_replace_text" && e.Kind == "control_text_equals" && a.Text != nil && e.Text != nil && *a.Text == *e.Text {
				return s.ID
			}
			if s.ToolName == "computer_set_checked" && e.Kind == "control_checked_equals" && a.Checked != nil && e.Checked != nil && *a.Checked == *e.Checked {
				return s.ID
			}
		}
		return 0
	}
	return 0
}
