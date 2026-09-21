package agents

import (
	"encoding/json"
	"errors"
	"fmt"
	"time"
)

var (
	ErrTaskNotFound = errors.New("task not found")
	ErrRunConflict  = errors.New("run state does not allow this operation")
	ErrRunStorage   = errors.New("agent task storage unavailable")
)

func validTaskID(id string) bool {
	if id == "" || len(id) > 128 {
		return false
	}
	for _, c := range id {
		if !(c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || c == '-' || c == '_') {
			return false
		}
	}
	return true
}

func copyTask(task *Task) (*Task, error) {
	data, err := json.Marshal(task)
	if err != nil {
		return nil, err
	}
	var copy Task
	if err := json.Unmarshal(data, &copy); err != nil {
		return nil, err
	}
	copy.cancel = task.cancel
	return &copy, nil
}

// saveTask is called with m.mu held. A successful response means a complete
// snapshot reached disk, not merely an in-memory status update.
func (m *Manager) saveTask(task *Task) (saveErr error) {
	if !validTaskID(task.ID) {
		return fmt.Errorf("invalid task ID")
	}
	if m.storageErr != nil {
		return fmt.Errorf("%w: %v", ErrRunStorage, m.storageErr)
	}
	if m.dataDir == "" {
		return nil
	}
	defer func() {
		if saveErr != nil {
			m.storageErr = saveErr
		}
	}()
	if err := persistTask(m.dataDir, task); err != nil {
		return fmt.Errorf("%w: %v", ErrRunStorage, err)
	}
	return nil
}

// loadTasks only runs before execution begins. It never resumes side effects.
func (m *Manager) loadTasks() {
	if m.dataDir == "" {
		return
	}
	tasks, err := loadTaskDatabase(m.dataDir)
	if err != nil {
		m.storageErr = err
		return
	}
	loaded := make(map[string]*Task, len(tasks))
	for _, task := range tasks {
		changed := false
		if task.Config.ComputerSession != "" && !task.ComputerSessionExpired && task.Status != TaskCompleted && task.Status != TaskCancelled && task.Status != TaskFailed {
			task.ComputerSessionExpired = true
			changed = true
		}
		// A stored computer approval is not fresh local consent. The companion
		// session expires on service restart even if the grant's TTL has not.
		if task.Config.ComputerSession != "" && task.PendingApproval != nil {
			task.PendingApproval = nil
			if task.Status == TaskWaiting {
				task.Status = TaskInterrupted
				task.Error = "Computer session ended when the service stopped. Start a new locally consented session; previous approvals are no longer valid."
			}
			changed = true
		}
		if task.Status == TaskRunning || task.Status == TaskPending {
			task.Status = TaskInterrupted
			task.Error = "Service stopped before this run finished. Review and resume explicitly."
			if task.Checkpoint != nil && task.Checkpoint.ExecutingCall != "" {
				task.Status = TaskUncertain
				task.Error = "Service stopped during a tool call. Its outcome is unknown; inspect the target before resolving this run."
			}
			changed = true
		}
		if task.Status == TaskWaiting && (task.PendingApproval == nil || task.Checkpoint == nil) {
			task.Status = TaskInterrupted
			task.Error = "Legacy approval has no resumable checkpoint. Review past actions before starting a new task."
			changed = true
		}
		if changed {
			if err := m.saveTask(task); err != nil {
				m.storageErr = err
				return
			}
		}
		loaded[task.ID] = task
	}
	m.tasks = loaded
}

func (m *Manager) StorageError() error {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.storageErr != nil {
		return fmt.Errorf("%w: %v", ErrRunStorage, m.storageErr)
	}
	return nil
}

// updateTask publishes a new immutable snapshot only after persistence succeeds.
func (m *Manager) updateTask(id string, change func(*Task) error) (*Task, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	task, ok := m.tasks[id]
	if !ok || task.DeletedAt != nil {
		return nil, ErrTaskNotFound
	}
	next, err := copyTask(task)
	if err != nil {
		return nil, err
	}
	if err := change(next); err != nil {
		return nil, err
	}
	if err := m.saveTask(next); err != nil {
		return nil, err
	}
	m.tasks[id] = next
	return copyTask(next)
}

func finishTask(task *Task, status TaskStatus, message string) {
	now := time.Now().UTC()
	task.Status, task.Error, task.CompletedAt = status, message, &now
	task.PendingApproval = nil
}
