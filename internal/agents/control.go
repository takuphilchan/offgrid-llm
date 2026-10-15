package agents

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/takuphilchan/offgrid-llm/pkg/api"
)

type Instruction struct {
	RequestID string    `json:"request_id"`
	Digest    string    `json:"digest"`
	Text      string    `json:"text"`
	At        time.Time `json:"at"`
}

// CanSteerTask describes saved-state eligibility, not a grant. Corrections to
// running parent work are queued; they never cancel input already dispatched.
func CanSteerTask(t *Task) bool {
	if t == nil || t.AdmissionHeld || t.DeletedAt != nil || t.ParentID != "" || len(t.Instructions)+len(t.PendingInstructions) >= 32 || t.Checkpoint == nil || t.Delegation != nil {
		return false
	}
	return t.Status == TaskRunning || (t.Status == TaskInterrupted && t.Checkpoint.ExecutingCall == "")
}

// Pause cancels further scheduling, not the truth about an in-flight effect.
// The same manager lock guards intent persistence: an already persisted tool
// call becomes uncertain; a later worker write cannot dispatch after this state.
func (r *Runner) Pause(id, actor string, takeover bool) (*Task, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	task, err := r.manager.updateTask(id, func(t *Task) error {
		if t.Actor != actor {
			return ErrTaskNotFound
		}
		if t.Status == TaskCompleted || t.Status == TaskFailed || t.Status == TaskCancelled {
			return ErrRunConflict
		}
		if t.Status == TaskUncertain {
			return ErrRunConflict
		}
		t.PendingApproval = nil
		if t.Status != TaskInput {
			t.Status = TaskInterrupted
		}
		t.ErrorCode, t.Error = "task_paused", "Paused. Review or update the task before continuing."
		if takeover && t.Config.ComputerSession != "" {
			t.ComputerSessionExpired = true
		}
		if uncertainEffect(t.Checkpoint) {
			t.Status, t.ErrorCode, t.Error = TaskUncertain, "computer_uncertain_outcome", "Paused during a tool call. Inspect its outcome before continuing; it will not be repeated automatically."
		}
		releaseInterruptedRead(t.Checkpoint)
		return nil
	})
	if err == nil {
		if cancel := r.active[id]; cancel != nil {
			cancel()
		}
		if cascadeErr := r.stopChildrenLocked(task, false); cascadeErr != nil {
			return task, cascadeErr
		}
		r.observe(task)
	}
	return task, err
}

// r.mu is held. Children share the user's lifecycle decision but never inherit
// an approval. In-flight effects stay uncertain and require reconciliation.
func (r *Runner) stopChildrenLocked(parent *Task, cancelled bool) error {
	var failure error
	for _, link := range parent.ChildHistory {
		child, ok := r.manager.GetTask(link.ID)
		if !ok || child.Actor != parent.Actor || child.ParentID != parent.ID {
			continue
		}
		if child.Status == TaskCompleted || child.Status == TaskFailed || child.Status == TaskCancelled || child.Status == TaskUncertain {
			continue
		}
		_, err := r.manager.updateTask(child.ID, func(t *Task) error {
			t.PendingApproval = nil
			t.Status, t.ErrorCode, t.Error = TaskInterrupted, "task_paused", "Parent task paused. Resume explicitly."
			if cancelled {
				finishTask(t, TaskCancelled, "Parent task stopped.")
			}
			if uncertainEffect(t.Checkpoint) {
				t.Status, t.ErrorCode, t.Error = TaskUncertain, "uncertain_outcome", "Parent stopped during this tool call. Inspect and reconcile its outcome."
			}
			releaseInterruptedRead(t.Checkpoint)
			return nil
		})
		if cancel := r.active[child.ID]; cancel != nil {
			cancel()
		}
		if err != nil {
			failure = err
		}
	}
	return failure
}

// Steer queues running-work corrections or applies them to a settled paused
// checkpoint. Old proposals/approvals cannot survive an applied correction.
// A queued correction does not claim to have prevented an in-flight action.
func (r *Runner) Steer(id, actor, requestID, instruction string) (*Task, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	instruction = strings.TrimSpace(instruction)
	if len(requestID) < 8 || len(requestID) > 128 || strings.ContainsAny(requestID, "\x00\r\n") || instruction == "" || len(instruction) > 16000 {
		return nil, ErrRunConflict
	}
	sum := sha256.Sum256([]byte(instruction))
	digest := hex.EncodeToString(sum[:])
	return r.manager.updateTask(id, func(t *Task) error {
		if t.Actor != actor {
			return ErrTaskNotFound
		}
		for _, previous := range t.Instructions {
			if previous.RequestID == requestID {
				if previous.Digest != digest {
					return ErrRunConflict
				}
				return nil
			}
		}
		for _, previous := range t.PendingInstructions {
			if previous.RequestID == requestID {
				if previous.Digest != digest {
					return ErrRunConflict
				}
				return nil
			}
		}
		if !CanSteerTask(t) {
			return ErrRunConflict
		}
		entry := Instruction{requestID, digest, instruction, time.Now().UTC()}
		if t.Status == TaskRunning {
			t.PendingInstructions = append(t.PendingInstructions, entry)
			return nil
		}
		if _, active := r.active[id]; active {
			return ErrRunConflict
		}
		for _, pending := range t.PendingInstructions {
			if err := applyInstruction(t, pending); err != nil {
				return err
			}
		}
		t.PendingInstructions = nil
		if err := applyInstruction(t, entry); err != nil {
			return err
		}
		t.Error, t.ErrorCode = "Instruction saved. Resume this task to continue.", "task_paused"
		return nil
	})
}

