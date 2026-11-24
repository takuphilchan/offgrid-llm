package agents

import (
	"encoding/hex"
	"fmt"
	"time"
)

const maxModelSwitches = 2

// ModelSwitch records a decision boundary, not a successful model load or
// qualification. The normal inference lease must revalidate the destination pin.
type ModelSwitch struct {
	Version   int       `json:"version"`
	From      string    `json:"from"`
	To        string    `json:"to"`
	Reason    string    `json:"reason"`
	Iteration int       `json:"iteration"`
	At        time.Time `json:"at"`
}

// Called only for an owned DecisionValidationError before any call was queued.
// rejectModelDecision persists the repair, selection and switch together before
// execution can prepare context or call another model. Child switching also
// requires the manager's verified ownership and atomic shared-budget admission.
func adaptRejectedDecision(task *Task) {
	adaptRejectedGraphDecision(task, false)
}

func adaptRejectedGraphDecision(task *Task, childAuthorized bool) {
	cp, selection := task.Checkpoint, task.Config.ModelSelection
	if selection == nil || selection.Mode != "adaptive" || task.ParentID != "" && !childAuthorized || !modelSwitchBoundarySafe(task) {
		return
	}
	if !validSwitchSelection(task) {
		task.Status, task.ErrorCode = TaskInterrupted, "model_policy_invalid"
		task.Error = "The saved model policy or switch history no longer matches this task. No fallback inference was started."
		setRunPhase(task, "interrupted", "")
		return
	}
	if len(cp.ModelSwitches) >= maxModelSwitches {
		return
	}
	used := map[string]bool{selection.Primary: true, task.Model: true}
	for _, entry := range cp.ModelSwitches {
		used[entry.From], used[entry.To] = true, true
	}
	for _, candidate := range selection.Candidates {
		if used[candidate.Model] {
			continue
		}
		// Switching must not expand the measured tool boundary, even if the
		// profile's authority would permit a broader tool. Keep identical scopes.
		if candidate.ToolAdapter.Scope != task.Config.ToolAdapter.Scope || candidate.ToolAdapter.CapabilitySHA256 != task.Config.ToolAdapter.CapabilitySHA256 {
			task.Status, task.ErrorCode = TaskInterrupted, "model_switch_scope_mismatch"
			task.Error = "The next configured fallback has a different measured tool scope. No fallback inference was started."
			setRunPhase(task, "interrupted", "")
			return
		}
		now := time.Now().UTC()
		cp.ModelSwitches = append(cp.ModelSwitches, ModelSwitch{Version: 1, From: task.Model, To: candidate.Model, Reason: "invalid_model_decision", Iteration: cp.Iteration, At: now})
		from := task.Model
		task.Model, task.Config.ContextWindow = candidate.Model, candidate.ContextWindow
		pin := *candidate.ToolAdapter
		task.Config.ToolAdapter = &pin
		if cp.InputBudget != nil {
			cp.InputBudget.ModelWindow = candidate.ContextWindow
		}
		if task.Context != nil {
			// Token measurements from a different tokenizer cannot calibrate
			// this model. Preserve history/compaction, then measure again.
			task.Context.MeasuredPromptTokens, task.Context.LastBytes = 0, 0
		}
		selection.Selected, selection.Reason = candidate.Model, "invalid_model_decision"
		task.Steps = append(task.Steps, Step{ID: len(task.Steps) + 1, Type: "model_switch", Content: fmt.Sprintf("Switched from %s to configured local fallback %s after a model decision was rejected before dispatch. Runtime qualification will be checked again; existing effects and budgets are unchanged.", from, candidate.Model), Timestamp: now})
		return
	}
}

func modelSwitchBoundarySafe(task *Task) bool {
	cp := task.Checkpoint
	return task.Status == TaskRunning && cp != nil && cp.ExecutingCall == "" && cp.ReadOnlyCall == "" && len(cp.Calls) == 0 && cp.ComputerReceipt == nil && cp.ExecutionIntent == nil && cp.ExecutionReceipt == nil && task.PendingApproval == nil && task.PendingInput == nil && !task.ProcessStopPending && !task.ControlPending && len(task.PendingInstructions) == 0
}

func validSwitchSelection(task *Task) bool {
	selection, cp := task.Config.ModelSelection, task.Checkpoint
	if cp.InputBudget != nil && cp.InputBudget.modelWindow() != task.Config.ContextWindow {
		return false
	}
	if selection.Version != 1 || selection.Selected != task.Model || len(selection.Candidates) < 1 || len(selection.Candidates) > 3 || selection.Candidates[0].Model != selection.Primary || len(cp.ModelSwitches) > maxModelSwitches {
		return false
	}
	seen, selected := map[string]bool{}, false
	for _, candidate := range selection.Candidates {
		pin := candidate.ToolAdapter
		if !localModelReference(candidate.Model) || seen[candidate.Model] || candidate.ContextWindow < 1 || pin == nil || ValidateAgentToolScope(pin) != nil || !modelPinDigest(pin.EvidenceID) || !modelPinDigest(pin.FingerprintID) {
			return false
		}
		seen[candidate.Model] = true
		if candidate.Model == task.Model {
			selected = task.Config.ToolAdapter != nil && *task.Config.ToolAdapter == *pin && task.Config.ContextWindow == candidate.ContextWindow
		}
	}
	prior, iteration := "", 0
	used := map[string]bool{}
	for _, entry := range cp.ModelSwitches {
		if entry.Version != 1 || entry.Reason != "invalid_model_decision" || entry.At.IsZero() || !seen[entry.From] || !seen[entry.To] || entry.From == entry.To || entry.To == selection.Primary || used[entry.To] || entry.Iteration <= iteration || entry.Iteration > cp.Iteration || prior != "" && entry.From != prior {
			return false
		}
		used[entry.From], used[entry.To] = true, true
		prior, iteration = entry.To, entry.Iteration
	}
	return selected && (prior == "" || prior == task.Model)
}

// Explicit Resume does not replenish the same unresolved adaptive correction
// sequence. Nor may restart reach counting with mismatched selection evidence.
func (r *Runner) adaptiveModelBoundary(task *Task) (bool, error) {
	if stopped, err := r.modelGraphBoundary(task); stopped || err != nil {
		return stopped, err
	}
	selection := task.Config.ModelSelection
	if selection == nil || selection.Mode != "adaptive" {
		return false, nil
	}
	code, message := "", ""
	if !validSwitchSelection(task) {
		code, message = "model_policy_invalid", "The saved model policy or switch history no longer matches this task. No inference was started."
	} else if task.Checkpoint.DecisionRepairs+task.Checkpoint.ProposalRepairs >= maxProposalRepairs {
		code, message = "model_decision_repair_limit", "Paused after repeated invalid model decisions. The saved correction budget is exhausted; resuming does not replenish it. Earlier work remains saved."
	}
	if code == "" {
		return false, nil
	}
	task.Status, task.ErrorCode, task.Error = TaskInterrupted, code, message
	setRunPhase(task, "interrupted", "")
	return true, r.persist(task)
}

func modelPinDigest(value string) bool {
	decoded, err := hex.DecodeString(value)
	return err == nil && len(decoded) == 32 && hex.EncodeToString(decoded) == value
}
