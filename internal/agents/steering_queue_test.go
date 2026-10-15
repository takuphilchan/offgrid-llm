package agents

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/takuphilchan/offgrid-llm/pkg/api"
)

func TestRunningSteeringSupersedesUndispatchedProposal(t *testing.T) {
	for _, boundary := range []string{"model", "authorization", "completion"} {
		t.Run(boundary, func(t *testing.T) {
			m := NewManagerWithPersistence(nil, nil, nil, t.TempDir())
			entered, release := make(chan struct{}), make(chan struct{})
			var once sync.Once
			tools := &runTestTools{authorize: func(string, json.RawMessage, ToolExecution) error {
				if boundary == "authorization" {
					once.Do(func() { close(entered); <-release })
				}
				return nil
			}}
			var calls atomic.Int32
			r := NewRunner(m, tools, func(_ context.Context, _ *Task, messages []api.ChatMessage, _ []api.Tool) (*api.ChatCompletionResponse, error) {
				if calls.Add(1) == 1 {
					if boundary != "authorization" {
						close(entered)
						<-release
					}
					if boundary == "completion" {
						return runAnswer(api.ChatMessage{Role: "assistant", Content: "Outdated answer"}), nil
					}
					return taskCall("write_file", `{"path":"old.txt"}`), nil
				}
				if messages[len(messages)-1].StringContent() != "Do not write; answer only." {
					t.Error("correction missing", messages)
				}
				return runAnswer(api.ChatMessage{Role: "assistant", Content: "Revised answer"}), nil
			})
			defer r.Shutdown(context.Background())
			task, _ := r.Create("Write a note", "model", "alice", taskConfig())
			if _, err := r.Continue(context.Background(), task.ID, "alice", "start", "", true); err != nil {
				t.Fatal(err)
			}
			<-entered
			for i := 0; i < 2; i++ {
				queued, err := r.Steer(task.ID, "alice", "correction-1", "Do not write; answer only.")
				if err != nil || len(queued.PendingInstructions) != 1 || len(queued.Instructions) != 0 {
					t.Fatal("not queued exactly once", queued, err)
				}
			}
			if _, err := r.Steer(task.ID, "bob", "correction-2", "Forged instruction"); !errors.Is(err, ErrTaskNotFound) {
				t.Fatal(err)
			}
			if _, err := r.Steer(task.ID, "alice", "correction-1", "Conflicting instruction"); !errors.Is(err, ErrRunConflict) {
				t.Fatal(err)
			}
			close(release)
			done := awaitTask(t, m, task.ID, TaskCompleted)
			settleTask(t, r, task.ID)
			if tools.calls.Load() != 0 || done.Result != "Revised answer" || len(done.PendingInstructions) != 0 || len(done.Instructions) != 1 {
				t.Fatal("stale proposal executed", done, tools.calls.Load())
			}
			if done.Checkpoint.Iteration != int(calls.Load()) {
				t.Fatalf("steering refunded inference: iterations=%d calls=%d", done.Checkpoint.Iteration, calls.Load())
			}
		})
	}
}

func TestSteeringCannotRefundTheLastIteration(t *testing.T) {
	m := NewManagerWithPersistence(nil, nil, nil, t.TempDir())
	entered, release := make(chan struct{}), make(chan struct{})
	var calls atomic.Int32
	r := NewRunner(m, &runTestTools{}, func(context.Context, *Task, []api.ChatMessage, []api.Tool) (*api.ChatCompletionResponse, error) {
		if calls.Add(1) == 1 {
			close(entered)
			<-release
		}
		return runAnswer(api.ChatMessage{Role: "assistant", Content: "Superseded reply"}), nil
	})
	defer r.Shutdown(context.Background())
	config := taskConfig()
	config.MaxIterations = 1
	task, err := r.Create("Answer", "model", "alice", config)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = r.Continue(context.Background(), task.ID, "alice", "start", "", true); err != nil {
		t.Fatal(err)
	}
	<-entered
	if _, err = r.Steer(task.ID, "alice", "correction-final", "Use the corrected context."); err != nil {
		t.Fatal(err)
	}
	close(release)
	done := awaitTask(t, m, task.ID, TaskFailed)
	settleTask(t, r, task.ID)
	if calls.Load() != 1 || done.Checkpoint.Iteration != 1 || !strings.Contains(done.Error, "Maximum agent iterations") {
		t.Fatal("iteration limit bypassed", calls.Load(), done)
	}
}

