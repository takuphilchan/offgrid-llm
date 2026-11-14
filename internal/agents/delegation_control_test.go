package agents

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/takuphilchan/offgrid-llm/pkg/api"
)

func TestDelegationCorrectionFencesLateResultWithoutReopeningBudget(t *testing.T) {
	m := NewManagerWithPersistence(nil, nil, nil, t.TempDir())
	entered, release := make(chan struct{}), make(chan struct{})
	r := NewRunner(m, NewToolRegistry(), func(_ context.Context, task *Task, _ []api.ChatMessage, _ []api.Tool) (*api.ChatCompletionResponse, error) {
		if task.ParentID == "" {
			return taskCall("delegate_tasks", `{"tasks":[{"key":"research","goal":"Check the old source","tools":[]}]}`), nil
		}
		close(entered)
		<-release // Deliberately ignore generation cancellation to exercise late data.
		return runAnswer(api.ChatMessage{Role: "assistant", Content: "Old source is correct"}), nil
	})
	defer r.Shutdown(context.Background())
	cfg := taskConfig()
	cfg.MaxIterations = 12
	root, err := r.Create("Check a source", "model", "alice", cfg)
	if err != nil {
		t.Fatal(err)
	}
	root, err = r.Continue(context.Background(), root.ID, root.Actor, "start", "", false)
	if err != nil || root.Status != TaskChildren {
		t.Fatal(err, root.Status)
	}
	reserved := root.Checkpoint.ReservedChildIterations
	r.advanceDelegation(context.Background(), root.ID)
	select {
	case <-entered:
	case <-time.After(5 * time.Second):
		t.Fatal("child did not enter")
	}
	corrected, err := r.Steer(root.ID, root.Actor, "correction-1", "Use the revised source instead")
	if err != nil || len(corrected.PendingInstructions) != 1 || corrected.ControlEpoch != root.ControlEpoch+1 {
		close(release)
		t.Fatal("correction not accepted", err)
	}
	retried, err := r.Steer(root.ID, root.Actor, "correction-1", "Use the revised source instead")
	if err != nil || retried.ControlEpoch != corrected.ControlEpoch {
		close(release)
		t.Fatal("duplicate correction reapplied", err)
	}
	child, _ := m.GetTask(root.ChildHistory[0].ID)
	if child.Status != TaskInterrupted {
		close(release)
		t.Fatal("child not fenced")
	}
	r.advanceDelegation(context.Background(), root.ID)
	stillWaiting, _ := m.GetTask(root.ID)
	if len(stillWaiting.Instructions) != 0 {
		close(release)
		t.Fatal("correction applied before quiescence")
	}
	close(release)
	deadline := time.Now().Add(5 * time.Second)
	for {
		r.mu.Lock()
		active := len(r.active)
		r.mu.Unlock()
		if active == 0 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("generation did not settle")
		}
		time.Sleep(time.Millisecond)
	}
	r.advanceDelegation(context.Background(), root.ID)
	updated, _ := m.GetTask(root.ID)
	child, _ = m.GetTask(child.ID)
	if updated.Status != TaskPending || updated.Delegation != nil || len(updated.Instructions) != 1 || len(updated.PendingInstructions) != 0 || updated.InstructionVersion != 1 || updated.Checkpoint.ReservedChildIterations != reserved || child.Status != TaskCancelled || child.ErrorCode != "instruction_superseded" {
		t.Fatalf("incorrect settled correction: parent=%s version=%d child=%s budget=%d", updated.Status, updated.InstructionVersion, child.Status, updated.Checkpoint.ReservedChildIterations)
	}
	if strings.Contains(updated.Result, "Old source") {
		t.Fatal("stale result became completion")
	}
	if updated.ChildHistory[0].Status != TaskCancelled {
		t.Fatal("activity retained a running superseded worker")
	}
}

func TestDelegationCorrectionWaitsForUncertainChild(t *testing.T) {
	m := NewManager(nil, nil, nil)
	r := NewRunner(m, NewToolRegistry(), func(context.Context, *Task, []api.ChatMessage, []api.Tool) (*api.ChatCompletionResponse, error) {
		return taskCall("delegate_tasks", `{"tasks":[{"key":"a","goal":"Inspect","tools":[]}]}`), nil
	})
	defer r.Shutdown(context.Background())
	cfg := taskConfig()
	cfg.MaxIterations = 12
	root, _ := r.Create("Inspect", "model", "alice", cfg)
	root, err := r.Continue(context.Background(), root.ID, root.Actor, "start", "", false)
	if err != nil {
		t.Fatal(err)
	}
	_, err = m.updateTask(root.ChildHistory[0].ID, func(child *Task) error { child.Status = TaskUncertain; return nil })
	if err != nil {
		t.Fatal(err)
	}
	if _, err = r.Steer(root.ID, root.Actor, "uncertain-correction", "Use the revised source"); err != nil {
		t.Fatal(err)
	}
	r.advanceDelegation(context.Background(), root.ID)
	root, _ = m.GetTask(root.ID)
	if len(root.PendingInstructions) != 1 || len(root.Instructions) != 0 || root.Status != TaskChildren {
		t.Fatal("unresolved child effect was bypassed")
	}
	child, _ := m.GetTask(root.ChildHistory[0].ID)
	if child.Status != TaskUncertain {
		t.Fatal("uncertain child was implicitly resumed")
	}
}

func TestPausedDelegationCorrectionDoesNotResume(t *testing.T) {
	m := NewManager(nil, nil, nil)
	r := NewRunner(m, NewToolRegistry(), func(context.Context, *Task, []api.ChatMessage, []api.Tool) (*api.ChatCompletionResponse, error) {
		return taskCall("delegate_tasks", `{"tasks":[{"key":"a","goal":"Inspect","tools":[]}]}`), nil
	})
	defer r.Shutdown(context.Background())
	cfg := taskConfig()
	cfg.MaxIterations = 12
	root, _ := r.Create("Inspect", "model", "alice", cfg)
	root, err := r.Continue(context.Background(), root.ID, root.Actor, "start", "", false)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = r.Pause(root.ID, root.Actor, false); err != nil {
		t.Fatal(err)
	}
	if _, err = r.Steer(root.ID, root.Actor, "paused-correction", "Inspect the new source"); err != nil {
		t.Fatal(err)
	}
	r.advanceDelegation(context.Background(), root.ID)
	root, _ = m.GetTask(root.ID)
	if root.Status != TaskInterrupted || root.SchedulerReady || len(root.Instructions) != 1 {
		t.Fatal("paused work automatically resumed")
	}
}
