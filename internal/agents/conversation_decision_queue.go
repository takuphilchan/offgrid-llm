package agents

import (
	"encoding/json"

	"github.com/takuphilchan/offgrid-llm/pkg/api"
)

// ConversationDecisions is a bounded service-only scan, separate from delivery:
// a decision waiting on an in-flight effect holds no generation/loading slot.
func (m *Manager) ConversationDecisions(workspace string, after int64, limit int) (*ConversationDeliveryPage, error) {
	if !validProfileText(workspace, 256, true) || after < 0 || limit < 1 || limit > 100 {
		return nil, ErrControlCommand
	}
	db, err := m.controlDatabase()
	if err != nil {
		return nil, err
	}
	defer db.Close()
	rows, err := db.Query(`SELECT sequence,actor,payload FROM agent_conversation_inbox
WHERE workspace=? AND sequence>? AND state='responded'
AND json_extract(payload,'$.decision.kind') IS NOT NULL
AND json_extract(payload,'$.applied_at') IS NULL ORDER BY sequence LIMIT ?`, workspace, after, limit+1)
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
		if item.Input.RunID == "" || item.Input.State != "responded" || item.Input.AppliedAt != nil || item.Input.Sequence != page.NextCursor || !validConversationDecision(item.Input.Decision) {
			return nil, ErrRunStorage
		}
		page.Items = append(page.Items, item)
	}
	return page, rows.Err()
}

func (m *Manager) DeferConversationDecision(scope ControlScope, conversation, input string, issue api.ConversationDeliveryIssue) error {
	if !validProfileOwner(scope.Workspace, scope.Actor) || !validTaskID(conversation) || !validTaskID(input) || !validProfileText(issue.Code, 128, true) || !validProfileText(issue.Message, 2048, true) || issue.RetryAt.IsZero() {
		return ErrControlCommand
	}
	db, err := m.controlDatabase()
	if err != nil {
		return err
	}
	defer db.Close()
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err = tx.Exec(`UPDATE agent_schema SET version=version WHERE singleton=1`); err != nil {
		return err
	}
	entry, err := readConversationInbox(tx, scope, conversation, input)
	if err != nil {
		return err
	}
	if entry.State != "responded" || entry.Decision == nil || entry.AppliedAt != nil {
		return ErrRunConflict
	}
	if entry.DecisionIssue != nil && *entry.DecisionIssue == issue {
		return nil
	}
	entry.DecisionIssue = &issue
	if err = putConversationInbox(tx, scope, entry); err != nil {
		return err
	}
	if err = conversationInputEvent(tx, scope, entry, "input_decision_deferred", ""); err != nil {
		return err
	}
	return tx.Commit()
}
