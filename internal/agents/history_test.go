package agents

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestHistoryDeletionOwnershipStatesAndRestart(t *testing.T) {
	dir := t.TempDir()
	m := NewManagerWithPersistence(nil, nil, nil, dir)
	r := NewRunner(m, nil, nil)
	for _, status := range []TaskStatus{TaskPending, TaskRunning, TaskWaiting, TaskInterrupted, TaskUncertain, TaskCompleted, TaskFailed, TaskCancelled} {
		t.Run(string(status), func(t *testing.T) {
			task, err := r.Create("private history 日本語", "model", "alice", DefaultAgentConfig())
			if err != nil {
				t.Fatal(err)
			}
			_, err = m.updateTask(task.ID, func(task *Task) error { task.Status = status; task.Result = "private result"; return nil })
			if err != nil {
				t.Fatal(err)
			}
			if err := r.Delete(task.ID, "bob"); !errors.Is(err, ErrTaskNotFound) {
				t.Fatalf("cross-owner deletion: %v", err)
			}
			err = r.Delete(task.ID, "alice")
			if status != TaskCompleted && status != TaskFailed && status != TaskCancelled {
				if !errors.Is(err, ErrRunConflict) {
					t.Fatalf("unsafe deletion: %v", err)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if _, ok := m.GetTask(task.ID); ok {
				t.Fatal("deleted task still visible")
			}
			data, err := os.ReadFile(filepath.Join(dir, "agent_tasks", task.ID+".json"))
			if err != nil || strings.Contains(string(data), "private") || strings.Contains(string(data), "checkpoint") {
				t.Fatalf("history was not scrubbed: %s %v", data, err)
			}
			reloaded := NewManagerWithPersistence(nil, nil, nil, dir)
			if !reloaded.IsDeleted(task.ID) {
				t.Fatal("deletion lost on restart")
			}
			if _, ok := reloaded.GetTask(task.ID); ok {
				t.Fatal("deleted task restored")
			}
			for _, item := range reloaded.ListTasks() {
				if item.ID == task.ID {
					t.Fatal("deleted task listed")
				}
			}
			if _, err := NewRunner(reloaded, nil, nil).Continue(context.Background(), task.ID, "alice", "resume", "", false); !errors.Is(err, ErrTaskNotFound) {
				t.Fatalf("deleted task resumed: %v", err)
			}
		})
	}
}

func TestHistoryDeletionRejectsWorkersUnresolvedCallsAndStorageFailure(t *testing.T) {
	m := NewManagerWithPersistence(nil, nil, nil, t.TempDir())
	r := NewRunner(m, nil, nil)
	task, err := r.Create("keep me", "model", "alice", DefaultAgentConfig())
	if err != nil {
		t.Fatal(err)
	}
	_, err = m.updateTask(task.ID, func(task *Task) error {
		task.Status = TaskCancelled
		task.Checkpoint.ExecutingCall = "uncertain-call"
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := r.Delete(task.ID, "alice"); !errors.Is(err, ErrRunConflict) {
		t.Fatalf("lost uncertain outcome: %v", err)
	}
	_, err = m.updateTask(task.ID, func(task *Task) error { task.Checkpoint.ExecutingCall = ""; return nil })
	if err != nil {
		t.Fatal(err)
	}
	r.active[task.ID] = func() {}
	if err := r.Delete(task.ID, "alice"); !errors.Is(err, ErrRunConflict) {
		t.Fatalf("deleted settling worker: %v", err)
	}
	delete(r.active, task.ID)
	m.storageErr = errors.New("disk failure")
	if err := r.Delete(task.ID, "alice"); !errors.Is(err, ErrRunStorage) {
		t.Fatalf("reported success on storage failure: %v", err)
	}
	if _, ok := m.GetTask(task.ID); !ok {
		t.Fatal("lost task after failed deletion")
	}
}
