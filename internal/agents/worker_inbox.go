package agents

import (
	"encoding/json"
	"time"

	"github.com/takuphilchan/offgrid-llm/pkg/api"
)

// Delivery is durable before application. If the service dies between the two,
// the existing PendingInstructions checkpoint applies that message once on an
// explicit continuation. Re-delivery does not create another instruction.
func (m *Manager) deliverWorkerMessages(id string) error {
	db, err := m.controlDatabase()
	if err != nil {
		return err
	}
	defer db.Close()
	m.mu.Lock()
	defer m.mu.Unlock()
	task := m.tasks[id]
	if task == nil || task.Worker == nil {
		return ErrTaskNotFound
	}
	if task.Checkpoint == nil || task.Checkpoint.ExecutingCall != "" {
		return ErrRunConflict
	}
	scope := ControlScope{task.Worker.Workspace, task.Actor}
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err = tx.Exec(`UPDATE agent_schema SET version=version WHERE singleton=1`); err != nil {
		return err
	}
	changed := false
	for _, entry := range task.PendingInstructions {
		if !task.WorkerMessageRequests[entry.RequestID] {
			continue
		}
		message, err := readWorkerMessage(tx, scope, task.Worker.SessionID, entry.RequestID)
		if err != nil {
			return err
		}
		if message.State != "accepted" {
			continue
		}
		now := time.Now().UTC()
		message.State, message.DeliveredAt = "delivered", &now
		if err = putWorkerMessage(tx, scope, message); err != nil {
			return err
		}
		changed = true
	}
	if !changed {
		return nil
	}
	record, err := readControlRecord(tx, scope, task.Worker.SessionID, "worker")
	if err != nil {
		return err
	}
	var worker api.WorkerSession
	if err = json.Unmarshal(record.Snapshot, &worker); err != nil {
		return err
	}
	worker.Revision++
	if err = putControlRecord(tx, scope, worker.ID, "worker", worker.Revision, &worker); err != nil {
		return err
	}
	if _, err = appendControlEvent(tx, scope, worker.ID, worker.Revision, "worker_messages_delivered", &worker, ""); err != nil {
		return err
	}
	return tx.Commit()
}
