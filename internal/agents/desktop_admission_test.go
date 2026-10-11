package agents

import (
	"context"
	"strings"
	"testing"
)

func TestDesktopAdmissionHeldDurablyBeforeModelAndResolvesOnce(t *testing.T) {
	dir := t.TempDir()
	m := NewManagerWithPersistence(nil, nil, nil, dir)
	r := NewRunner(m, &runTestTools{}, nil)
	cfg := DefaultAgentConfig()
	cfg.TaskFirst = true
	cfg.RequestedComputerScope = "desktop"
	cfg.RequestedComputerApprovalMode = "full_task"
	task, created, err := r.CreateRequest("Open Chrome", "model", "alice", cfg, "request", strings.Repeat("a", 64))
	if err != nil || !created || task.Status != TaskInput || task.PendingInput.Mode != "desktop" {
		t.Fatal(task, created, err)
	}
	if _, err = r.Continue(context.Background(), task.ID, "alice", "start", "", false); err == nil {
		t.Fatal("held work ran before consent")
	}
	m2 := NewManagerWithPersistence(nil, nil, nil, dir)
	r2 := NewRunner(m2, &runTestTools{}, nil)
	again, created, err := r2.CreateRequest("Open Chrome", "model", "alice", cfg, "request", strings.Repeat("a", 64))
	if err != nil || created || again.ID != task.ID || again.PendingInput.ID != task.PendingInput.ID {
		t.Fatal("lost acknowledgment duplicated admission", err)
	}
	allowed := task.Config
	allowed.ComputerSession = "host"
	allowed.ComputerDriver = "windows-uia"
	allowed.ComputerApprovalMode = "full_task"
	allowed.ComputerScope = "desktop"
	for _, bad := range []string{"actor", "scope", "policy", "input"} {
		actor, input, c := task.Actor, task.PendingInput.ID, allowed
		switch bad {
		case "actor":
			actor = "bob"
		case "scope":
			c.ComputerScope = ""
		case "policy":
			c.ComputerApprovalMode = "ask_every_time"
		case "input":
			input = "other"
		}
		if _, err = r2.ResolveComputerInput(task.ID, actor, input, c); err == nil {
			t.Fatal("invalid resolution", bad)
		}
	}
	resolved, err := r2.ResolveComputerInput(task.ID, "alice", task.PendingInput.ID, allowed)
	if err != nil || resolved.Status != TaskPending || len(resolved.Checkpoint.Messages) != 2 || len(resolved.Steps) != 1 {
		t.Fatalf("invalid initial resolution: %+v %v", resolved, err)
	}
	again, err = r2.ResolveComputerInput(task.ID, "alice", task.PendingInput.ID, allowed)
	if err != nil || len(again.Steps) != 1 {
		t.Fatal("resolution duplicated", err)
	}
	m3 := NewManagerWithPersistence(nil, nil, nil, dir)
	saved, _ := m3.GetTask(task.ID)
	if !saved.ComputerSessionExpired || saved.Status != TaskInterrupted {
		t.Fatal("restart retained desktop authority")
	}
}
