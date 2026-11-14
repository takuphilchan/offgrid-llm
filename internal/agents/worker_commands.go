package agents

import (
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"strings"

	"github.com/takuphilchan/offgrid-llm/pkg/api"
)

func workerCommandDigest(c api.WorkerCommand) string {
	data, _ := json.Marshal(c)
	hash := sha256.Sum256(append([]byte("worker-command-v1\x00"), data...))
	return hex.EncodeToString(hash[:])
}

func validateWorkerCommand(c api.WorkerCommand) error {
	if !validTaskID(c.TargetID) || c.ExpectedRevision < 1 {
		return ErrControlCommand
	}
	if c.Kind == "answer" {
		if !validTaskID(c.Payload.QuestionID) {
			return ErrControlCommand
		}
	} else if c.Payload.QuestionID != "" {
		return ErrControlCommand
	}
	switch c.Kind {
	case "message", "continue", "answer":
		if !validProfileText(c.Payload.Content, 16000, true) || strings.TrimSpace(c.Payload.Content) == "" {
			return ErrControlCommand
		}
	case "stop", "close":
		if c.Payload.Content != "" {
			return ErrControlCommand
		}
	default:
		return ErrControlCommand
	}
	return nil
}

func (m *Manager) WorkerCommandReceipt(scope ControlScope, c api.WorkerCommand) (*ConversationCommandReceipt, error) {
	if err := validateWorkerCommand(c); err != nil {
		return nil, err
	}
	if !validProfileOwner(scope.Workspace, scope.Actor) {
		return nil, ErrControlNotFound
	}
	db, err := m.controlDatabase()
	if err != nil {
		return nil, err
	}
	defer db.Close()
	return recordedControlReceipt(db, scope, c.RequestID, workerCommandDigest(c))
}

func (r *Runner) CommandWorker(scope ControlScope, c api.WorkerCommand) (*ConversationCommandReceipt, error) {
	return r.commandWorker(scope, c, false)
}

