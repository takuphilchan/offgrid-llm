package agents

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
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

// CanSteerTask describes saved-state eligibility, not a grant. Steer also checks
// that the worker has settled under the admission lock before accepting a change.
func CanSteerTask(t *Task) bool {
	return t != nil && t.DeletedAt == nil && len(t.Instructions) < 32 && t.Status == TaskInterrupted && t.Checkpoint != nil && t.Checkpoint.ExecutingCall == "" && t.Delegation == nil
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

// Steer accepts only an explicitly paused, settled run. Old proposed calls and
// approvals cannot survive a changed user instruction. Request IDs prevent a
// lost response from appending the same instruction twice.
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
		if _, active := r.active[id]; active {
			return ErrRunConflict
		}
		if !CanSteerTask(t) {
			return ErrRunConflict
		}
		abandonProposals(t, "Not executed: the user changed the task while paused.")
		t.Checkpoint.Messages = append(t.Checkpoint.Messages, api.ChatMessage{Role: "user", Content: instruction})
		t.Instructions = append(t.Instructions, Instruction{requestID, digest, instruction, time.Now().UTC()})
		t.Error, t.ErrorCode = "Instruction saved. Resume this task to continue.", "task_paused"
		return nil
	})
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
