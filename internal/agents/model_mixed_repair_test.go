package agents

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/takuphilchan/offgrid-llm/internal/capabilities"
	"github.com/takuphilchan/offgrid-llm/pkg/api"
)

func TestModelGraphMixedRejectionsCannotRefundCorrections(t *testing.T) {
	m := NewManagerWithPersistence(nil, nil, nil, t.TempDir())
	tools := invalidAccessTools()
	calls := 0
	r := NewRunner(m, tools, func(context.Context, *Task, []api.ChatMessage, []api.Tool) (*api.ChatCompletionResponse, error) {
		calls++
		switch calls {
		case 1, 2:
			return nil, rejectedDecision()
		case 3:
			return accessProposalAnswer(), nil
		default:
			return nil, context.Canceled
		}
	})
	config := switchFixtureConfig()
	config.MaxIterations = 30
	task, err := r.Create("Use only approved access", "primary", "alice", config)
	if err != nil {
		t.Fatal(err)
	}
	_, _ = r.Continue(context.Background(), task.ID, "alice", "start", "", false)
	saved, _ := m.GetTask(task.ID)
	if calls != 3 || tools.calls.Load() != 0 || saved.Status != TaskInterrupted || saved.Checkpoint.ModelBudget.Repairs[task.ID] != 3 {
		t.Fatalf("mixed rejections refunded debt: calls=%d effects=%d status=%s error=%s budget=%+v", calls, tools.calls.Load(), saved.Status, saved.Error, saved.Checkpoint.ModelBudget)
	}
	if len(saved.Checkpoint.Calls) != 0 || saved.Checkpoint.ExecutingCall != "" {
		t.Fatal("rejected proposal remained executable")
	}
	restarted := NewManagerWithPersistence(nil, nil, nil, m.dataDir)
	if err := restarted.StorageError(); err != nil {
		t.Fatal(err)
	}
	saved, _ = restarted.GetTask(task.ID)
	if saved.Checkpoint.ModelBudget.Repairs[task.ID] != 3 {
		t.Fatal("restart lost mixed rejection charges")
	}
}

func TestModelGraphOnlyAcceptedResultsResolveDecisionDebt(t *testing.T) {
	for _, mode := range []string{"tool", "metadata", "delegation", "approval", "input", "failed_tool", "invalid_metadata"} {
		t.Run(mode, func(t *testing.T) {
			m := NewManagerWithPersistence(nil, nil, nil, t.TempDir())
			tools := &runTestTools{
				authorize: func(string, json.RawMessage, ToolExecution) error {
					switch mode {
					case "approval":
						return capabilities.ErrApprovalRequired
					case "input":
						return &InputRequired{Request: InputRequest{Kind: "computer", Mode: "app", Target: "Fixture"}}
					}
					return nil
				},
				execute: func(string, json.RawMessage, ToolExecution) (string, error) {
					if mode == "failed_tool" {
						return "", errors.New("fixture outcome unknown")
					}
					return "saved fixture result", nil
				},
			}
			calls := 0
			r := NewRunner(m, tools, func(_ context.Context, task *Task, _ []api.ChatMessage, _ []api.Tool) (*api.ChatCompletionResponse, error) {
				calls++
				if calls == 1 {
					return nil, rejectedDecision()
				}
				if calls == 2 {
					switch mode {
					case "metadata":
						return taskCall("task_plan", `{"items":[{"id":"one","title":"Review supplied facts","state":"pending","evidence_steps":[]}]}`), nil
					case "invalid_metadata":
						return taskCall("task_plan", `{"items":[]}`), nil
					case "delegation":
						return taskCall("delegate_tasks", `{"tasks":[{"key":"one","goal":"Review supplied facts","depends_on":[],"tools":[]}]}`), nil
					default:
						return accessProposalAnswer(), nil
					}
				}
				saved, _ := m.GetTask(task.ID)
				want := 0
				if mode == "invalid_metadata" {
					want = 1
				}
				wantTotal := want
				if mode == "invalid_metadata" {
					wantTotal++
				} // The rejected metadata is also unresolved.
				if saved.Checkpoint.DecisionRepairs != want || saved.Checkpoint.ModelBudget.Repairs[task.ID] != wantTotal {
					t.Fatal("incorrect debt before next inference", saved.Checkpoint.ModelBudget)
				}
				if mode == "invalid_metadata" {
					return nil, context.Canceled
				}
				return runAnswer(api.ChatMessage{Role: "assistant", Content: "Reviewed the supplied facts"}), nil
			})
			config := switchFixtureConfig()
			config.MaxIterations, config.DisableDelegation = 30, false
			task, err := r.Create("Review supplied facts", "primary", "alice", config)
			if err != nil {
				t.Fatal(err)
			}
			_, _ = r.Continue(context.Background(), task.ID, "alice", "start", "", false)
			saved, _ := m.GetTask(task.ID)
			want := 1
			if mode == "tool" || mode == "metadata" || mode == "delegation" {
				want = 0
			}
			wantTotal := want
			if mode == "invalid_metadata" {
				wantTotal++
			}
			if saved.Checkpoint.DecisionRepairs != want || saved.Checkpoint.ModelBudget.Repairs[task.ID] != wantTotal || saved.Checkpoint.ModelBudget.Switches != 1 {
				t.Fatalf("acceptance changed wrong charge: mode=%s status=%s debt=%d budget=%+v", mode, saved.Status, saved.Checkpoint.DecisionRepairs, saved.Checkpoint.ModelBudget)
			}
			if mode == "approval" && saved.Status != TaskWaiting || mode == "input" && saved.Status != TaskInput || mode == "delegation" && saved.Status != TaskChildren {
				t.Fatal("fixture did not reach expected boundary", mode, saved.Status)
			}
			if (mode == "tool" || mode == "metadata") && (saved.Status != TaskCompleted || calls != 3) {
				t.Fatal("accepted progress did not complete", mode, saved.Status, calls)
			}
		})
	}
}
