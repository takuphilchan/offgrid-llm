package agents

import (
	"context"
	"fmt"
	"testing"

	"github.com/takuphilchan/offgrid-llm/internal/inference"
	"github.com/takuphilchan/offgrid-llm/pkg/api"
)

func TestGrammarFailurePersistsWithoutRetryOrPrivateDetails(t *testing.T) {
	for _, uncertain := range []bool{false, true} {
		t.Run(fmt.Sprint(uncertain), func(t *testing.T) {
			dir := t.TempDir()
			manager := NewManagerWithPersistence(nil, nil, nil, dir)
			tools := &runTestTools{}
			attempts := 0
			runner := NewRunner(manager, tools, func(_ context.Context, task *Task, _ []api.ChatMessage, _ []api.Tool) (*api.ChatCompletionResponse, error) {
				attempts++
				if uncertain {
					// Simulate a durable unresolved mutation. The runtime diagnostic
					// must never override its reconciliation requirement.
					if _, err := manager.updateTask(task.ID, func(saved *Task) error {
						saved.Checkpoint.ExecutingCall = "unresolved-mutation"
						return nil
					}); err != nil {
						t.Fatal(err)
					}
				}
				return nil, fmt.Errorf("inference: %w", &inference.EngineError{Code: inference.ErrCodeInvalidToolGrammar, Message: "PRIVATE", Details: "PRIVATE"})
			})
			run, err := runner.Create("Synthetic task", "model", "alice", DefaultAgentConfig())
			if err != nil {
				t.Fatal(err)
			}
			got, err := runner.Continue(context.Background(), run.ID, "alice", "start", "", false)
			if err == nil || attempts != 1 || tools.calls.Load() != 0 {
				t.Fatal("unexpected retry or effects")
			}
			if uncertain {
				if got.Status != TaskUncertain || got.ErrorCode == inference.ErrCodeInvalidToolGrammar || got.Checkpoint.ExecutingCall == "" {
					t.Fatal("lost uncertain effect")
				}
			} else if got.Status != TaskInterrupted || got.ErrorCode != inference.ErrCodeInvalidToolGrammar || got.Error != inference.InvalidToolGrammarMessage {
				t.Fatalf("lost safe diagnostic: %+v", got)
			}
			if err := runner.Shutdown(context.Background()); err != nil {
				t.Fatal(err)
			}
			saved, ok := NewManagerWithPersistence(nil, nil, nil, dir).GetTask(run.ID)
			if !ok || saved.Status != got.Status || saved.Error != got.Error || saved.ErrorCode != got.ErrorCode {
				t.Fatal("diagnostic not durable")
			}
		})
	}
}
