package agents

import (
	"encoding/json"

	"github.com/takuphilchan/offgrid-llm/pkg/api"
)

// Service-only candidate pages carry authoritative ownership. They are never a
// public cross-actor listing. Each conversation contributes only its queue head.
type ConversationDeliveryCandidate struct {
	Scope ControlScope
	Input api.ConversationInboxEntry
}

type ConversationDeliveryPage struct {
	Items      []ConversationDeliveryCandidate
	NextCursor int64
	HasMore    bool
}

func (m *Manager) ConversationDeliveries(workspace string, after int64, limit int) (*ConversationDeliveryPage, error) {
	if !validProfileText(workspace, 256, true) || after < 0 || limit < 1 || limit > 100 {
		return nil, ErrControlCommand
	}
	db, err := m.controlDatabase()
	if err != nil {
		return nil, err
	}
	defer db.Close()
	rows, err := db.Query(`SELECT i.sequence,i.actor,i.payload FROM agent_conversation_inbox i
WHERE i.workspace=? AND i.sequence>? AND i.state='accepted'
AND NOT EXISTS (SELECT 1 FROM agent_conversation_inbox p WHERE p.workspace=i.workspace AND p.actor=i.actor AND p.conversation_id=i.conversation_id AND (p.state='processing' OR (p.state='accepted' AND p.sequence<i.sequence)))
ORDER BY i.sequence LIMIT ?`, workspace, after, limit+1)
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
		if item.Input.RunID != "" || item.Input.State != "accepted" || item.Input.Sequence != page.NextCursor {
			return nil, ErrRunStorage
		}
		page.Items = append(page.Items, item)
	}
	return page, rows.Err()
}

// An unavailable model/context leaves the accepted input and graph fences in
// place with an inspectable limitation. It never creates a failed execution or
// spends model budget. A concurrent successful delivery wins this boundary.
func (m *Manager) DeferConversationDelivery(scope ControlScope, conversation, input string, issue api.ConversationDeliveryIssue) error {
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
	if entry.RunID != "" || entry.State != "accepted" {
		return ErrRunConflict
	}
	if entry.DeliveryIssue != nil && *entry.DeliveryIssue == issue {
		return nil
	}
	entry.DeliveryIssue = &issue
	if err = putConversationInbox(tx, scope, entry); err != nil {
		return err
	}
	if err = conversationInputEvent(tx, scope, entry, "input_delivery_deferred", ""); err != nil {
		return err
	}
	return tx.Commit()
}
