package agents

import (
	"encoding/json"
	"time"

	"github.com/takuphilchan/offgrid-llm/pkg/api"
)

func (m *Manager) StoppingConversationInputs(workspace string, after int64, limit int) (*ConversationDeliveryPage, error) {
	if !validProfileText(workspace, 256, true) || after < 0 || limit < 1 || limit > 100 {
		return nil, ErrControlCommand
	}
	db, err := m.controlDatabase()
	if err != nil {
		return nil, err
	}
	defer db.Close()
	rows, err := db.Query(`SELECT sequence,actor,payload FROM agent_conversation_inbox WHERE workspace=? AND sequence>? AND state='stopping' ORDER BY sequence LIMIT ?`, workspace, after, limit+1)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	page := &ConversationDeliveryPage{Items: []ConversationDeliveryCandidate{}, NextCursor: after}
	for rows.Next() {
		if len(page.Items) == limit {
			page.HasMore = true
			break
		}
		item := ConversationDeliveryCandidate{Scope: ControlScope{Workspace: workspace}}
		var data []byte
		if err = rows.Scan(&page.NextCursor, &item.Scope.Actor, &data); err != nil {
			return nil, err
		}
		if err = json.Unmarshal(data, &item.Input); err != nil {
			return nil, err
		}
		if item.Input.Sequence != page.NextCursor || item.Input.State != "stopping" || item.Input.StoppedAt == nil || item.Input.AppliedAt != nil {
			return nil, ErrRunStorage
		}
		page.Items = append(page.Items, item)
	}
	return page, rows.Err()
}

// Release the withdrawn input's hold only after foreground cancellation and any
// already-dispatched focused-work operations settle. This does not stop work.
func (r *Runner) SettleStoppedConversationInput(scope ControlScope, conversation, input string) (*api.ConversationInboxEntry, error) {
	if !validProfileOwner(scope.Workspace, scope.Actor) {
		return nil, ErrControlNotFound
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
	defer r.manager.mu.Unlock()
	tx, err := db.Begin()
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	if _, err = tx.Exec(`UPDATE agent_schema SET version=version WHERE singleton=1`); err != nil {
		return nil, err
	}
	entry, err := readConversationInbox(tx, scope, conversation, input)
	if err != nil {
		return nil, err
	}
	if entry.State == "stopped" && entry.HoldReleasedAt != nil {
		return entry, nil
	}
	if entry.State != "stopping" || entry.StoppedAt == nil || entry.AppliedAt != nil {
		return nil, ErrRunConflict
	}
	if entry.RunID != "" {
		task := r.manager.tasks[entry.RunID]
		if task == nil || task.Actor != scope.Actor || task.ConversationTurn == nil || task.ConversationTurn.InputID != input || task.Origin == nil || task.Origin.Workspace != scope.Workspace || task.Origin.Conversation != conversation {
			return nil, ErrControlNotFound
		}
		if r.active[task.ID] != nil || task.ControlPending || !terminalControlTask(task) {
			return nil, ErrRunConflict
		}
	}
	batch, err := r.conversationInputGraphOrdered(tx, scope, entry, false)
	if err != nil {
		return nil, err
	}
	for _, task := range batch {
		releaseConversationInput(task, input)
		if err = putTaskTransaction(tx, task); err != nil {
			return nil, err
		}
	}
	now := time.Now().UTC()
	entry.State, entry.HoldReleasedAt = "stopped", &now
	if err = putConversationInbox(tx, scope, entry); err != nil {
		return nil, err
	}
	if err = conversationInputEvent(tx, scope, entry, "input_stopped", ""); err != nil {
		return nil, err
	}
	if err = tx.Commit(); err != nil {
		return nil, err
	}
	for _, task := range batch {
		r.manager.tasks[task.ID] = task
	}
	return entry, nil
}