func TestRunningSteeringWaitsForDispatchedResultAndNeverReplays(t *testing.T) {
	for _, uncertain := range []bool{false, true} {
		t.Run(map[bool]string{false: "recorded", true: "uncertain"}[uncertain], func(t *testing.T) {
			m := NewManagerWithPersistence(nil, nil, nil, t.TempDir())
			entered, release := make(chan struct{}), make(chan struct{})
			tools := &runTestTools{authorize: func(string, json.RawMessage, ToolExecution) error { return nil }, execute: func(string, json.RawMessage, ToolExecution) (string, error) {
				close(entered)
				<-release
				if uncertain {
					return "", errors.New("lost mutation acknowledgment")
				}
				return "original action recorded", nil
			}}
			var calls atomic.Int32
			r := NewRunner(m, tools, func(_ context.Context, _ *Task, messages []api.ChatMessage, _ []api.Tool) (*api.ChatCompletionResponse, error) {
				if calls.Add(1) == 1 {
					return taskCall("write_file", `{"path":"note.txt"}`), nil
				}
				if messages[len(messages)-1].StringContent() != "Summarize what already happened." {
					t.Error("correction missing")
				}
				found := false
				for _, m := range messages {
					if m.Role == "tool" && strings.Contains(m.StringContent(), "original action recorded") {
						found = true
					}
				}
				if !found {
					t.Error("dispatched result lost")
				}
				return runAnswer(api.ChatMessage{Role: "assistant", Content: "The earlier action completed."}), nil
			})
			defer r.Shutdown(context.Background())
			task, _ := r.Create("Write", "model", "alice", taskConfig())
			if _, err := r.Continue(context.Background(), task.ID, "alice", "start", "", true); err != nil {
				t.Fatal(err)
			}
			<-entered
			queued, err := r.Steer(task.ID, "alice", "correction-1", "Summarize what already happened.")
			if err != nil || queued.Checkpoint.ExecutingCall == "" || len(queued.PendingInstructions) != 1 {
				t.Fatal("in-flight action wrongly canceled", queued, err)
			}
			close(release)
			status := TaskCompleted
			if uncertain {
				status = TaskUncertain
			}
			saved := awaitTask(t, m, task.ID, status)
			settleTask(t, r, task.ID)
			if tools.calls.Load() != 1 {
				t.Fatal("mutation replayed")
			}
			if uncertain && (len(saved.PendingInstructions) != 1 || len(saved.Instructions) != 0 || calls.Load() != 1) {
				t.Fatal("uncertain effects bypassed", saved)
			}
		})
	}
}

func TestQueuedSteeringSurvivesRestartWithoutImplicitResume(t *testing.T) {
	dir := t.TempDir()
	m := NewManagerWithPersistence(nil, nil, nil, dir)
	r := NewRunner(m, &runTestTools{}, nil)
	task, _ := r.Create("Original", "model", "alice", taskConfig())
	if _, err := m.updateTask(task.ID, func(t *Task) error { t.Status = TaskRunning; return nil }); err != nil {
		t.Fatal(err)
	}
	if _, err := r.Steer(task.ID, "alice", "correction-1", "The replacement instruction"); err != nil {
		t.Fatal(err)
	}
	restored := NewManagerWithPersistence(nil, nil, nil, dir)
	saved, _ := restored.GetTask(task.ID)
	if saved.Status != TaskInterrupted || len(saved.PendingInstructions) != 1 || len(saved.Instructions) != 0 {
		t.Fatal("restart applied or lost correction", saved)
	}
	next := NewRunner(restored, &runTestTools{}, func(_ context.Context, _ *Task, messages []api.ChatMessage, _ []api.Tool) (*api.ChatCompletionResponse, error) {
		if messages[len(messages)-1].StringContent() != "The replacement instruction" {
			t.Error("restart lost correction")
		}
		return runAnswer(api.ChatMessage{Role: "assistant", Content: "Resumed explicitly"}), nil
	})
	done, err := next.Continue(context.Background(), task.ID, "alice", "resume", "", false)
	if err != nil || done.Status != TaskCompleted || len(done.Instructions) != 1 || len(done.PendingInstructions) != 0 {
		t.Fatal(done, err)
	}
}
