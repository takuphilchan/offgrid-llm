package agents

import (
	"encoding/json"
	"sort"
	"time"

	"github.com/takuphilchan/offgrid-llm/internal/capabilities"
	"github.com/takuphilchan/offgrid-llm/pkg/api"
)

func (r *Runner) assessWorkflow(t *Task) {
	if t.Workflow == nil || !WorkflowVersionSupported(t) {
		return
	}
	if evaluator, ok := r.tools.(interface{ AssessWorkflow(*Task) []OutcomeCheck }); ok {
		t.Workflow.Checks = evaluator.AssessWorkflow(t)
	}
	verified, pending := 0, len(t.Workflow.Checks) == 0
	seen := map[string]bool{}
	_ = json.Unmarshal([]byte(t.Checkpoint.CompletionProgress), &seen)
	if seen == nil {
		seen = map[string]bool{}
	}
	progress := false
	for _, c := range t.Workflow.Checks {
		if c.State == "pending" {
			pending = true
		}
		if c.State == "verified" {
			verified++
			if !seen[c.Expectation.ID] {
				progress = true
				seen[c.Expectation.ID] = true
			}
		}
	}
	if progress {
		t.Checkpoint.CompletionRepairs = 0
		encoded, _ := json.Marshal(seen)
		t.Checkpoint.CompletionProgress = string(encoded)
	}
	t.Workflow.State = "unverified"
	if verified > 0 {
		t.Workflow.State = "partial"
	}
	if verified == len(t.Workflow.Checks) && verified > 0 {
		t.Workflow.State = "checks_passed"
	}
	if pending {
		t.Workflow.State = "pending"
	}
}

// Returns true only when completion must be repaired. No tool is dispatched
// here; the next model proposal still passes all ordinary authorization gates.
func (r *Runner) repairWorkflowCompletion(t *Task) (bool, error) {
	if !usesWorkflow(t) {
		return false, nil
	}
	if err := initializeWorkflow(t); err != nil {
		return false, err
	}
	accessUnobserved := ComputerAccessNeedsObservation(t, r.tools)
	// Authority alone is not an external outcome. A direct conversational
	// answer needs no invented application/check. Verified service artifacts
	// use service read-back evidence, not native-document expectations. Other
	// actions, an explicit expectation or a saved plan retain the evidence gate.
	if !accessUnobserved && len(t.Workflow.Checks) == 0 && len(t.Config.NativeExpectations) == 0 && len(t.Plan) == 0 {
		hasAction := false
		for _, step := range t.Steps {
			if step.Type == "action" {
				descriptor, known := r.tools.Capability(step.ToolName)
				if known && workspaceArtifactCapability(step.ToolName, descriptor) {
					var receipt struct {
						Verified bool   `json:"verified"`
						Check    string `json:"check"`
						SHA256   string `json:"sha256"`
					}
					if json.Unmarshal([]byte(step.ToolResult), &receipt) == nil && receipt.Verified && receipt.Check == "stored_bytes_sha256_and_format" && len(receipt.SHA256) == 64 {
						continue
					}
				}
			}
			if step.Type == "action" || step.Type == "reconciled" || step.Type == "tool_error" {
				hasAction = true
				break
			}
		}
		if !hasAction && t.Checkpoint.ExecutingCall == "" {
			t.Workflow = nil
			return false, nil
		}
	}
	r.assessWorkflow(t)
	remainingPlan := []string{}
	for _, item := range t.Plan {
		if item.State != "done" {
			remainingPlan = append(remainingPlan, item.Title)
		}
	}
	// The plan is advisory model interpretation. Do not make a stale plan
	// checkbox a second user-facing outcome form once a substantive action has
	// independent verification. A launch/blank document alone is readiness,
	// however, and cannot close still-recorded work.
	if len(remainingPlan) == 1 && t.Workflow.State == "checks_passed" {
		for _, check := range t.Workflow.Checks {
			if check.State == "verified" && (check.Expectation.Kind == "document_edit_matches" || check.Expectation.Kind == "control_text_equals" || check.Expectation.Kind == "control_checked_equals") {
				remainingPlan = nil
				break
			}
		}
	}
	if !accessUnobserved && t.Workflow.State != "pending" && len(remainingPlan) == 0 {
		return false, nil
	}
	remaining := []string{}
	for _, c := range t.Workflow.Checks {
		if c.State == "pending" {
			remaining = append(remaining, c.Expectation.ID)
		}
	}
	sort.Strings(remaining)
	data, _ := json.Marshal(remaining)
	feedback := "Completion is not verified. Pending check IDs: " + string(data) + ". If no checks exist, use task_expectations first. Continue the same task from recorded evidence; do not repeat launches or uncertain actions. Unsupported checks must be reported as unverified, not claimed successful."
	if accessUnobserved {
		feedback = "Computer access was granted for your requested operation, but the selected application has not been inspected. A saved workspace artifact verifies only storage, not browser research or application work. Continue with the available application discovery and observation tools, then perform and verify the original request. Use actual observed sources for research findings and links; do not substitute a file containing the search query. Preserve account/profile constraints and do not repeat prior dispatched actions."
	}
	if len(remainingPlan) > 0 {
		plan, _ := json.Marshal(remainingPlan)
		feedback += " Your saved plan still contains unfinished work: " + string(plan) + ". Perform the requested work and update the plan with actual evidence before finishing. A verified app launch does not finish a document or research task."
	}
	t.Checkpoint.Messages = append(t.Checkpoint.Messages, api.ChatMessage{Role: "system", Content: feedback})
	t.Steps = append(t.Steps, Step{ID: len(t.Steps) + 1, Type: "completion_rejected", Content: feedback, Timestamp: time.Now().UTC()})
	if t.Checkpoint.CompletionRepairs >= 3 {
		t.Status, t.ErrorCode, t.Error = TaskInterrupted, "workflow_completion_repair_limit", "Paused after three completion repairs without new verified progress. Review the remaining checks and saved results before explicitly continuing."
		setRunPhase(t, "interrupted", "")
	} else {
		t.Checkpoint.CompletionRepairs++
		setRunPhase(t, "repairing_tool", "")
	}
	return true, r.persist(t)
}

// ComputerAccessNeedsObservation distinguishes a pending model-requested target
// inspection from a preattached grant, which is only capability, not an obligation.
// In contrast, a resolved model access request records an explicit outstanding
// operation. Storage receipts must not erase that operation before the selected
// target was even inspected. This is a necessary check, not semantic proof of
// research quality or of every part of the natural-language request.
func ComputerAccessNeedsObservation(t *Task, tools interface {
	Capability(string) (capabilities.Descriptor, bool)
}) bool {
	if t.LastAccess == nil || t.LastAccess.CallID == "" || t.LastAccess.Kind != "computer" {
		return false
	}
	// An explicit user correction supersedes the old proposed operation, not its
	// effects. Existing workflow/uncertainty gates continue to own those effects.
	if offset := t.Config.ComputerStepOffset; offset > 0 && offset <= len(t.Steps) {
		resolved := t.Steps[offset-1]
		if resolved.Type == "input_resolved" && !resolved.Timestamp.IsZero() {
			for _, instruction := range t.Instructions {
				if instruction.At.After(resolved.Timestamp) {
					return false
				}
			}
		}
	}
	for index, step := range t.Steps {
		if index < t.Config.ComputerStepOffset || step.Type != "action" {
			continue
		}
		switch step.ToolName {
		case "computer_observe", "computer_verify_application", "browser_observe":
			if descriptor, known := tools.Capability(step.ToolName); known && companionCapability(descriptor) {
				return false
			}
		}
	}
	return true
}
