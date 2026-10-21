package agents

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"reflect"

	"github.com/takuphilchan/offgrid-llm/pkg/api"
)

// prepareNativeActionCheck is metadata only. The provider validates canonical
// typed arguments against trusted observations; authorization and the host's
// independent freshness/authority checks still run after this is persisted.
func (r *Runner) prepareNativeActionCheck(task *Task, call api.ToolCall) error {
	if !UsesNativeActionContract(task) {
		return nil
	}
	deriver, ok := r.tools.(interface {
		DeriveNativeActionExpectation(*Task, api.ToolCall) (*OutcomeExpectation, error)
	})
	if !ok {
		return nil // Existing explicit-check guards still fail closed.
	}
	if call.ID == "" || len(call.ID) > 128 || task.Checkpoint == nil || task.Checkpoint.ExecutingCall != "" {
		return fmt.Errorf("action has no safe pending identity")
	}
	copyTask := *task
	if err := initializeWorkflow(&copyTask); err != nil {
		return err
	}
	if copyTask.Workflow == nil {
		return ErrWorkflowVersion
	}
	// Bindings and predicate pointers must not alias existing evidence if any
	// later validation fails. The versioned serializer also enforces its bound.
	encoded, err := json.Marshal(copyTask.Workflow)
	if err != nil {
		return err
	}
	var workflow WorkflowEvidence
	if err := json.Unmarshal(encoded, &workflow); err != nil {
		return err
	}
	copyTask.Workflow = &workflow
	expectation, err := deriver.DeriveNativeActionExpectation(&copyTask, call)
	if err != nil || expectation == nil {
		if err == nil {
			for _, old := range workflow.Checks {
				if old.ActionDerivedCallID == call.ID {
					return fmt.Errorf("pending action no longer has a validated predicate")
				}
			}
		}
		return err
	}
	e := *expectation
	if e.Text != nil {
		value := *e.Text
		e.Text = &value
	}
	if e.Checked != nil {
		value := *e.Checked
		e.Checked = &value
	}
	e.ID = fmt.Sprintf("action-%x", sha256.Sum256([]byte(call.ID)))[:55]
	if e.Title == "" {
		e.Title = "Verify typed action"
	}
	for _, old := range workflow.Checks {
		prior, proposed := old.Expectation, e
		prior.ID, prior.Title, proposed.ID, proposed.Title = "", "", "", ""
		if old.ActionDerivedCallID == call.ID {
			if !reflect.DeepEqual(prior, proposed) {
				return fmt.Errorf("pending action identity cannot change its predicate")
			}
			return nil // Same invocation; ordinary authorization rechecks freshness.
		}
		if old.ActionDerivedCallID != "" {
			continue
		}
		if reflect.DeepEqual(prior, proposed) {
			return nil // Explicit checks retain their current-state semantics.
		}
		if old.State != "verified" && sameNativeCheckObject(prior, proposed) {
			return fmt.Errorf("the action conflicts with an outstanding declared check; preserve it or request user steering")
		}
	}
	if err := declareExpectations(&copyTask, []OutcomeExpectation{e}, "model_proposed"); err != nil {
		return err
	}
	workflow.Checks[len(workflow.Checks)-1].ActionDerivedCallID = call.ID
	if binder, ok := r.tools.(interface{ BindWorkflowExpectations(*Task) error }); ok {
		if err := binder.BindWorkflowExpectations(&copyTask); err != nil {
			return err
		}
	}
	task.Workflow = copyTask.Workflow
	return nil
}

func sameNativeCheckObject(a, b OutcomeExpectation) bool {
	if a.Kind != b.Kind {
		return false
	}
	switch a.Kind {
	case "application_open":
		return a.ApplicationID == b.ApplicationID
	case "document_blank":
		return a.TargetID == b.TargetID && a.Command == b.Command
	case "document_edit_matches":
		return a.TargetID == b.TargetID && a.Document == b.Document && a.Snapshot == b.Snapshot
	case "control_text_equals", "control_checked_equals":
		return a.TargetID == b.TargetID && a.Element == b.Element
	}
	return false
}

// Only the trusted pre-effect rejection path may discard a generated pending
// check. Explicit checks and dispatched/uncertain actions remain immutable.
func discardRejectedActionCheck(task *Task, callID string) {
	if !UsesNativeActionContract(task) || task.Workflow == nil || task.Checkpoint == nil || callID == "" || task.Checkpoint.ExecutingCall != "" {
		return
	}
	for _, step := range task.Steps {
		if step.CallID == callID && (step.Type == "action" || step.Type == "reconciled") {
			return
		}
	}
	checks := task.Workflow.Checks[:0]
	for _, check := range task.Workflow.Checks {
		dispatched := check.Binding != nil && check.Binding.Document != nil && check.Binding.Document.ActionID != ""
		if check.ActionDerivedCallID == callID && !dispatched && check.State == "pending" && len(check.EvidenceSteps) == 0 {
			continue
		}
		checks = append(checks, check)
	}
	task.Workflow.Checks = checks
}
