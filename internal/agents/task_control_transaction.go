package agents

import "database/sql"

// Used when a run checkpoint and canonical control state must become visible
// together. Open the control store before taking the task publication lock.
func (m *Manager) updateTaskControl(id string, change func(*Task, *sql.Tx) error) (*Task, error) {
	db, err := m.controlDatabase()
	if err != nil {
		return nil, err
	}
	defer db.Close()
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.storageErr != nil {
		return nil, ErrRunStorage
	}
	task, ok := m.tasks[id]
	if !ok || task.DeletedAt != nil {
		return nil, ErrTaskNotFound
	}
	next, err := copyTask(task)
	if err != nil {
		return nil, err
	}
	tx, err := db.Begin()
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	if _, err = tx.Exec(`UPDATE agent_schema SET version=version WHERE singleton=1`); err != nil {
		return nil, err
	}
	if err = change(next, tx); err != nil {
		return nil, err
	}
	if err = putTaskTransaction(tx, next); err != nil {
		m.storageErr = err
		return nil, err
	}
	if err = tx.Commit(); err != nil {
		m.storageErr = err
		return nil, err
	}
	m.tasks[id] = next
	return copyTask(next)
}
