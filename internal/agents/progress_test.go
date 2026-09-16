package agents

import (
	"context"
	"errors"
	"io"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/takuphilchan/offgrid-llm/pkg/api"
)

func TestRunPreviewIsDurableBeforeCompletionAndNeverContext(t *testing.T) {
	dir := t.TempDir()
	m := NewManagerWithPersistence(nil, nil, nil, dir)
	r := NewRunner(m, &runTestTools{}, nil)
	streaming := make(chan struct{})
	r.StreamCaller = func(ctx context.Context, task *Task, messages []api.ChatMessage, tools []api.Tool, emit func(string, string) error) (*api.ChatCompletionResponse, error) {
		if err := emit("generating", "Visible partial 日本語"); err != nil {
			return nil, err
		}
		close(streaming)
		<-ctx.Done()
		return nil, ctx.Err()
	}
	task, err := r.Create("task", "model", "alice", DefaultAgentConfig())
	if err != nil {
		t.Fatal(err)
	}
	if _, err = r.Continue(context.Background(), task.ID, "alice", "start", "", true); err != nil {
		t.Fatal(err)
	}
	select {
	case <-streaming:
	case <-time.After(5 * time.Second):
		t.Fatal("no live preview")
	}
	saved, _ := m.GetTask(task.ID)
	if saved.Status != TaskRunning || saved.Progress.Preview != "Visible partial 日本語" || saved.Result != "" || len(saved.Checkpoint.Messages) != 2 {
		t.Fatalf("bad provisional snapshot: %+v", saved)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err = r.Shutdown(ctx); err != nil {
		t.Fatal(err)
	}
	reloaded := NewManagerWithPersistence(nil, nil, nil, dir)
	saved, _ = reloaded.GetTask(task.ID)
	if saved.Status != TaskInterrupted || saved.Progress.Preview != "Visible partial 日本語" || saved.Result != "" || len(saved.Checkpoint.Messages) != 2 {
		t.Fatalf("lost or falsely completed preview: %+v", saved)
	}
}

func TestStreamingFailureFlushesBoundedPreviewWithoutCompleting(t *testing.T) {
	m := NewManagerWithPersistence(nil, nil, nil, t.TempDir())
	r := NewRunner(m, &runTestTools{}, nil)
	r.StreamCaller = func(ctx context.Context, task *Task, messages []api.ChatMessage, tools []api.Tool, emit func(string, string) error) (*api.ChatCompletionResponse, error) {
		if err := emit("generating", "start "); err != nil {
			return nil, err
		}
		if err := emit("generating", strings.Repeat("日", 40000)); err != nil {
			return nil, err
		}
		return nil, io.ErrUnexpectedEOF
	}
	task, _ := r.Create("task", "model", "alice", DefaultAgentConfig())
	_, err := r.Continue(context.Background(), task.ID, "alice", "start", "", false)
	if !errors.Is(err, io.ErrUnexpectedEOF) {
		t.Fatal(err)
	}
	saved, _ := m.GetTask(task.ID)
	if saved.Status != TaskInterrupted || saved.Result != "" || !saved.Progress.Truncated || len(saved.Progress.Preview) > 65536 || !utf8.ValidString(saved.Progress.Preview) || !strings.HasPrefix(saved.Progress.Preview, "start ") {
		t.Fatalf("unsafe partial response: %+v", saved.Progress)
	}
}

func TestProgressCannotOverwriteCancellation(t *testing.T) {
	m := NewManager(nil, nil, nil)
	r := NewRunner(m, &runTestTools{}, nil)
	r.StreamCaller = func(ctx context.Context, task *Task, messages []api.ChatMessage, tools []api.Tool, emit func(string, string) error) (*api.ChatCompletionResponse, error) {
		if err := emit("generating", "partial"); err != nil {
			return nil, err
		}
		if _, err := r.Stop(task.ID, task.Actor, "cancel", ""); err != nil {
			return nil, err
		}
		return nil, emit("generating", "must not be published")
	}
	task, _ := r.Create("task", "model", "alice", DefaultAgentConfig())
	_, _ = r.Continue(context.Background(), task.ID, "alice", "start", "", false)
	saved, _ := m.GetTask(task.ID)
	if saved.Status != TaskCancelled || saved.Result != "" || saved.Progress.Preview != "partial" {
		t.Fatalf("cancellation overwritten: %+v", saved)
	}
}

func TestTruncatedStreamNeverExecutesToolsOrCompletes(t *testing.T) {
	tools := &runTestTools{}
	m := NewManager(nil, nil, nil)
	r := NewRunner(m, tools, nil)
	r.StreamCaller = func(ctx context.Context, task *Task, messages []api.ChatMessage, tools []api.Tool, emit func(string, string) error) (*api.ChatCompletionResponse, error) {
		if err := emit("generating", "Not finished"); err != nil {
			return nil, err
		}
		message := api.ChatMessage{Content: "Not finished", ToolCalls: []api.ToolCall{{ID: "partial", Type: "function", Function: api.FunctionCall{Name: "write_file", Arguments: `{"path":"target"}`}}}}
		return &api.ChatCompletionResponse{Choices: []api.ChatCompletionChoice{{FinishReason: "length", Message: message}}}, nil
	}
	task, _ := r.Create("task", "model", "alice", DefaultAgentConfig())
	result, err := r.Continue(context.Background(), task.ID, "alice", "start", "", false)
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != TaskFailed || result.Result != "" || len(result.Checkpoint.Messages) != 2 || result.PendingApproval != nil || tools.calls.Load() != 0 {
		t.Fatalf("unsafe streamed result: %+v", result)
	}
}
