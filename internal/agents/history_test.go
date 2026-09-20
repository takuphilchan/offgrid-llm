package agents

import (
	"context"
	"errors"
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
			db, err := openTaskDatabase(dir)
			if err != nil {
				t.Fatal(err)
			}
			var data []byte
			err = db.QueryRow(`SELECT snapshot FROM agent_tasks WHERE id=?`, task.ID).Scan(&data)
			db.Close()
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

func TestLegacyInterruptedHistoryDeletion(t *testing.T) {
	dir := t.TempDir()
	m := NewManagerWithPersistence(nil, nil, nil, dir)
	r := NewRunner(m, nil, nil)
	task, err := r.Create("legacy task", "model", "alice", DefaultAgentConfig())
	if err != nil {
		t.Fatal(err)
	}
	_, err = m.updateTask(task.ID, func(task *Task) error {
		task.Actor = ""
		task.Status = TaskWaiting
		task.Checkpoint = nil
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	// Exercise the real legacy loader, not a synthetic terminal record.
	m = NewManagerWithPersistence(nil, nil, nil, dir)
	r = NewRunner(m, nil, nil)
	loaded, ok := m.GetTask(task.ID)
	if !ok || loaded.Status != TaskInterrupted || !CanDeleteTask(loaded) {
		t.Fatalf("unrecoverable legacy record should be removable: %#v", loaded)
	}
	if err := r.Delete(task.ID, "alice"); !errors.Is(err, ErrTaskNotFound) {
		t.Fatalf("legacy history must remain local-admin controlled: %v", err)
	}
	r.active[task.ID] = func() {}
	if err := r.Delete(task.ID, "local-admin"); !errors.Is(err, ErrRunConflict) {
		t.Fatalf("settling worker must remain protected: %v", err)
	}
	delete(r.active, task.ID)
	if err := r.Delete(task.ID, "local-admin"); err != nil {
		t.Fatal(err)
	}
	m = NewManagerWithPersistence(nil, nil, nil, dir)
	if !m.IsDeleted(task.ID) || len(m.ListTasks()) != 0 {
		t.Fatal("legacy history resurrected")
	}
}

func TestInterruptedDeletionRequiresNoRecoveryState(t *testing.T) {
	for _, task := range []*Task{
		{Status: TaskInterrupted, Checkpoint: &Checkpoint{}},
		{Status: TaskInterrupted, Checkpoint: &Checkpoint{ExecutingCall: "unknown-call"}},
		{Status: TaskInterrupted, PendingApproval: &Approval{ID: "approval"}},
		{Status: TaskUncertain},
		{Status: TaskRunning},
		{Status: TaskPending},
	} {
		if CanDeleteTask(task) {
			t.Fatalf("unsafe deletion permitted: %#v", task)
		}
	}
	if !CanDeleteTask(&Task{Status: TaskInterrupted, Actor: "alice"}) {
		t.Fatal("interrupted history without recovery state cannot be removed")
	}
}
