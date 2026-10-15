package server

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/takuphilchan/offgrid-llm/internal/agents"
	"github.com/takuphilchan/offgrid-llm/internal/computer"
)

var errWorkflowControlExpectationRequired = errors.New("matching control expectation required before edit")

type workflowControlExpectationError struct {
	targetID, element                     string
	document, snapshot, command, editMode string
}

func (e *workflowControlExpectationError) Error() string {
	return errWorkflowControlExpectationRequired.Error()
}
func (e *workflowControlExpectationError) Unwrap() error {
	return errWorkflowControlExpectationRequired
}

// ValidateWorkflowEdit is a read-only pre-dispatch gate. It accepts canonical
// arguments only; model aliases have already been resolved by the runner. The
// host still independently validates freshness, scope, and action policy.
func (b *browserRunTools) ValidateWorkflowEdit(t *agents.Task, name string, args json.RawMessage) error {
	if t == nil || !agents.WorkflowVersionSupported(t) {
		return computer.ErrInvalidControl
	}
	if !t.Config.TaskFirst || t.Config.NativeWorkflowVersion == 0 || !nativeComputerDriver(t.Config.ComputerDriver) {
		return nil
	}
	if name == "computer_create_document" || name == "computer_insert_text" || name == "computer_replace_selection" || (name == "computer_replace_text" && computer.DocumentTool(name, args)) {
		return validateDocumentWorkflowEdit(t, name, args)
	}
	kind := ""
	switch name {
	case "computer_replace_text":
		kind = "control_text_equals"
	case "computer_set_checked":
		kind = "control_checked_equals"
	default:
		return nil
	}
	var a struct {
		Observation string  `json:"observation_id"`
		Element     string  `json:"element"`
		Text        *string `json:"text,omitempty"`
		Checked     *bool   `json:"checked,omitempty"`
	}
	if !decodeNativeModelResult(string(args), &a) || a.Element == "" ||
		(kind == "control_text_equals" && (a.Text == nil || a.Checked != nil)) ||
		(kind == "control_checked_equals" && (a.Checked == nil || a.Text != nil)) {
		return computer.ErrInvalidControl
	}
	view, err := savedDiscoveryView(t, a.Observation)
	if err != nil {
		return err
	}
	var control *computer.NativeElement
	for i := range view.Elements {
		if view.Elements[i].ID == a.Element {
			if control != nil {
				return computer.ErrInvalidControl
			}
			control = &view.Elements[i]
		}
	}
	if control == nil {
		return computer.ErrInvalidControl
	}
	if (kind == "control_text_equals" && !control.Writable) || (kind == "control_checked_equals" && !control.Checkable) {
		return computer.ErrInvalidControl
	}
	if control.Identity != "" {
		matches := 0
		for _, e := range view.Elements {
			if e.Identity == control.Identity {
				matches++
			}
		}
		if matches != 1 {
			return computer.ErrInvalidControl
		}
	}
	if t.Workflow != nil {
		for _, c := range t.Workflow.Checks {
			e, binding := c.Expectation, c.Binding
			if e.Kind != kind || binding == nil || binding.Session != t.Config.ComputerSession ||
				e.TargetID != view.Observation.Target.ID || binding.TargetSHA256 != evidenceTargetDigest(view.Observation.Target) {
				continue
			}
			// Older hosts can use an exact observation-local binding. They still
			// cannot claim cross-observation verification without host identity.
			if binding.Identity == "" {
				if control.Identity != "" || binding.Observation != a.Observation || e.Element != a.Element {
					continue
				}
			} else if binding.Identity != control.Identity {
				continue
			}
			if (kind == "control_text_equals" && e.Text != nil && *e.Text == *a.Text) ||
				(kind == "control_checked_equals" && e.Checked != nil && *e.Checked == *a.Checked) {
				return nil
			}
		}
	}
	return &workflowControlExpectationError{targetID: view.Observation.Target.ID, element: control.ID}
}