func (r *Runner) commandWorker(scope ControlScope, c api.WorkerCommand, primaryReview bool) (*ConversationCommandReceipt, error) {
	if err := validateWorkerCommand(c); err != nil {
		return nil, err
	}
	if c.Kind == "continue" {
		return r.continueWorker(scope, c, true)
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
	var publish, publishParent *Task
	receipt, err := withControlCommandDatabase(db, scope, c.RequestID, workerCommandDigest(c), func(tx *sql.Tx) (*ConversationCommandReceipt, error) {
		record, err := readControlRecord(tx, scope, c.TargetID, "worker")
		if err != nil {
			return nil, err
		}
		if record.Revision != c.ExpectedRevision {
			return nil, ErrRunConflict
		}
		var worker api.WorkerSession
		if err = json.Unmarshal(record.Snapshot, &worker); err != nil {
			return nil, err
		}
		current := r.manager.tasks[worker.CurrentRunID]
		parent := r.manager.tasks[worker.RootRunID]
		if current == nil || parent == nil || current.Actor != scope.Actor || parent.Actor != scope.Actor || current.ParentID != parent.ID || current.DeletedAt != nil || parent.DeletedAt != nil || current.Worker == nil || current.Worker.SessionID != worker.ID || current.Worker.Workspace != scope.Workspace {
			return nil, ErrControlNotFound
		}
		next, err := copyTask(current)
		if err != nil {
			return nil, err
		}
		var message *api.WorkerMessage
		switch c.Kind {
		case "message", "answer":
			if hasConversationHold(parent) || hasConversationHold(current) {
				return nil, ErrRunConflict
			}
			statusAllowed := current.Status == TaskPending || current.Status == TaskRunning || current.Status == TaskInterrupted
			if c.Kind == "answer" {
				statusAllowed = current.Status == TaskInput
			}
			if worker.Closed || worker.Stopped || parent.ControlAction == "stop" || !questionAnswerParentAllowed(parent, current, c, primaryReview) || len(parent.PendingInstructions) > 0 || current.InstructionVersion != parent.InstructionVersion || current.ControlPending || !statusAllowed || len(current.Instructions)+len(current.PendingInstructions) >= 32 {
				return nil, ErrRunConflict
			}
			if c.Kind == "answer" {
				if err = answerWorkerQuestion(next, parent, c); err != nil {
					return nil, err
				}
			}
			message, err = appendWorkerMailboxInstruction(tx, scope, next, parent, c)
			if err != nil {
				return nil, err
			}
		case "stop":
			if !terminalControlTask(next) {
				fenceWorkRun(next, "stop")
			}
			worker.Stopped = true
			next.ControlAction = "stop"
			next.ProcessStopPending = HasOwnedExecutionProcesses(next)
			next.ControlPending = r.active[next.ID] != nil || next.ProcessStopPending
		case "close":
			if !terminalControlTask(next) || r.active[next.ID] != nil {
				return nil, ErrRunConflict
			}
			if err = checkWorkerInboxSettled(tx, scope, next); err != nil {
				return nil, err
			}
			worker.Closed = true
		}
		if err = putTaskTransaction(tx, next); err != nil {
			return nil, err
		}
		if c.Kind == "answer" && parent.Status == TaskInput && reviewingWorkerQuestion(parent) {
			if parent.ControlPending || parent.ControlAction != "" || parent.PendingInput == nil || parent.PendingInput.Kind != "worker_clarification" || parent.Checkpoint == nil || len(parent.Checkpoint.Calls) != 1 {
				return nil, ErrRunConflict
			}
			publishParent, err = copyTask(parent)
			if err != nil {
				return nil, err
			}
			recordWorkerUserAnswer(publishParent, c.RequestID, c.Payload.Content)
			finishWorkerQuestionReview(publishParent, parent.Checkpoint.Calls[0].ID)
			if err = putTaskTransaction(tx, publishParent); err != nil {
				return nil, err
			}
		}
		if c.Kind == "stop" && reviewingWorkerQuestion(parent) && parent.Delegation.QuestionReview.RunID == next.ID && parent.ControlAction == "" && !hasConversationHold(parent) && (parent.Status == TaskRunning || parent.Status == TaskPending || parent.Status == TaskInput) {
			publishParent, err = copyTask(parent)
			if err != nil {
				return nil, err
			}
			publishParent.ControlEpoch++
			publishParent.Status, publishParent.SchedulerReady = TaskChildren, false
			publishParent.PendingInput, publishParent.Checkpoint.WorkerAnswerCommand = nil, nil
			if err = putTaskTransaction(tx, publishParent); err != nil {
				return nil, err
			}
		}
		latest, err := readControlRecord(tx, scope, worker.ID, "worker")
		if err != nil {
			return nil, err
		}
		var updated api.WorkerSession
		if err = json.Unmarshal(latest.Snapshot, &updated); err != nil {
			return nil, err
		}
		updated.Stopped, updated.Closed = updated.Stopped || worker.Stopped, updated.Closed || worker.Closed
		updated.Revision++
		if err = putControlRecord(tx, scope, updated.ID, "worker", updated.Revision, &updated); err != nil {
			return nil, err
		}
		cursor, err := appendControlEvent(tx, scope, updated.ID, updated.Revision, "worker_"+c.Kind, &updated, "")
		if err != nil {
			return nil, err
		}
		publish = next
		return &ConversationCommandReceipt{RequestID: c.RequestID, TargetID: updated.ID, Revision: updated.Revision, EventCursor: cursor, Worker: &updated, Message: message}, nil
	})
	if err == nil && publish != nil {
		r.manager.tasks[publish.ID] = publish
		if publishParent != nil {
			r.manager.tasks[publishParent.ID] = publishParent
		}
	}
	r.manager.mu.Unlock()
	if err != nil {
		return nil, err
	}
	if publish != nil && c.Kind == "stop" {
		if publishParent != nil {
			if cancel := r.active[publishParent.ID]; cancel != nil {
				cancel()
			}
		}
		if cancel := r.active[publish.ID]; cancel != nil {
			cancel()
		}
		if publish.ProcessStopPending {
			r.workers.Add(1)
			go r.settleWorkStop(publish)
		}
	}
	return receipt, nil
}
