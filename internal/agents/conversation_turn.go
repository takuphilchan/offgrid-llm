package agents

import (
	"database/sql"
	"encoding/json"
	"time"

	"github.com/takuphilchan/offgrid-llm/pkg/api"
)

// A foreground turn uses the durable runner but is not a work item or a child.
// Its identity and allowance never replace the focused work's identity/budget.
type ConversationTurnBinding struct {
	InputID  string                    `json:"input_id"`
	Decision *api.ConversationDecision `json:"decision,omitempty"`
}

func validateConversationTurn(t *Task) error {
	if t.ConversationTurn == nil {
		return nil
	}
	if !validTaskID(t.ConversationTurn.InputID) || t.Origin == nil || t.Origin.TurnID != t.ConversationTurn.InputID || t.WorkID != "" || t.Worker != nil || t.ParentID != "" || t.Delegation != nil || len(t.ChildHistory) != 0 || hasConversationHold(t) {
		return ErrRunStorage
	}
	// Foreground coordination has no host grant. Host actions belong to a work
	// run with its own authority, journal, controls and budget.
	if t.Config.Execution != nil || t.Config.ComputerSession != "" || t.Config.RequestedComputerScope != "" || !t.Config.DisableDelegation || !t.Config.ScopedCapabilities || len(t.Config.AllowedTools) != 0 || len(t.Config.AllowedCapabilities) != 0 {
		return ErrRunStorage
	}
	if d := t.ConversationTurn.Decision; d != nil && !validConversationDecision(d) {
		return ErrRunStorage
	}
	return nil
}

func readConversationInbox(q controlReader, scope ControlScope, conversation, id string) (*api.ConversationInboxEntry, error) {
	var data []byte
	err := q.QueryRow(`SELECT payload FROM agent_conversation_inbox WHERE workspace=? AND actor=? AND conversation_id=? AND input_id=?`, scope.Workspace, scope.Actor, conversation, id).Scan(&data)
	if err == sql.ErrNoRows {
		return nil, ErrControlNotFound
	}
	if err != nil {
		return nil, err
	}
	var entry api.ConversationInboxEntry
	err = json.Unmarshal(data, &entry)
	return &entry, err
}

func putConversationInbox(tx *sql.Tx, scope ControlScope, entry *api.ConversationInboxEntry) error {
	data, err := json.Marshal(entry)
	if err != nil {
		return err
	}
	result, err := tx.Exec(`UPDATE agent_conversation_inbox SET state=?,payload=? WHERE workspace=? AND actor=? AND conversation_id=? AND input_id=?`, entry.State, data, scope.Workspace, scope.Actor, entry.ConversationID, entry.ID)
	if err != nil {
		return err
	}
	count, err := result.RowsAffected()
	if err == nil && count != 1 {
		err = ErrControlNotFound
	}
	return err
}

func conversationInputEvent(tx *sql.Tx, scope ControlScope, entry *api.ConversationInboxEntry, kind, runID string) error {
	_, err := conversationInputEventCursor(tx, scope, entry, kind, runID)
	return err
}

func conversationInputEventCursor(tx *sql.Tx, scope ControlScope, entry *api.ConversationInboxEntry, kind, runID string) (int64, error) {
	record, err := readControlRecord(tx, scope, entry.ConversationID, "conversation")
	if err != nil {
		return 0, err
	}
	var conversation Conversation
	if err = json.Unmarshal(record.Snapshot, &conversation); err != nil {
		return 0, err
	}
	conversation.Revision++
	if err = putControlRecord(tx, scope, conversation.ID, "conversation", conversation.Revision, &conversation); err != nil {
		return 0, err
	}
	return appendControlEvent(tx, scope, conversation.ID, conversation.Revision, kind, entry, runID)
}

// Settlement and the task snapshot share a transaction. A failed observer or
// compatibility projection cannot erase a reply or manufacture another turn.
// A reply is not proof that a correction was applied: holds remain until the
// explicit incorporation reducer resolves them.
func syncConversationTurn(tx *sql.Tx, task *Task) error {
	if task.ConversationTurn == nil {
		return nil
	}
	scope := ControlScope{task.Origin.Workspace, task.Actor}
	entry, err := readConversationInbox(tx, scope, task.Origin.Conversation, task.ConversationTurn.InputID)
	if err != nil {
		return err
	}
	if entry.RunID != task.ID {
		return ErrRunConflict
	}
	decisionChanged := false
	if decision := task.ConversationTurn.Decision; decision != nil {
		if err = validateConversationDecision(entry, decision); err != nil {
			return err
		}
		if entry.Decision != nil && *entry.Decision != *decision {
			return ErrRunConflict
		}
		if entry.Decision == nil {
			copy := *decision
			entry.Decision = &copy
			decisionChanged = true
		}
	}
	if entry.RunStatus == task.Status && !decisionChanged {
		return nil
	}
	eventKind := "input_run_state"
	if entry.RunStatus == task.Status {
		eventKind = "input_decision_recorded"
	}
	if entry.State != "processing" {
		return ErrRunConflict
	}
	entry.RunStatus = task.Status
	if terminalControlTask(task) {
		now := time.Now().UTC()
		entry.SettledAt = &now
		entry.State = "failed"
		if task.Status == TaskCompleted {
			entry.State = "responded"
			// A direct ordinary answer needs no separate classification or tool
			// call. Explicit corrections/new work/questions never fall through.
			if entry.Decision == nil && (entry.Input.Intent == "" || entry.Input.Intent == "message") {
				entry.Decision = &api.ConversationDecision{Kind: "answered"}
			}
		}
		if task.Result != "" {
			entry.ResultReference = resultReference(task)
		}
		history, err := json.Marshal(map[string]any{"id": "reply-" + entry.ID, "role": "assistant", "content": task.Result, "input_id": entry.ID, "run_id": task.ID, "input_state": entry.State, "run_status": task.Status, "error": task.Error, "result_reference": entry.ResultReference})
		if err != nil {
			return err
		}
		if _, err = tx.Exec(`INSERT INTO agent_conversation_messages(workspace,actor,conversation_id,message_id,payload) VALUES(?,?,?,?,?)`, scope.Workspace, scope.Actor, entry.ConversationID, "reply-"+entry.ID, history); err != nil {
			return err
		}
	}
	if err = putConversationInbox(tx, scope, entry); err != nil {
		return err
	}
	if task.StartedAt != nil || task.Status != TaskPending {
		if _, err = tx.Exec(`UPDATE agent_control_outbox SET disposition=? WHERE run_id=? AND disposition='pending'`, string(task.Status), task.ID); err != nil {
			return err
		}
	}
	return conversationInputEvent(tx, scope, entry, eventKind, "")
}
