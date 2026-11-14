package agents

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"

	"github.com/takuphilchan/offgrid-llm/pkg/api"
)

// The allocation and its idempotency receipt commit with both run snapshots.
// Only the root's unallocated remainder moves; no spent or unknown charge is
// removed, and no new attempt or worker identity is created.
type WorkerBudgetGrant struct {
	Digest    string `json:"digest"`
	Requested int    `json:"requested"`
	Granted   int    `json:"granted"`
	Total     int    `json:"total"`
	Reason    string `json:"reason"`
}

func workerBudgetTool() api.Tool {
	return api.Tool{Type: "function", Function: api.FunctionDef{Name: "worker_request_budget", Description: "Request additional model calls from this work's unallocated allowance when needed to finish the assignment. Explain the remaining deliverable. This does not reset spent or uncertain usage, increase the root budget, or use its reserved verification capacity. A grant may be smaller than requested; on denial preserve useful partial results.", Parameters: map[string]interface{}{"type": "object", "additionalProperties": false, "required": []string{"model_calls", "reason"}, "properties": map[string]interface{}{"model_calls": map[string]interface{}{"type": "integer", "minimum": 1, "maximum": 50}, "reason": map[string]interface{}{"type": "string", "minLength": 1, "maxLength": 2048}}}}}
}

func (r *Runner) requestWorkerBudget(task *Task, call api.ToolCall) (string, error) {
	var args struct {
		ModelCalls int    `json:"model_calls"`
		Reason     string `json:"reason"`
	}
	if err := decodeRuntimeArguments(call.Function.Arguments, &args); err != nil {
		return "", err
	}
	if args.ModelCalls < 1 || args.ModelCalls > 50 || !validProfileText(args.Reason, 2048, true) || call.ID == "" || len(call.ID) > 256 {
		return "", ErrControlCommand
	}
	digest := sha256.Sum256([]byte(call.Function.Arguments))
	key := hex.EncodeToString(digest[:])
	r.mu.Lock()
	defer r.mu.Unlock()
	r.manager.mu.Lock()
	defer r.manager.mu.Unlock()
	current := r.manager.tasks[task.ID]
	parent := r.manager.tasks[task.ParentID]
	if r.closing || current == nil || parent == nil || current.Worker == nil || current.Config.WorkerPolicy == nil || current.Actor != task.Actor || parent.Actor != task.Actor || current.ParentID != parent.ID || current.Worker.WorkID != parent.WorkID || parent.WorkID == "" || current.Checkpoint == nil || parent.Checkpoint == nil {
		return "", ErrRunConflict
	}
	if previous, exists := current.Checkpoint.WorkerBudgetRequests[call.ID]; exists {
		if previous.Digest != key {
			return "", ErrRunConflict
		}
		// The runtime resumes with the saved allocation and exact prior receipt.
		copy, err := copyTask(current)
		if err != nil {
			return "", err
		}
		adoptWorkerBudgetCheckpoint(task, copy)
		data, _ := json.Marshal(previous)
		return string(data), nil
	}
	if current.Status != TaskRunning || parent.Status != TaskChildren || current.ControlPending || parent.ControlPending || current.ControlAction != "" || parent.ControlAction != "" || hasConversationHold(current) || hasConversationHold(parent) || len(current.PendingInstructions) > 0 || len(parent.PendingInstructions) > 0 || current.InstructionVersion != parent.InstructionVersion || current.ControlEpoch != task.ControlEpoch || current.AssignmentVersion != task.AssignmentVersion || parent.Config.DisableDelegation || len(current.Checkpoint.WorkerBudgetRequests) >= 50 {
		return "", ErrRunConflict
	}
	member := false
	if parent.Delegation != nil {
		for _, link := range parent.Delegation.Children {
			member = member || link.ID == current.ID && !link.Invalidated
		}
	}
	if !member || current.Checkpoint.ExecutingCall != "" {
		return "", ErrRunConflict
	}
	if len(current.Checkpoint.Calls) != 1 || current.Checkpoint.Calls[0].ID != call.ID || current.Checkpoint.Calls[0].Function != call.Function || call.Function.Name != "worker_request_budget" {
		return "", ErrRunConflict
	}
	allowance, err := childDecisionBudget(parent, 1)
	if err != nil {
		return "", fmt.Errorf("No additional allocation: %w; preserve partial results", err)
	}
	ceiling := 50
	if current.Config.Profile != nil {
		ceiling = min(ceiling, current.Config.Profile.Settings.MaxIterations)
	}
	grant := min(args.ModelCalls, allowance, ceiling-current.Config.MaxIterations)
	if grant < 1 {
		return "", fmt.Errorf("Worker's pinned profile call ceiling is reached; preserve partial results")
	}
	root, err := copyTask(parent)
	if err != nil {
		return "", err
	}
	child, err := copyTask(current)
	if err != nil {
		return "", err
	}
	root.Checkpoint.ReservedChildIterations += grant
	child.Config.MaxIterations += grant
	if child.Checkpoint.WorkerBudgetRequests == nil {
		child.Checkpoint.WorkerBudgetRequests = map[string]WorkerBudgetGrant{}
	}
	receipt := WorkerBudgetGrant{Digest: key, Requested: args.ModelCalls, Granted: grant, Total: child.Config.MaxIterations, Reason: args.Reason}
	child.Checkpoint.WorkerBudgetRequests[call.ID] = receipt
	if err = r.manager.saveTaskBatch([]*Task{root, child}); err != nil {
		return "", err
	}
	r.manager.tasks[root.ID], r.manager.tasks[child.ID] = root, child
	copy, err := copyTask(child)
	if err != nil {
		return "", err
	}
	adoptWorkerBudgetCheckpoint(task, copy)
	data, _ := json.Marshal(receipt)
	return string(data), nil
}

// The execution loop retains its checkpoint pointer across runtime tools.
// Refresh it in place so clearing the recorded call also clears that cursor.
func adoptWorkerBudgetCheckpoint(task, saved *Task) {
	checkpoint := task.Checkpoint
	*checkpoint = *saved.Checkpoint
	saved.Checkpoint = checkpoint
	*task = *saved
}
