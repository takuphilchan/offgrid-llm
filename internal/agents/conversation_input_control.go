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

func validateInputControl(conversation string, c api.ConversationInputControlCommand) error {
	if !validTaskID(conversation) || !validTaskID(c.TargetID) || !validProfileText(c.RequestID, 128, true) || strings.ContainsAny(c.RequestID, "\r\n") || c.ExpectedRevision < 1 {
		return ErrControlCommand
	}
	if c.Kind == "revalidate" && c.Payload.InstructionVersion > 0 || (c.Kind == "stop" || c.Kind == "continue") && c.Payload.InstructionVersion == 0 {
		return nil
	}
	return ErrControlCommand
}

func inputControlDigest(conversation string, c api.ConversationInputControlCommand) string {
	data, _ := json.Marshal(struct {
		Conversation string
		Command      api.ConversationInputControlCommand
	}{conversation, c})
	hash := sha256.Sum256(append([]byte("conversation-input-control-v1\x00"), data...))
	return hex.EncodeToString(hash[:])
}

func (m *Manager) ConversationInputCommandReceipt(scope ControlScope, c api.ConversationInputCommand) (*ConversationCommandReceipt, error) {
	if err := validateConversationInput(c); err != nil {
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
	return recordedControlReceipt(db, scope, c.RequestID, conversationInputDigest(c))
}

func (m *Manager) ConversationInputControlReceipt(scope ControlScope, conversation string, c api.ConversationInputControlCommand) (*ConversationCommandReceipt, error) {
	if err := validateInputControl(conversation, c); err != nil {
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
	return recordedControlReceipt(db, scope, c.RequestID, inputControlDigest(conversation, c))
}

// ControlConversationInput accepts cancellation independently of inference and
// source readiness. Continuation/revalidation require current source access.
func (r *Runner) ControlConversationInput(scope ControlScope, conversation string, c api.ConversationInputControlCommand) (*ConversationCommandReceipt, error) {
	previous, err := r.manager.ConversationInputControlReceipt(scope, conversation, c)
	if err != nil || previous != nil {
		return previous, err
	}
	if c.Kind != "stop" {
		entry, err := r.manager.ConversationInput(scope, conversation, c.TargetID)
		if err != nil {
			return nil, err
		}
		task, ok := r.manager.GetTask(entry.RunID)
		if !ok || task.Actor != scope.Actor {
			return nil, ErrControlNotFound
		}
		if err = r.checkSourceAccess(task); err != nil {
			return nil, err
		}
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
	var publish []*Task
	receipt, err := withControlCommandDatabase(db, scope, c.RequestID, inputControlDigest(conversation, c), func(tx *sql.Tx) (*ConversationCommandReceipt, error) {
		record, err := readControlRecord(tx, scope, conversation, "conversation")
		if err != nil {
			return nil, err
		}
		if record.Revision != c.ExpectedRevision {
			return nil, ErrRunConflict
		}
		entry, err := readConversationInbox(tx, scope, conversation, c.TargetID)
		if err != nil {
			return nil, err
		}
		var foreground *Task
		if entry.RunID != "" {
			current := r.manager.tasks[entry.RunID]
			if current == nil || current.DeletedAt != nil || current.Actor != scope.Actor || current.ConversationTurn == nil || current.ConversationTurn.InputID != entry.ID || current.Origin == nil || current.Origin.Workspace != scope.Workspace || current.Origin.Conversation != conversation {
				return nil, ErrControlNotFound
			}
			foreground, err = copyTask(current)
			if err != nil {
				return nil, err
			}
		}
		if c.Kind == "stop" && entry.AppliedAt == nil && entry.StoppedAt == nil {
			if foreground != nil && !terminalControlTask(foreground) {
				fenceWorkRun(foreground, "stop")
				foreground.ControlPending = r.active[foreground.ID] != nil
				if err = putTaskTransaction(tx, foreground); err != nil {
					return nil, err
				}
				publish = append(publish, foreground)
				entry, err = readConversationInbox(tx, scope, conversation, c.TargetID)
				if err != nil {
					return nil, err
				}
			}
			now := time.Now().UTC()
			entry.State, entry.StoppedAt = "stopping", &now
			entry.DeliveryIssue, entry.DecisionIssue = nil, nil
		} else if c.Kind == "continue" {
			// Continue the pinned interrupted attempt and its remaining allowance.
			// A failed/completed/stopped attempt is never silently replaced.
			if foreground == nil || entry.State != "processing" || entry.StoppedAt != nil || foreground.Status != TaskInterrupted {
				return nil, ErrRunConflict
			}
			if err = prepareWorkContinuation(foreground, r.active[foreground.ID] != nil); err != nil {
				return nil, err
			}
			if err = putTaskTransaction(tx, foreground); err != nil {
				return nil, err
			}
			publish = append(publish, foreground)
			entry, err = readConversationInbox(tx, scope, conversation, c.TargetID)
			if err != nil {
				return nil, err
			}
		} else if c.Kind == "revalidate" {
			if foreground == nil || foreground.Status != TaskCompleted || entry.State != "responded" || entry.StoppedAt != nil || entry.AppliedAt != nil || entry.Decision == nil || entry.Decision.Kind != "correction" {
				return nil, ErrRunConflict
			}
			work, err := linkedInputWork(tx, scope, conversation, entry.WorkID)
			if err != nil {
				return nil, err
			}
			if work.InstructionVersion != c.Payload.InstructionVersion || work.Status == TaskCancelled || work.Status == TaskCompleted || work.Status == TaskFailed {
				return nil, ErrRunConflict
			}
			now := time.Now().UTC()
			entry.RevalidatedAt, entry.RevalidatedInstructionVersion, entry.DecisionIssue = &now, c.Payload.InstructionVersion, nil
		}
		if err = putConversationInbox(tx, scope, entry); err != nil {
			return nil, err
		}
		cursor, err := conversationInputEventCursor(tx, scope, entry, "input_"+c.Kind, "")
		if err != nil {
			return nil, err
		}
		record, err = readControlRecord(tx, scope, conversation, "conversation")
		if err != nil {
			return nil, err
		}
		var current Conversation
		if err = json.Unmarshal(record.Snapshot, &current); err != nil {
			return nil, err
		}
		return &ConversationCommandReceipt{RequestID: c.RequestID, TargetID: entry.ID, Revision: current.Revision, EventCursor: cursor, Conversation: &current, Input: entry}, nil
	})
	if err == nil {
		for _, task := range publish {
			r.manager.tasks[task.ID] = task
		}
	}
	r.manager.mu.Unlock()
	if err == nil && c.Kind == "stop" {
		for _, task := range publish {
			if cancel := r.active[task.ID]; cancel != nil {
				cancel()
			}
		}
	}
	return receipt, err
}
