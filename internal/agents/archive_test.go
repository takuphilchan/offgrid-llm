package agents

import (
	"context"
	"errors"
	"testing"
)

func TestArchivePreservesUncertainEvidenceAndRestart(t *testing.T) {
	dir := t.TempDir()
	m := NewManagerWithPersistence(nil, nil, nil, dir)
	r := NewRunner(m, nil, nil)
	task, err := r.Create("historical test", "model", "alice", DefaultAgentConfig())
	if err != nil {
		t.Fatal(err)
	}
	_, err = m.updateTask(task.ID, func(t *Task) error {
		t.Status = TaskUncertain
		t.Result = "original evidence"
		t.Checkpoint.ExecutingCall = "unresolved-call"
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = r.Archive(task.ID, "bob", true); !errors.Is(err, ErrTaskNotFound) {
		t.Fatalf("cross-owner archive: %v", err)
	}
	archived, err := r.Archive(task.ID, "alice", true)
	if err != nil {
		t.Fatal(err)
	}
	again, err := r.Archive(task.ID, "alice", true)
	if err != nil || !archived.ArchivedAt.Equal(*again.ArchivedAt) {
		t.Fatal("archive was not idempotent", err)
	}
	reloaded := NewManagerWithPersistence(nil, nil, nil, dir)
	saved, ok := reloaded.GetTask(task.ID)
	if !ok || saved.ArchivedAt == nil || saved.Status != TaskUncertain || saved.Result != "original evidence" || saved.Checkpoint.ExecutingCall != "unresolved-call" {
		t.Fatalf("evidence changed: %+v", saved)
	}
	r = NewRunner(reloaded, nil, nil)
	if _, err = r.Continue(context.Background(), task.ID, "alice", "resume", "", false); !errors.Is(err, ErrRunConflict) {
		t.Fatalf("archived work resumed: %v", err)
	}
	if err = r.Delete(task.ID, "alice"); !errors.Is(err, ErrRunConflict) {
		t.Fatalf("uncertain work deleted: %v", err)
	}
	restored, err := r.Archive(task.ID, "alice", false)
	if err != nil || restored.ArchivedAt != nil || restored.Status != TaskUncertain || restored.Checkpoint.ExecutingCall != "unresolved-call" || len(r.active) != 0 {
		t.Fatalf("restore executed or altered work: %+v %v", restored, err)
	}
}

func TestArchiveRejectsActiveApprovalChildrenAndStorageFailure(t *testing.T) {
	m := NewManagerWithPersistence(nil, nil, nil, t.TempDir())
	r := NewRunner(m, nil, nil)
	task, err := r.Create("root", "model", "alice", DefaultAgentConfig())
	if err != nil {
		t.Fatal(err)
	}
	if CanArchiveTask(&Task{Status: TaskChildren}) {
		t.Fatal("waiting parent is archivable")
	}
	for _, status := range []TaskStatus{TaskPending, TaskRunning, TaskWaiting} {
		m.updateTask(task.ID, func(t *Task) error { t.Status = status; return nil })
		if _, err = r.Archive(task.ID, "alice", true); !errors.Is(err, ErrRunConflict) {
			t.Fatalf("archived %s: %v", status, err)
		}
	}
	m.updateTask(task.ID, func(t *Task) error { t.Status = TaskUncertain; return nil })
	r.active[task.ID] = func() {}
	if _, err = r.Archive(task.ID, "alice", true); !errors.Is(err, ErrRunConflict) {
		t.Fatalf("active worker archived: %v", err)
	}
	delete(r.active, task.ID)
	m.updateTask(task.ID, func(t *Task) error {
		t.PendingApproval = &Approval{ID: "approval", RunID: t.ID, Actor: t.Actor, CallID: "call", Tool: "read", Arguments: []byte(`{}`), ArgumentsJSON: `{}`}
		return nil
	})
	if _, err = r.Archive(task.ID, "alice", true); !errors.Is(err, ErrRunConflict) {
		t.Fatalf("approval archived: %v", err)
	}
	child, err := r.Create("child", "model", "alice", DefaultAgentConfig())
	if err != nil {
		t.Fatal(err)
	}
	m.updateTask(task.ID, func(t *Task) error { t.PendingApproval = nil; t.ChildHistory = []ChildLink{{ID: child.ID}}; return nil })
	if _, err = r.Archive(task.ID, "alice", true); !errors.Is(err, ErrRunConflict) {
		t.Fatalf("active child archived: %v", err)
	}
	m.updateTask(child.ID, func(t *Task) error { t.Status = TaskUncertain; t.ParentID = task.ID; return nil })
	if _, err = r.Archive(child.ID, "alice", true); !errors.Is(err, ErrRunConflict) {
		t.Fatalf("child independently archived: %v", err)
	}
	m.storageErr = errors.New("full disk")
	if _, err = r.Archive(task.ID, "alice", true); !errors.Is(err, ErrRunStorage) {
		t.Fatalf("storage failure ignored: %v", err)
	}
	saved, _ := m.GetTask(task.ID)
	if saved.ArchivedAt != nil {
		t.Fatal("failed archive persisted")
	}
}
