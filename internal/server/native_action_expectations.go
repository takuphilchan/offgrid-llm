package server

import (
	"encoding/json"
	"strings"

	"github.com/takuphilchan/offgrid-llm/internal/agents"
	"github.com/takuphilchan/offgrid-llm/internal/capabilities"
	"github.com/takuphilchan/offgrid-llm/internal/computer"
	"github.com/takuphilchan/offgrid-llm/internal/jsonwire"
	"github.com/takuphilchan/offgrid-llm/pkg/api"
)

// DeriveNativeActionExpectation is a pure pre-dispatch operation. The runner
// persists the returned predicate and its binding before authorizing any input.
// Canonical observed references, never model prose, establish the object checked.
func (b *browserRunTools) DeriveNativeActionExpectation(task *agents.Task, call api.ToolCall) (*agents.OutcomeExpectation, error) {
	if !agents.UsesNativeActionContract(task) {
		return nil, nil
	}
	name, raw := call.Function.Name, json.RawMessage(call.Function.Arguments)
	if err := b.validateNativeActionReadback(task, name); err != nil {
		return nil, err
	}
	covered := name == "computer_open_application" || name == "computer_create_document" || name == "computer_insert_text" || name == "computer_replace_selection" || name == "computer_replace_text" || name == "computer_set_checked"
	if !covered {
		return nil, nil
	}
	if task.Workflow == nil || !agents.WorkflowVersionSupported(task) || task.Config.ComputerSession == "" || task.ComputerSessionExpired || !nativeComputerDriver(task.Config.ComputerDriver) || jsonwire.Validate(raw) != nil {
		return nil, computer.ErrInvalidControl
	}
	current, _ := documentState(task)
	if pendingDocumentReadback(task, current) {
		return nil, computer.ErrDocumentChanged
	}
	e := agents.OutcomeExpectation{ID: "action-pending", Title: "Verify requested action"}
	if name == "computer_open_application" {
		var args struct {
			Application string `json:"application_id"`
		}
		if !closedActionArguments(raw, &args, "application_id") || args.Application == "" {
			return nil, computer.ErrInvalidControl
		}
		if err := validateApplicationLookup(task, args.Application); err != nil {
			return nil, err
		}
		if len(agents.PendingApplicationLaunches(task)) != 0 {
			return nil, computer.ErrInvalidControl
		}
		for _, id := range agents.VerifiedApplications(task) {
			if id == args.Application {
				return nil, computer.ErrInvalidControl
			}
		}
		e.Kind, e.ApplicationID, e.Title = "application_open", args.Application, "Verify application opened"
		return &e, nil
	}
	if computer.DocumentTool(name, raw) {
		if err := computer.ValidateDocumentArguments(name, raw); err != nil {
			return nil, err
		}
		args, valid := decodeDocumentEditArguments(name, raw)
		state, err := documentState(task)
		if !valid || err != nil || state.step == 0 {
			return nil, computer.ErrInvalidControl
		}
		e.TargetID = state.inventory.Observation.Target.ID
		if name == "computer_create_document" {
			if !nativeCreationCanAdvance(task, state) {
				return nil, computer.ErrInvalidControl
			}
			e.Kind, e.Command, e.Title = "document_blank", args.Command, "Verify new blank document"
		} else {
			if !nativeEditCanAdvance(task, state) || !documentEditCapability(state, name) {
				return nil, computer.ErrDocumentChanged
			}
			e.Kind, e.Document, e.Snapshot, e.EditMode, e.Text = "document_edit_matches", args.Document, args.Snapshot, documentEditModeForTool(name), args.Text
			e.Title = "Verify exact document edit"
		}
	} else {
		var args struct {
			Observation string  `json:"observation_id"`
			Element     string  `json:"element"`
			Text        *string `json:"text,omitempty"`
			Checked     *bool   `json:"checked,omitempty"`
		}
		field := "text"
		if name == "computer_set_checked" {
			field = "checked"
		}
		if !closedActionArguments(raw, &args, "observation_id", "element", field) {
			return nil, computer.ErrInvalidControl
		}
		view, err := savedDiscoveryView(task, args.Observation)
		if err != nil {
			return nil, err
		}
		if b.documentToolsAvailable(task) && nativeDocumentTaskIntent(task) && !nativeDocumentControlFallback(task) {
			return nil, computer.ErrInvalidControl
		}
		if nativeDocumentControlFallback(task) && !freshControlViewAfterDocumentFallback(task, view.Observation.ID) {
			return nil, computer.ErrStaleObservation
		}
		e.TargetID, e.Element, e.Text, e.Checked = view.Observation.Target.ID, args.Element, args.Text, args.Checked
		e.Kind, e.Title = "control_text_equals", "Verify exact control text"
		if name == "computer_set_checked" {
			e.Kind, e.Title = "control_checked_equals", "Verify control state"
		}
	}
	// Reuse the canonical binder and edit guard on copies; rejected proposals
	// cannot mutate previous checks or gain authority by manufacturing metadata.
	copyTask, copyWorkflow := *task, *task.Workflow
	copyWorkflow.Checks = append(append([]agents.OutcomeCheck(nil), task.Workflow.Checks...), agents.OutcomeCheck{Expectation: e})
	copyTask.Workflow = &copyWorkflow
	if err := b.BindWorkflowExpectations(&copyTask); err != nil {
		return nil, err
	}
	if err := b.ValidateWorkflowEdit(&copyTask, name, raw); err != nil {
		return nil, err
	}
	return &e, nil
}

