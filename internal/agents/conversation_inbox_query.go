package agents

import (
	"encoding/json"

	"github.com/takuphilchan/offgrid-llm/pkg/api"
)

func (m *Manager) ConversationInput(scope ControlScope, conversation, input string) (*api.ConversationInboxEntry, error) {
	if !validProfileOwner(scope.Workspace, scope.Actor) || !validTaskID(conversation) || !validTaskID(input) {
		return nil, ErrControlNotFound
	}
	db, err := m.controlDatabase()
	if err != nil {
		return nil, err
	}
	defer db.Close()
	return readConversationInbox(db, scope, conversation, input)
}

func (m *Manager) ConversationInbox(scope ControlScope, conversation string, after int64, limit int) (*api.ConversationInboxPage, error) {
	if after < 0 || limit < 1 || limit > 200 {
		return nil, ErrControlCommand
	}
	db, err := m.controlDatabase()
	if err != nil {
		return nil, err
	}
	defer db.Close()
	if _, err = readControlRecord(db, scope, conversation, "conversation"); err != nil {
		return nil, err
	}
	rows, err := db.Query(`SELECT sequence,payload FROM agent_conversation_inbox WHERE workspace=? AND actor=? AND conversation_id=? AND sequence>? ORDER BY sequence LIMIT ?`, scope.Workspace, scope.Actor, conversation, after, limit+1)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	page := &api.ConversationInboxPage{Items: []api.ConversationInboxEntry{}, NextCursor: after}
	for rows.Next() {
		if len(page.Items) == limit {
			page.HasMore = true
			break
		}
		var data []byte
		if err = rows.Scan(&page.NextCursor, &data); err != nil {
			return nil, err
		}
		var entry api.ConversationInboxEntry
		if err = json.Unmarshal(data, &entry); err != nil {
			return nil, err
		}
		page.Items = append(page.Items, entry)
	}
	return page, rows.Err()
}
