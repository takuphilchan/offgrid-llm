package server

import (
	"encoding/json"

	"github.com/takuphilchan/offgrid-llm/internal/agents"
	"github.com/takuphilchan/offgrid-llm/internal/computer"
)

// A supported document protocol can discover that the selected surface is an
// ordinary form rather than a supported editor. This is a capability result,
// not a failed mutation or evidence that document text may be replaced.
const unavailableDocumentCapabilityResult = `{"status":"unavailable","capability":"native_documents","supported":false,"code":"computer_document_unavailable","effects":"none","message":"This surface has no supported document operations. Observe its controls again before choosing a supported control operation. Do not use a control edit to imitate creating or editing a document."}`

func isUnavailableDocumentCapability(step agents.Step) bool {
	if step.ToolName != "computer_documents" || (step.Type != "action" && step.Type != "tool_error") {
		return false
	}
	var result struct {
		Status     string `json:"status"`
		Capability string `json:"capability"`
		Supported  *bool  `json:"supported"`
		Code       string `json:"code"`
		Effects    string `json:"effects"`
	}
	if json.Unmarshal([]byte(step.ToolResult), &result) != nil || result.Code != "computer_document_unavailable" || result.Effects != "none" {
		return false
	}
	if step.Type == "tool_error" {
		// Older runner versions persisted a real read error in this shape. An
		// explicit resume may inspect it; an upgrade never resumes the task.
		return result.Status == "failed"
	}
	return result.Status == "unavailable" && result.Capability == "native_documents" && result.Supported != nil && !*result.Supported
}

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
			if isUnavailableDocumentCapability(step) {
				// Lack of document support neither loses the selected application
				// nor authorizes input. A subsequent control observation is needed.
				continue
			}
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
// Bind the negative result to the prior canonical target and discard it when a
// later observation replaces that target, even without an explicit focus call.
func nativeDocumentControlFallback(task *agents.Task) bool {
	if task == nil || task.Config.ComputerStepOffset < 0 || task.Config.ComputerStepOffset > len(task.Steps) {
		return false
	}
	var observed, unsupported computer.TargetIdentity
	for i := task.Config.ComputerStepOffset; i < len(task.Steps); i++ {
		step := task.Steps[i]
		if step.Type != "action" && !(step.Type == "tool_error" && step.ToolName == "computer_documents") {
			continue
		}
		switch step.ToolName {
		case "computer_open_application", "computer_focus_window", "computer_windows":
			observed, unsupported = computer.TargetIdentity{}, computer.TargetIdentity{}
		case "computer_verify_application":
			observed, unsupported = computer.TargetIdentity{}, computer.TargetIdentity{}
			var evidence computer.ApplicationEvidence
			if decodeNativeModelResult(step.ToolResult, &evidence) && evidence.Verified && evidence.Target != nil && evidence.View != nil && *evidence.Target == evidence.View.Observation.Target && evidence.Target.Validate() == nil {
				observed = *evidence.Target
			}
		case "computer_observe":
			var view computer.NativeView
			if !decodeNativeModelResult(step.ToolResult, &view) || view.Observation.ID == "" || view.Observation.Target.Validate() != nil {
				observed, unsupported = computer.TargetIdentity{}, computer.TargetIdentity{}
				continue
			}
			observed = view.Observation.Target
			if observed != unsupported {
				unsupported = computer.TargetIdentity{}
			}
		case "computer_documents":
			unsupported = computer.TargetIdentity{}
			if isUnavailableDocumentCapability(step) && observed.Validate() == nil {
				unsupported = observed
			} else {
				observed = computer.TargetIdentity{}
				var inventory computer.DocumentInventory
				if step.Type == "action" && decodeNativeModelResult(step.ToolResult, &inventory) && inventory.Complete && inventory.Validate(inventory.Observation.Target, inventory.Observation.CapturedAt) == nil {
					observed = inventory.Observation.Target
				}
			}
		}
	}
	return unsupported.Validate() == nil && unsupported == observed
}