// Reading the current value is not enough: the typed independent verification
// must certify the source action before Enter/navigation can change that value.
func (b *browserRunTools) pendingNativeControlReadback(task *agents.Task) bool {
	if !agents.UsesNativeActionContract(task) || task.Workflow == nil {
		return false
	}
	for _, check := range b.AssessWorkflow(task) {
		if check.ActionDerivedCallID == "" || check.State == "verified" || (check.Expectation.Kind != "control_text_equals" && check.Expectation.Kind != "control_checked_equals") {
			continue
		}
		for _, step := range task.Steps {
			if step.Type == "action" && step.CallID == check.ActionDerivedCallID && (step.ToolName == "computer_replace_text" || step.ToolName == "computer_set_checked") {
				return true
			}
		}
	}
	return false
}

func (b *browserRunTools) validateNativeActionReadback(task *agents.Task, name string) error {
	if !agents.UsesNativeActionContract(task) || !strings.HasPrefix(name, "computer_") {
		return nil
	}
	if effect, known := capabilities.NativeOperationEffect(name, capabilities.NativeEffectsRevision); known && effect == capabilities.NativeRead {
		return nil
	}
	state, _ := documentState(task)
	if pendingDocumentReadback(task, state) || b.pendingNativeControlReadback(task) {
		return &agents.ToolAuthorizationError{Code: "computer_readback_required", Message: "A previous native edit still requires independent read-back. Observe and verify its exact result before another input or navigation. No proposed input was dispatched."}
	}
	return nil
}

func freshControlViewAfterDocumentFallback(task *agents.Task, observation string) bool {
	for i := len(task.Steps) - 1; i >= task.Config.ComputerStepOffset; i-- {
		step := task.Steps[i]
		if isUnavailableDocumentCapability(step) {
			return false
		}
		if step.Type == "action" && step.ToolName == "computer_observe" {
			var view computer.NativeView
			return decodeNativeModelResult(step.ToolResult, &view) && view.Observation.ID == observation
		}
	}
	return false
}

