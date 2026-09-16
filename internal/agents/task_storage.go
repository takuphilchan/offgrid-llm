package agents

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
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
	dir := filepath.Join(m.dataDir, "agent_tasks")
	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("%w: %v", ErrRunStorage, err)
	}
	data, err := json.MarshalIndent(task, "", "  ")
	if err != nil {
		return fmt.Errorf("%w: %v", ErrRunStorage, err)
	}
	file, err := os.CreateTemp(dir, ".task-*")
	if err != nil {
		return fmt.Errorf("%w: %v", ErrRunStorage, err)
	}
	defer os.Remove(file.Name())
	if _, err = file.Write(data); err == nil {
		err = file.Sync()
	}
	if closeErr := file.Close(); err == nil {
		err = closeErr
	}
	if err == nil {
		err = os.Rename(file.Name(), filepath.Join(dir, task.ID+".json"))
	}
	if err != nil {
		return fmt.Errorf("%w: %v", ErrRunStorage, err)
	}
	return nil
}

// loadTasks only runs before execution begins. It never resumes side effects.
func (m *Manager) loadTasks() {
	if m.dataDir == "" {
		return
	}
	dir := filepath.Join(m.dataDir, "agent_tasks")
	entries, err := os.ReadDir(dir)
	if os.IsNotExist(err) {
		return
	}
	if err != nil {
		m.storageErr = err
		return
	}
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(dir, entry.Name()))
		if err != nil {
			m.storageErr = err
			return
		}
		var task Task
		if err = json.Unmarshal(data, &task); err != nil {
			m.storageErr = fmt.Errorf("invalid task snapshot %s: %w", entry.Name(), err)
			return
		}
		if !validTaskID(task.ID) || entry.Name() != task.ID+".json" {
			m.storageErr = fmt.Errorf("invalid task snapshot identity")
			return
		}
		changed := false
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
			if err := m.saveTask(&task); err != nil {
				m.storageErr = err
				return
			}
		}
		m.tasks[task.ID] = &task
	}
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
