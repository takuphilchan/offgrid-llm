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
	r.assessWorkflow(t)
	if t.Workflow.State != "pending" {
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