func evidenceTargetDigest(target computer.TargetIdentity) string {
	data, _ := json.Marshal(target)
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func (b *browserRunTools) BindWorkflowExpectations(t *agents.Task) error {
	if t == nil || t.Workflow == nil || !agents.WorkflowVersionSupported(t) {
		return computer.ErrInvalidControl
	}
	// Work on copies so a rejected declaration cannot partially alter existing
	// check bindings (runtime metadata rollback retains its previous slice).
	checks := append([]agents.OutcomeCheck(nil), t.Workflow.Checks...)
	for i := range checks {
		if documentOutcomeKind(checks[i].Expectation.Kind) && checks[i].Binding == nil {
			state, err := documentState(t)
			if err != nil {
				return err
			}
			if err := bindDocumentDeclaration(t, &checks[i], state); err != nil {
				return err
			}
		}
	}
	var view computer.NativeView
	stepID := 0
	if t.Config.ComputerStepOffset < 0 || t.Config.ComputerStepOffset > len(t.Steps) {
		return fmt.Errorf("invalid evidence boundary")
	}
	for index, s := range t.Steps[t.Config.ComputerStepOffset:] {
		if s.ID != index+t.Config.ComputerStepOffset+1 {
			return fmt.Errorf("invalid evidence step identity")
		}
		if s.Type != "action" {
			continue
		}
		if s.ToolName == "computer_observe" {
			view = computer.NativeView{}
			stepID = 0
			if decodeNativeModelResult(s.ToolResult, &view) {
				stepID = s.ID
			}
		}
		if s.ToolName == "computer_verify_application" {
			var e computer.ApplicationEvidence
			view = computer.NativeView{}
			stepID = 0
			if decodeNativeModelResult(s.ToolResult, &e) && e.Verified && e.Target != nil && e.View != nil && *e.Target == e.View.Observation.Target {
				view = *e.View
				stepID = s.ID
			}
		}
		if invalidatesNativeCheck(s.ToolName, "control_text_equals") || s.ToolName == "computer_windows" {
			view = computer.NativeView{}
			stepID = 0
		}
	}
	for i := range checks {
		c := &checks[i]
		if c.Binding != nil || (c.Expectation.Kind != "control_text_equals" && c.Expectation.Kind != "control_checked_equals") {
			continue
		}
		if stepID == 0 || view.Observation.ID == "" || view.Observation.Target.Validate() != nil || view.Observation.Target.ID != c.Expectation.TargetID || view.Observation.Target.Driver != t.Config.ComputerDriver {
			return fmt.Errorf("declare control checks using the latest observed target and control")
		}
		found := false
		for _, e := range view.Elements {
			if e.ID == c.Expectation.Element {
				if found {
					return fmt.Errorf("ambiguous control identity")
				}
				found = true
				c.Binding = &agents.OutcomeBinding{Session: t.Config.ComputerSession, TargetSHA256: evidenceTargetDigest(view.Observation.Target), Identity: e.Identity, Observation: view.Observation.ID, Step: stepID}
			}
		}
		if !found {
			return fmt.Errorf("control is not present in the latest observation")
		}
		if c.Binding.Identity != "" {
			matches := 0
			for _, e := range view.Elements {
				if e.Identity == c.Binding.Identity {
					matches++
				}
			}
			if matches != 1 {
				return fmt.Errorf("ambiguous host control identity")
			}
		}
	}
	t.Workflow.Checks = checks
	return nil
}

// AssessWorkflow consumes only the owner's canonical, successful steps. It
// never accepts model-selected evidence IDs or invokes host control.
func (b *browserRunTools) AssessWorkflow(task *agents.Task) []agents.OutcomeCheck {
	checks := append([]agents.OutcomeCheck(nil), task.Workflow.Checks...)
	for i := range checks {
		c := &checks[i]
		c.State, c.Reason, c.EvidenceSteps, c.CheckedAt = "pending", "Matching independent evidence has not been recorded.", nil, nil
		if documentOutcomeKind(c.Expectation.Kind) {
			bindRecordedDocumentAction(task, c)
			assessDocumentOutcome(task, c)
			continue
		}
		switch c.Expectation.Kind {
		case "application_open", "control_text_equals", "control_checked_equals":
		default:
			c.State, c.Reason = "unavailable", "No independent verifier is implemented for this outcome kind. Application/control checks cannot establish it."
			continue
		}
		if task.Config.ComputerStepOffset < 0 || task.Config.ComputerStepOffset > len(task.Steps) {
			continue
		}
		if c.Expectation.Kind != "application_open" {
			if c.Binding == nil {
				continue
			}
			if c.Binding.Identity == "" {
				c.State, c.Reason = "unavailable", "This host did not supply cross-observation control identity. No matching by label or text is permitted."
				continue
			}
			if c.Binding.Session != task.Config.ComputerSession {
				continue
			}
		}
		var observed computer.NativeView
		observeStep := 0
		for index := task.Config.ComputerStepOffset; index < len(task.Steps); index++ {
			s := task.Steps[index]
			if s.ID != index+1 {
				continue
			}
			if s.Type == "task_metadata" || s.Type == "rejected" || s.Type == "completion_rejected" {
				continue
			}
			// Unknown/uncertain mutations are never proof, and may invalidate
			// earlier postconditions even if they did not return successfully.
			if invalidatesNativeCheck(s.ToolName, c.Expectation.Kind) {
				c.State, c.Reason, c.EvidenceSteps, c.CheckedAt = "pending", "Later native input requires another independent check.", nil, nil
				observed, observeStep = computer.NativeView{}, 0
			}
			if s.Type != "action" {
				continue
			}
			switch s.ToolName {
			case "computer_observe":
				observed, observeStep = computer.NativeView{}, 0
				if decodeNativeModelResult(s.ToolResult, &observed) && observed.Observation.Target.Driver == task.Config.ComputerDriver && observed.Observation.Target.Validate() == nil {
					observeStep = s.ID
				}
				invalidateChangedControlView(c, observed)
			case "computer_verify_application":
				var e computer.ApplicationEvidence
				var a struct {
					Application string `json:"application_id"`
				}
				if !decodeNativeModelResult(s.ToolArgs, &a) || !decodeNativeModelResult(s.ToolResult, &e) || !e.Verified || e.Target == nil || e.View == nil || e.Check != "application_open" || e.Application != a.Application || e.Target.Validate() != nil || e.Target.Driver != task.Config.ComputerDriver || *e.Target != e.View.Observation.Target || e.View.Observation.ID == "" {
					continue
				}
				observed, observeStep = *e.View, s.ID
				invalidateChangedControlView(c, observed)
				if c.Expectation.Kind == "application_open" && c.Expectation.ApplicationID == e.Application && (c.Expectation.TargetID == "" || c.Expectation.TargetID == e.Target.ID) {
					verifyOutcome(c, []int{s.ID}, s.Timestamp)
				}
			case "computer_verify", "computer_verify_checked":
				if (c.Expectation.Kind != "control_text_equals" || s.ToolName != "computer_verify") && (c.Expectation.Kind != "control_checked_equals" || s.ToolName != "computer_verify_checked") {
					continue
				}
				if c.Binding == nil || observeStep < c.Binding.Step || observeStep == 0 || observed.Observation.Target.ID != c.Expectation.TargetID {
					continue
				}
				var a struct {
					Observation string  `json:"observation_id"`
					Element     string  `json:"element"`
					Text        *string `json:"text,omitempty"`
					Checked     *bool   `json:"checked,omitempty"`
				}
				if !decodeNativeModelResult(s.ToolArgs, &a) || c.Binding.TargetSHA256 != evidenceTargetDigest(observed.Observation.Target) {
					continue
				}
				identityMatches := 0
				exact := false
				for _, e := range observed.Elements {
					if e.Identity == c.Binding.Identity {
						identityMatches++
						if e.ID == a.Element {
							exact = true
						}
					}
				}
				if !exact || identityMatches != 1 {
					continue
				}
				// Re-check only this control/predicate. An unrelated read-back
				// must not erase a previously established outcome in the task.
				c.State, c.EvidenceSteps, c.CheckedAt = "pending", nil, nil
				c.Reason = "The latest independent re-check did not establish this control state."
				// Preserve exact argument/result/target/read-back validation.
				viewJSON, _ := json.Marshal(observed)
				candidate := *task
				candidate.Steps = []agents.Step{{ToolName: "computer_observe", ToolResult: string(viewJSON)}, s}
				if validateNativeCompletion(&candidate) != nil {
					continue
				}
				matched := c.Expectation.Kind == "control_text_equals" && s.ToolName == "computer_verify" && a.Text != nil && c.Expectation.Text != nil && *a.Text == *c.Expectation.Text
				matched = matched || c.Expectation.Kind == "control_checked_equals" && s.ToolName == "computer_verify_checked" && a.Checked != nil && c.Expectation.Checked != nil && *a.Checked == *c.Expectation.Checked
				if matched {
					verifyOutcome(c, []int{observeStep, s.ID}, s.Timestamp)
				}
			}
		}
	}
	return checks
}

// A newer observation can disprove an earlier successful read-back without
// OffGrid having dispatched any input (e.g. user edits or window replacement).
func invalidateChangedControlView(c *agents.OutcomeCheck, view computer.NativeView) {
	if c.Binding == nil || c.State != "verified" {
		return
	}
	matches := 0
	valid := false
	if evidenceTargetDigest(view.Observation.Target) == c.Binding.TargetSHA256 {
		for _, e := range view.Elements {
			if e.Identity != c.Binding.Identity {
				continue
			}
			matches++
			if c.Expectation.Kind == "control_text_equals" {
				valid = e.Text != nil && !e.TextLimited && c.Expectation.Text != nil && *e.Text == *c.Expectation.Text
			}
			if c.Expectation.Kind == "control_checked_equals" {
				valid = e.Checkable && e.Checked != nil && c.Expectation.Checked != nil && *e.Checked == *c.Expectation.Checked
			}
		}
	}
	if matches != 1 || !valid {
		c.State, c.EvidenceSteps, c.CheckedAt, c.Reason = "pending", nil, nil, "Newer observation no longer establishes this control state."
	}
}

func verifyOutcome(c *agents.OutcomeCheck, steps []int, at time.Time) {
	c.State, c.Reason, c.EvidenceSteps, c.CheckedAt = "verified", "", steps, &at
}

func invalidatesNativeCheck(name, kind string) bool {
	switch name {
	case "computer_activate", "computer_shortcut":
		return true
	case "computer_replace_text", "computer_set_checked", "computer_open_application", "computer_focus_window", "computer_windows":
		return kind != "application_open"
	}
	return false
}
