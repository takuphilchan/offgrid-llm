package agents

import (
	"encoding/json"
	"time"

	"github.com/takuphilchan/offgrid-llm/pkg/api"
)

// AdmitConversationInputWork atomically turns one completed foreground decision
// into one work identity. Its source input stays in history once. Retrying an
// admitted decision returns the saved identity before consulting new config.
func (r *Runner) AdmitConversationInputWork(scope ControlScope, conversationID, inputID string, expectedRevision int64, model string, config AgentConfig) (*api.ConversationInboxEntry, error) {
	if !validProfileOwner(scope.Workspace, scope.Actor) {
		return nil, ErrControlNotFound
	}
	input, err := r.manager.ConversationInput(scope, conversationID, inputID)
	if err != nil {
		return nil, err
	}
	foreground, found := r.manager.GetTask(input.RunID)
	if !found || foreground.Actor != scope.Actor || foreground.ConversationTurn == nil || foreground.ConversationTurn.InputID != inputID || foreground.Origin == nil || foreground.Origin.Workspace != scope.Workspace || foreground.Origin.Conversation != conversationID {
		return nil, ErrControlNotFound
	}
	if input.AppliedAt == nil {
		if err = r.checkSourceAccess(foreground); err != nil {
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
	if entry.Decision == nil || entry.Decision.Kind != "new_work" || validateConversationDecision(entry, entry.Decision) != nil {
		return nil, ErrControlCommand
	}
	if entry.AppliedAt != nil {
		if entry.Incorporation != "new_work" || entry.CreatedWorkID == "" {
			return nil, ErrRunConflict
		}
		if _, err = linkedInputWork(tx, scope, conversationID, entry.CreatedWorkID); err != nil {
			return nil, err
		}
		return entry, nil
	}
	current := r.manager.tasks[entry.RunID]
	if current == nil || current.ID != foreground.ID || current.Status != TaskCompleted || current.ControlAction != "" || entry.State != "responded" || entry.RunStatus != TaskCompleted {
		return nil, ErrRunConflict
	}
	record, err := readControlRecord(tx, scope, conversationID, "conversation")
	if err != nil {
		return nil, err
	}
	if record.Revision != expectedRevision {
		return nil, ErrRunConflict
	}
	var conversation Conversation
	if err = json.Unmarshal(record.Snapshot, &conversation); err != nil {
		return nil, err
	}
	if err = validatePinnedPreference(tx, scope, conversation.Preferences); err != nil {
		return nil, err
	}
	if entry.Input.Model != "" && entry.Input.Model != model {
		return nil, ErrRunConflict
	}
	// Only fresh service resolution may supply preferences; existing live host
	// bindings and automation identities can never transfer from another run.
	if config.Execution != nil || config.ComputerSession != "" || config.Automation != nil {
		return nil, ErrControlCommand
	}
	batch, err := r.conversationInputGraph(tx, scope, entry)
	if err != nil {
		return nil, err
	}
	workInput := &ConversationInput{Content: entry.Input.Content, Model: model, Acceptance: []string{entry.Decision.ExpectedResult}}
	task, err := prepareConversationRun(workInput.Content, model, scope.Actor, config, &ConversationOrigin{Workspace: scope.Workspace, Conversation: conversationID, TurnID: inputID})
	if err != nil {
		return nil, err
	}
	task.RequestID = "input-work-" + inputID
	task.RequestDigest = commandDigest(ConversationCommand{RequestID: inputID, TargetID: conversationID, Kind: "foreground_new_work", Payload: ConversationCommandPayload{Input: workInput}})
	work, err := createConversationWork(tx, scope, &conversation, workInput, task)
	if err != nil {
		return nil, err
	}
	for _, previous := range batch {
		releaseConversationInput(previous, inputID)
		if err = putTaskTransaction(tx, previous); err != nil {
			return nil, err
		}
	}
	if _, err = tx.Exec(`INSERT INTO agent_control_links(workspace,actor,conversation_id,work_id) VALUES(?,?,?,?)`, scope.Workspace, scope.Actor, conversationID, work.ID); err != nil {
		return nil, err
	}
	conversation.Revision++
	if err = putControlRecord(tx, scope, conversationID, "conversation", conversation.Revision, &conversation); err != nil {
		return nil, err
	}
	now := time.Now().UTC()
	entry.CreatedWorkID, entry.Incorporation, entry.AppliedAt, entry.DecisionIssue = work.ID, "new_work", &now, nil
	if err = putConversationInbox(tx, scope, entry); err != nil {
		return nil, err
	}
	if _, err = appendControlEvent(tx, scope, conversationID, conversation.Revision, "input_work_admitted", entry, task.ID); err != nil {
		return nil, err
	}
	if err = tx.Commit(); err != nil {
		return nil, err
	}
	for _, previous := range batch {
		r.manager.tasks[previous.ID] = previous
	}
	r.manager.tasks[task.ID] = task
	return entry, nil
}
