package agents

import (
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"strings"
	"time"

	"github.com/takuphilchan/offgrid-llm/pkg/api"
)

func workCommandDigest(c api.WorkCommand) string {
	data, _ := json.Marshal(c)
	hash := sha256.Sum256(append([]byte("work-command-v1\x00"), data...))
	return hex.EncodeToString(hash[:])
}

func validateWorkCommand(c api.WorkCommand) error {
	if !validProfileText(c.TargetID, 128, true) || c.ExpectedRevision < 1 {
		return ErrControlCommand
	}
	switch c.Kind {
	case "stop", "pause", "continue":
		if c.Payload.Instruction != "" {
			return ErrControlCommand
		}
	case "correct":
		if !validProfileText(c.Payload.Instruction, 16000, true) || strings.TrimSpace(c.Payload.Instruction) == "" {
			return ErrControlCommand
		}
	default:
		return ErrControlCommand
	}
	return nil
}

// ControlWork commits the command, every affected run fence and work revision
// before cancellation. No inference or executor operation occurs in the tx.
func (r *Runner) ControlWork(scope ControlScope, c api.WorkCommand) (*ConversationCommandReceipt, error) {
	if err := validateWorkCommand(c); err != nil {
		return nil, err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.closing {
		return nil, ErrRunConflict
	}
	db, err := r.manager.controlDatabase()
	if err != nil {
		return nil, err
	}
	defer db.Close()
	r.manager.mu.Lock()
	batch := []*Task{}
	receipt, err := withControlCommandDatabase(db, scope, c.RequestID, workCommandDigest(c), func(tx *sql.Tx) (*ConversationCommandReceipt, error) {
		record, err := readControlRecord(tx, scope, c.TargetID, "work")
		if err != nil {
			return nil, err
		}
		var work WorkItem
		if err = json.Unmarshal(record.Snapshot, &work); err != nil {
			return nil, err
		}
		if record.Revision != c.ExpectedRevision {
			return nil, ErrRunConflict
		}
		current := r.manager.tasks[work.RunID]
		if current == nil || current.Actor != scope.Actor || current.WorkID != work.ID || current.Origin == nil || current.Origin.Workspace != scope.Workspace || current.DeletedAt != nil {
			return nil, ErrControlNotFound
		}
		root, err := copyTask(current)
		if err != nil {
			return nil, err
		}
		if c.Kind == "continue" {
			if err = prepareWorkContinuation(root, r.active[root.ID] != nil); err != nil {
				return nil, err
			}
		} else if c.Kind == "correct" {
			if err = r.prepareWorkCorrection(root, c); err != nil {
				return nil, err
			}
		} else if !terminalControlTask(root) {
			fenceWorkRun(root, c.Kind)
		}
		batch = append(batch, root)
		if !terminalControlTask(current) {
			for _, link := range root.ChildHistory {
				child := r.manager.tasks[link.ID]
				if child == nil || child.Actor != scope.Actor || child.ParentID != root.ID || child.DeletedAt != nil {
					return nil, ErrRunConflict
				}
				if terminalControlTask(child) {
					continue
				}
				if c.Kind == "correct" && root.Delegation == nil {
					continue
				}
				next, err := copyTask(child)
				if err != nil {
					return nil, err
				}
				if c.Kind == "continue" {
					if next.ControlAction == "stop" {
						continue
					}
					if err = prepareWorkContinuation(next, r.active[next.ID] != nil); err != nil {
						return nil, err
					}
					next.SchedulerReady = false // The existing dependency coordinator admits children.
				} else {
					fenceWorkRun(next, c.Kind)
				}
				batch = append(batch, next)
			}
		}
		if c.Kind == "stop" || c.Kind == "pause" {
			for _, task := range batch {
				if task.ControlAction != c.Kind {
					continue
				}
				task.ProcessStopPending = c.Kind == "stop" && HasOwnedExecutionProcesses(task)
				task.ControlPending = r.active[task.ID] != nil || task.ProcessStopPending
				root.ControlPending = root.ControlPending || task.ControlPending
			}
		}
		for _, task := range batch {
			if err = putTaskTransaction(tx, task); err != nil {
				return nil, err
			}
		}
		record, err = readControlRecord(tx, scope, work.ID, "work")
		if err != nil {
			return nil, err
		}
		if err = json.Unmarshal(record.Snapshot, &work); err != nil {
			return nil, err
		}
		// A command has its own revision even if an already completed outcome
		// made Stop a no-op. It cannot replace completion with cancellation.
		work.Revision++
		if err = putControlRecord(tx, scope, work.ID, "work", work.Revision, &work); err != nil {
			return nil, err
		}
		cursor, err := appendControlEvent(tx, scope, work.ID, work.Revision, "work_"+c.Kind, &work, "")
		if err != nil {
			return nil, err
		}
		return &ConversationCommandReceipt{RequestID: c.RequestID, TargetID: work.ID, Revision: work.Revision, EventCursor: cursor, Work: &work}, nil
	})
	if err == nil {
		for _, task := range batch {
			r.manager.tasks[task.ID] = task
		}
	}
	r.manager.mu.Unlock()
	if err != nil {
		return nil, err
	}
	for _, task := range batch {
		if c.Kind != "continue" && (c.Kind != "correct" || task.ParentID != "") {
			if cancel := r.active[task.ID]; cancel != nil {
				cancel()
			}
		}
		if c.Kind == "stop" && task.ProcessStopPending {
			// Process termination uses the service lifetime, never the HTTP
			// request or an inference slot. Stop acceptance returns immediately.
			r.workers.Add(1)
			go r.settleWorkStop(task)
		}
	}
	return receipt, nil
}

func prepareWorkContinuation(t *Task, active bool) error {
	if hasConversationHold(t) {
		return ErrRunConflict
	}
	if active || t.ControlPending || t.ControlAction == "stop" || t.ComputerSessionExpired || t.Checkpoint == nil || t.Checkpoint.ExecutingCall != "" || !WorkflowVersionSupported(t) {
		return ErrRunConflict
	}
	if t.Status == TaskInput {
		return continueWorkerQuestionWait(t)
	}
	if t.Status != TaskInterrupted && t.Status != TaskPending {
		return ErrRunConflict
	}
	if t.ErrorCode == proposalRepairLimitCode {
		t.Checkpoint.PreEffectRepairs, t.Checkpoint.ProposalRepairs = 0, 0
		t.Checkpoint.ProposalUnresolved = true
	}
	t.ControlEpoch++
	t.ControlAction, t.ErrorCode, t.Error = "", "", ""
	t.Status, t.SchedulerReady, t.CompletedAt = TaskPending, true, nil
	if t.Delegation != nil && t.Delegation.QuestionReview == nil {
		t.Status, t.SchedulerReady = TaskChildren, false
	}
	return nil
}

func terminalControlTask(t *Task) bool {
	return !t.ControlPending && (t.Status == TaskCompleted || t.Status == TaskFailed || t.Status == TaskCancelled)
}

func fenceWorkRun(t *Task, kind string) {
	if kind == "stop" || kind == "pause" {
		t.ControlAction = kind
	}
	t.ControlEpoch++
	t.SchedulerReady, t.PendingApproval = false, nil
	if kind == "stop" {
		finishTask(t, TaskCancelled, "Stopped by user.")
		if HasOwnedExecutionProcesses(t) {
			t.Status, t.ErrorCode, t.Error, t.CompletedAt = TaskInterrupted, "execution_stopping", "Stopping owned processes; termination is not yet confirmed.", nil
		}
	} else {
		if t.Status != TaskInput {
			t.Status = TaskInterrupted
		}
		t.ErrorCode, t.Error = "task_paused", "Paused. Continue explicitly when ready."
		if kind == "correct" {
			t.ErrorCode, t.Error = "correction_pending", "Parent correction fenced this assignment."
		}
	}
	if uncertainEffect(t.Checkpoint) {
		t.Status, t.ErrorCode, t.Error = TaskUncertain, "uncertain_outcome", "Control accepted during a dispatched operation. Its actual outcome must be reconciled; it will not be replayed."
	}
	releaseInterruptedRead(t.Checkpoint)
}

func (r *Runner) prepareWorkCorrection(t *Task, c api.WorkCommand) error {
	if !CanSteerTask(t) && !(t.Status == TaskPending && t.Checkpoint != nil && t.Checkpoint.ExecutingCall == "" && len(t.Instructions)+len(t.PendingInstructions) < 32) {
		return ErrRunConflict
	}
	digest := sha256.Sum256([]byte(c.Payload.Instruction))
	entry := Instruction{RequestID: c.RequestID, Digest: hex.EncodeToString(digest[:]), Text: c.Payload.Instruction, At: time.Now().UTC()}
	t.ControlEpoch++
	if t.Delegation != nil {
		t.PendingInstructions = append(t.PendingInstructions, entry)
		t.Delegation.CorrectionPaused = t.Delegation.CorrectionPaused || t.Status == TaskInterrupted
		t.Status, t.ErrorCode, t.Error = TaskChildren, "correction_pending", "Correction accepted. Waiting for delegated operations to settle."
	} else if t.Status == TaskRunning {
		t.PendingInstructions = append(t.PendingInstructions, entry)
	} else {
		if _, active := r.active[t.ID]; active {
			return ErrRunConflict
		}
		for _, pending := range t.PendingInstructions {
			if err := applyInstruction(t, pending); err != nil {
				return err
			}
		}
		t.PendingInstructions = nil
		return applyInstruction(t, entry)
	}
	return nil
}

func (r *Runner) settleWorkStop(task *Task) {
	defer r.workers.Done()
	defer r.settleWorkControls()
	settled, err := r.stopOwnedExecution(task)
	if err != nil {
		return
	}
	updated, err := r.manager.updateTask(task.ID, func(current *Task) error {
		if current.ControlEpoch != task.ControlEpoch {
			return ErrRunConflict
		}
		current.ProcessStopPending = false
		if settled.Status == TaskInterrupted && settled.ErrorCode == "execution_stopping" && current.ErrorCode == "execution_stopping" {
			finishTask(current, TaskCancelled, "Stopped by user. Owned process termination confirmed; earlier effects are retained.")
			current.ErrorCode = ""
		}
		return nil
	})
	if err == nil {
		r.observe(updated)
	}
}
