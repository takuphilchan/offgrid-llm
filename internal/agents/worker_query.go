package agents

import (
	"encoding/json"

	"github.com/takuphilchan/offgrid-llm/pkg/api"
)

func (m *Manager) WorkerMessages(scope ControlScope, worker string, after int64, limit int) (*api.WorkerMessagePage, error) {
	if after < 0 || limit < 1 || limit > 200 {
		return nil, ErrControlCommand
	}
	db, err := m.controlDatabase()
	if err != nil {
		return nil, err
	}
	defer db.Close()
	if _, err = readControlRecord(db, scope, worker, "worker"); err != nil {
		return nil, err
	}
	rows, err := db.Query(`SELECT sequence,payload FROM agent_worker_messages WHERE workspace=? AND actor=? AND worker_id=? AND sequence>? ORDER BY sequence LIMIT ?`, scope.Workspace, scope.Actor, worker, after, limit+1)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	page := &api.WorkerMessagePage{Items: []api.WorkerMessage{}, NextCursor: after}
	for rows.Next() {
		if len(page.Items) == limit {
			page.HasMore = true
			break
		}
		var data []byte
		if err = rows.Scan(&page.NextCursor, &data); err != nil {
			return nil, err
		}
		var message api.WorkerMessage
		if err = json.Unmarshal(data, &message); err != nil {
			return nil, err
		}
		page.Items = append(page.Items, message)
	}
	return page, rows.Err()
}