func applyInstruction(t *Task, entry Instruction) error {
	if err := reviseWorkflow(t, entry.RequestID, entry.Digest); err != nil {
		return err
	}
	abandonProposals(t, "Not executed: the user changed the task before this proposal was dispatched.")
	if t.Checkpoint.PreEffectRepair || t.Checkpoint.ProposalUnresolved || t.Checkpoint.ProposalRepairs > 0 {
		t.Checkpoint.PreEffectRepair, t.Checkpoint.PreEffectRepairTool, t.Checkpoint.PreEffectRepairs = false, "", 0
		t.Checkpoint.ProposalRepairs = 0
		t.Checkpoint.NativeReadRepair = nil
		// An explicit user correction can change the requested outcome.
		t.Checkpoint.ProposalUnresolved = false
		t.Checkpoint.ProposalTool = ""
		t.Checkpoint.WorkflowEditPending = false
	}
	t.Checkpoint.Messages = append(t.Checkpoint.Messages, api.ChatMessage{Role: "user", Content: entry.Text})
	t.Instructions = append(t.Instructions, entry)
	return nil
}

var errSteeringPending = errors.New("a user correction reached the next safe boundary")

// Only the running worker consumes its inbox. The same task transaction that
// retires old proposals records application, so restart cannot apply it twice.
func (r *Runner) applyPendingInstructions(task *Task) (bool, error) {
	r.manager.mu.RLock()
	current := r.manager.tasks[task.ID]
	exists, pending := current != nil, current != nil && len(current.PendingInstructions) > 0
	r.manager.mu.RUnlock()
	if !exists {
		return false, ErrTaskNotFound
	}
	if !pending || task.Delegation != nil {
		return false, nil
	}
	updated, err := r.manager.updateTask(task.ID, func(saved *Task) error {
		if saved.Status != TaskRunning {
			return context.Canceled
		}
		if saved.Checkpoint == nil || saved.Checkpoint.ExecutingCall != "" || task.Checkpoint.ExecutingCall != "" {
			return ErrRunConflict
		}
		next, err := copyTask(task)
		if err != nil {
			return err
		}
		for _, entry := range saved.PendingInstructions {
			if err := applyInstruction(next, entry); err != nil {
				return err
			}
		}
		next.PendingInstructions = nil
		*saved = *next
		return nil
	})
	if err != nil {
		return false, err
	}
	*task = *updated
	r.observe(updated)
	return true, nil
}

func abandonProposals(t *Task, reason string) {
	for _, call := range t.Checkpoint.Calls {
		t.Checkpoint.Messages = append(t.Checkpoint.Messages, api.ChatMessage{Role: "tool", ToolCallID: call.ID, Name: call.Function.Name, Content: reason})
	}
	t.Checkpoint.Calls = nil
	t.PendingApproval = nil
	t.PendingInput = nil
}

// RequestAccess is an explicit recovery command, not a replay of old actions.
// A new immutable call replaces only proposals that have never been dispatched.
func (r *Runner) RequestAccess(id, actor, mode, target, url string) (*Task, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, active := r.active[id]; active {
		return nil, ErrRunConflict
	}
	return r.manager.updateTask(id, func(t *Task) error {
		if t.Actor != actor {
			return ErrTaskNotFound
		}
		if !t.Config.TaskFirst || t.ParentID != "" || t.Delegation != nil || t.Checkpoint == nil || t.Checkpoint.ExecutingCall != "" || (t.Status != TaskInterrupted && t.Status != TaskInput) {
			return ErrRunConflict
		}
		if (mode != "app" && mode != "browser") || strings.TrimSpace(target) == "" || len(target) > 256 || len(url) > 2048 {
			return ErrRunConflict
		}
		if t.PendingInput != nil && t.PendingInput.Mode == mode && t.PendingInput.Target == target && t.PendingInput.URL == url {
			return nil
		}
		abandonProposals(t, "Not executed: local computer access is being renewed.")
		args, _ := json.Marshal(map[string]string{"mode": mode, "target": target, "url": url})
		call := api.ToolCall{ID: "call-" + runID(), Type: "function", Function: api.FunctionCall{Name: "request_computer_access", Arguments: string(args)}}
		t.Checkpoint.Messages = append(t.Checkpoint.Messages, api.ChatMessage{Role: "assistant", ToolCalls: []api.ToolCall{call}})
		t.Checkpoint.Calls = []api.ToolCall{call}
		input := InputRequest{ID: "input-" + runID(), CallID: call.ID, Kind: "computer", Mode: mode, Target: target, URL: url, PreviousSession: t.Config.ComputerSession}
		t.PendingInput = &input
		t.Status = TaskInput
		t.PendingApproval = nil
		t.Config.ComputerSession = ""
		t.ComputerSessionExpired = false
		t.Error, t.ErrorCode = "", ""
		return nil
	})
}
