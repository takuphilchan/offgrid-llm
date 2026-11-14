package agents

import (
	"encoding/json"
	"time"
)

// AdmitConversationTurn is a service-only delivery operation. The service
// prepares authorized context/model configuration against expectedRevision.
// Delivery is FIFO and idempotent by the accepted input's identity. A retry
// returns the original run, including interrupted/settled runs, without resume.
func (r *Runner) AdmitConversationTurn(scope ControlScope, conversationID, inputID string, expectedRevision int64, model string, config AgentConfig) (*Task, error) {
	if !validProfileOwner(scope.Workspace, scope.Actor) || !validTaskID(conversationID) || !validTaskID(inputID) {
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
	entry, err := readConversationInbox(tx, scope, conversationID, inputID)
	if err != nil {
		return nil, err
	}
	if entry.RunID != "" {
		task := r.manager.tasks[entry.RunID]
		if task == nil || task.DeletedAt != nil || task.Actor != scope.Actor || task.ConversationTurn == nil || task.ConversationTurn.InputID != inputID || task.Origin == nil || task.Origin.Workspace != scope.Workspace || task.Origin.Conversation != conversationID {
			return nil, ErrRunStorage
		}
		return copyTask(task)
	}
	if entry.State != "accepted" {
		return nil, ErrRunConflict
	}
	// The same transaction guards both the queue head and the new run. Even
	// separate callers cannot claim two entries from one conversation at once.
	var first string
	if err = tx.QueryRow(`SELECT input_id FROM agent_conversation_inbox WHERE workspace=? AND actor=? AND conversation_id=? AND state IN ('accepted','processing') ORDER BY sequence LIMIT 1`, scope.Workspace, scope.Actor, conversationID).Scan(&first); err != nil {
		return nil, err
	}
	if first != inputID {
		return nil, ErrRunConflict
	}
	record, err := readControlRecord(tx, scope, conversationID, "conversation")
	if err != nil {
		return nil, err
	}
	if expectedRevision != record.Revision {
		return nil, ErrRunConflict
	}
	var conversation Conversation
	if err = json.Unmarshal(record.Snapshot, &conversation); err != nil {
		return nil, err
	}
	project, err := readControlRecord(tx, scope, conversation.ProjectID, "project")
	if err != nil {
		return nil, err
	}
	if config.ProjectID != "" && (config.ProjectID != project.ID || config.ProjectRevision != project.Revision) {
		return nil, ErrRunConflict
	}
	if prepared := config.EffectivePreferences; prepared != nil {
		user, err := readUserPreferences(tx, scope)
		if err != nil {
			return nil, err
		}
		if prepared.UserPreferencesRevision != user.Revision || prepared.Model != model {
			return nil, ErrRunConflict
		}
	}
	if entry.Input.Model != "" && entry.Input.Model != model {
		return nil, ErrRunConflict
	}
	if config.Execution != nil || config.ComputerSession != "" || config.RequestedComputerScope != "" || config.Automation != nil {
		return nil, ErrControlCommand
	}
	config.ProjectID, config.ProjectRevision = project.ID, project.Revision
	config.ResultIdentityVersion = 1
	config.DisableDelegation, config.ScopedCapabilities = true, true
	config.AllowedTools, config.AllowedCapabilities = nil, nil
	config.DelegateProfiles = nil
	origin := &ConversationOrigin{Workspace: scope.Workspace, Conversation: conversationID, TurnID: inputID}
	task, err := prepareConversationRun(entry.Input.Content, model, scope.Actor, config, origin)
	if err != nil {
		return nil, err
	}
	task.ConversationTurn = &ConversationTurnBinding{InputID: inputID}
	task.RequestID, task.RequestDigest = "foreground-"+inputID, inputID
	task.AdmissionHeld, task.SchedulerReady = false, true
	now := time.Now().UTC()
	entry.RunID, entry.State, entry.RunStatus, entry.DeliveredAt = task.ID, "processing", task.Status, &now
	entry.DeliveryIssue = nil
	if err = putConversationInbox(tx, scope, entry); err != nil {
		return nil, err
	}
	if err = putTaskTransaction(tx, task); err != nil {
		return nil, err
	}
	if err = conversationInputEvent(tx, scope, entry, "input_delivered", task.ID); err != nil {
		return nil, err
	}
	if err = tx.Commit(); err != nil {
		return nil, err
	}
	r.manager.tasks[task.ID] = task
	return copyTask(task)
}