func currentNativeControlView(task *agents.Task) (computer.NativeView, bool) {
	for i := len(task.Steps) - 1; i >= task.Config.ComputerStepOffset; i-- {
		step := task.Steps[i]
		if step.Type != "action" {
			continue
		}
		var view computer.NativeView
		switch step.ToolName {
		case "computer_observe":
			if !decodeNativeModelResult(step.ToolResult, &view) {
				return view, false
			}
		case "computer_verify_application":
			var evidence computer.ApplicationEvidence
			if !decodeNativeModelResult(step.ToolResult, &evidence) || evidence.View == nil {
				return view, false
			}
			view = *evidence.View
		default:
			continue
		}
		current, err := savedDiscoveryView(task, view.Observation.ID)
		if err != nil || (nativeDocumentControlFallback(task) && !freshControlViewAfterDocumentFallback(task, view.Observation.ID)) {
			return view, false
		}
		return current, true
	}
	return computer.NativeView{}, false
}

func closedActionArguments(raw []byte, target any, names ...string) bool {
	if len(raw) == 0 || len(raw) > 128<<10 || jsonwire.Validate(raw) != nil {
		return false
	}
	var fields map[string]json.RawMessage
	if json.Unmarshal(raw, &fields) != nil || len(fields) != len(names) {
		return false
	}
	for _, name := range names {
		value, exists := fields[name]
		if !exists || strings.TrimSpace(string(value)) == "null" {
			return false
		}
	}
	return decodeNativeModelResult(string(raw), target)
}

func lastNativePlanStep(task *agents.Task) int {
	last := 0
	for _, step := range task.Steps {
		if step.Type == "task_metadata" && step.ToolName == "task_plan" {
			last = step.ID
		}
	}
	return last
}

func lastNativeEvidence(check agents.OutcomeCheck) int {
	last := 0
	for _, step := range check.EvidenceSteps {
		if step > last {
			last = step
		}
	}
	return last
}

func nativeCreationCanAdvance(task *agents.Task, state savedDocumentState) bool {
	if state.step == 0 || task.Workflow == nil {
		return false
	}
	for _, check := range task.Workflow.Checks {
		if check.Expectation.Kind != "document_blank" || check.Expectation.TargetID != state.inventory.Observation.Target.ID || check.Binding == nil || check.Binding.Document == nil {
			continue
		}
		if check.State != "verified" {
			return false
		}
		if state.inventory.Active != nil && check.Binding.Document.ID == state.inventory.Active.Document {
			edited := false
			for _, other := range task.Workflow.Checks {
				edited = edited || (other.Expectation.Kind == "document_edit_matches" && other.Expectation.Document == state.inventory.Active.Document && other.State == "verified")
			}
			if !edited || lastNativePlanStep(task) <= lastNativeEvidence(check) {
				return false
			}
		}
	}
	return true
}

func nativeEditCanAdvance(task *agents.Task, state savedDocumentState) bool {
	if state.readStep == 0 || task.Workflow == nil {
		return false
	}
	for _, check := range task.Workflow.Checks {
		if check.Expectation.Kind == "document_edit_matches" && check.State == "verified" && check.Binding != nil && check.Binding.Document != nil && matchesDocumentOutcome(check.Binding, state.read.Binding) && check.Binding.Document.ExpectedSHA256 == state.read.TextSHA256 && lastNativePlanStep(task) <= lastNativeEvidence(check) {
			return false
		}
	}
	return true
}

func documentEditCapability(state savedDocumentState, name string) bool {
	if state.readStep == 0 || !state.read.Complete || state.read.Composition != "stable" {
		return false
	}
	for _, doc := range state.inventory.Documents {
		if !doc.Selected || doc.ID != state.read.Binding.Document {
			continue
		}
		switch name {
		case "computer_insert_text":
			return doc.Capabilities.InsertText && state.read.Selection != nil && state.read.Selection.Start == state.read.Selection.End
		case "computer_replace_selection":
			return doc.Capabilities.ReplaceSelection && state.read.Selection != nil && state.read.Selection.Start < state.read.Selection.End
		case "computer_replace_text":
			return doc.Capabilities.ReplaceValue
		}
	}
	return false
}
