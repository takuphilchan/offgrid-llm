package agents

import (
	"time"

	"github.com/takuphilchan/offgrid-llm/pkg/api"
)

// IncorporateConversationInput applies an explicit foreground decision at a
// settled checkpoint. It is service-owned, not a client approval/resume action.
// Answering a side question preserves the work budget and instruction version;
// a correction applies the exact accepted user text, not generated prose.
func (r *Runner) IncorporateConversationInput(scope ControlScope, conversationID, inputID, runID, kind string, instructionVersion uint64) (*api.ConversationInboxEntry, error) {
	if kind != "answered" && kind != "correction" {
		return nil, ErrControlCommand
	}
	if !validProfileOwner(scope.Workspace, scope.Actor) {
		return nil, ErrControlNotFound
	}
	// Check external authority outside the runner/store locks. Foreground side
	// questions may omit revoked context; corrections must still have access to
	// the work they modify, independently of their foreground run's sources.
	if kind == "correction" {
		entry, err := r.manager.ConversationInput(scope, conversationID, inputID)
		if err != nil {
			return nil, err
		}
		work, err := r.manager.WorkItem(scope, entry.WorkID)
		if err != nil {
			return nil, err
		}
		root, found := r.manager.GetTask(work.RunID)
		if !found || root.Actor != scope.Actor {
			return nil, ErrControlNotFound
		}
		if err = r.checkSourceAccess(root); err != nil {
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
	foreground := r.manager.tasks[runID]
	if entry.RunID != runID || foreground == nil || foreground.DeletedAt != nil || foreground.Actor != scope.Actor || foreground.ConversationTurn == nil || foreground.ConversationTurn.InputID != inputID || foreground.Origin == nil || foreground.Origin.Workspace != scope.Workspace || foreground.Origin.Conversation != conversationID {
		return nil, ErrControlNotFound
	}
	if entry.AppliedAt != nil {
		if entry.Incorporation != kind {
			return nil, ErrRunConflict
		}
		return entry, nil
	}
	if entry.Decision != nil && (entry.Decision.Kind != kind || kind == "correction" && conversationDecisionVersion(entry) != instructionVersion) {
		return nil, ErrRunConflict
	}
	if foreground.Status != TaskRunning && foreground.Status != TaskCompleted || foreground.ControlAction != "" || entry.State != "processing" && entry.State != "responded" {
		return nil, ErrRunConflict
	}
	if kind == "answered" && (entry.Input.Intent == "correction" || entry.Input.Intent == "new_work" || entry.Input.Intent == "answer") || kind == "correction" && entry.WorkID == "" {
		return nil, ErrControlCommand
	}
	batch, err := r.conversationInputGraph(tx, scope, entry)
	if err != nil {
		return nil, err
	}
	if kind == "correction" {
		if err = prepareInboxCorrection(batch, entry, instructionVersion); err != nil {
			return nil, err
		}
	}
	for _, task := range batch {
		releaseConversationInput(task, inputID)
		if err = putTaskTransaction(tx, task); err != nil {
			return nil, err
		}
	}
	now := time.Now().UTC()
	entry.Incorporation, entry.AppliedAt = kind, &now
	entry.DecisionIssue = nil
	if err = putConversationInbox(tx, scope, entry); err != nil {
		return nil, err
	}
	if err = conversationInputEvent(tx, scope, entry, "input_applied", ""); err != nil {
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
