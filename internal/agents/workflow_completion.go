package agents

import (
	"encoding/json"
	"sort"
	"time"

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
	// Authority alone is not an external outcome. A direct conversational
	// answer needs no invented application/check. Once an action or explicit
	// expectation exists the ordinary evidence gate still applies.
	if len(t.Workflow.Checks) == 0 && len(t.Config.NativeExpectations) == 0 && len(t.Plan) == 0 {
		hasAction := false
		for _, step := range t.Steps {
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
	if t.Workflow.State != "pending" && len(remainingPlan) == 0 {
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
