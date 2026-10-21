package agents

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/takuphilchan/offgrid-llm/internal/capabilities"
	"github.com/takuphilchan/offgrid-llm/pkg/api"
)

func TestArtifactRepairCannotBeErasedByReadsOrFalseCompletion(t *testing.T) {
	for _, next := range []string{"repeat", "answer", "read"} {
		t.Run(next, func(t *testing.T) {
			dir := t.TempDir()
			manager := NewManagerWithPersistence(nil, nil, nil, dir)
			tools := &observationTestTools{descriptor: capabilities.Descriptor{Name: "task_save_artifact", Kind: capabilities.Write, Namespace: "workspace", Source: "workspace", Risk: capabilities.RiskLow}}
			tools.authorize = func(name string, _ json.RawMessage, _ ToolExecution) error {
				if name == "task_save_artifact" {
					return &ToolAuthorizationError{Code: "invalid_task_artifact", Message: "artifact name requires .txt"}
				}
				return nil
			}
			tools.execute = func(string, json.RawMessage, ToolExecution) (string, error) {
				return `{"observation":{"id":"fresh"}}`, nil
			}
			calls := 0
			r := NewRunner(manager, tools, func(_ context.Context, task *Task, _ []api.ChatMessage, _ []api.Tool) (*api.ChatCompletionResponse, error) {
				calls++
				if calls > 1 && (!task.Checkpoint.PreEffectRepair || task.Checkpoint.ExecutingCall != "" || len(task.Checkpoint.Calls) != 0) {
					t.Fatal("lost repair or retained executable rejection")
				}
				if calls > 1 && next == "answer" {
					return runAnswer(api.ChatMessage{Content: "Saved successfully."}), nil
				}
				if next == "read" && calls%2 == 0 {
					return observationCall("computer_observe"), nil
				}
				return observationCall("task_save_artifact"), nil
			})
			task, err := r.Create("Save a draft", "model", "alice", observationConfig())
			if err != nil {
				t.Fatal(err)
			}
			got, err := r.Continue(context.Background(), task.ID, task.Actor, "start", "", false)
			wantCalls, wantEffects := 4, int32(0)
			if next == "read" {
				wantCalls, wantEffects = 7, 3
			}
			if err != nil || got.Status != TaskInterrupted || got.ErrorCode != proposalRepairLimitCode || calls != wantCalls || tools.calls.Load() != wantEffects {
				t.Fatalf("unbounded/erased repair: status=%s code=%s calls=%d effects=%d err=%v", got.Status, got.ErrorCode, calls, tools.calls.Load(), err)
			}
			if err := r.Shutdown(context.Background()); err != nil {
				t.Fatal(err)
			}
			saved, ok := NewManagerWithPersistence(nil, nil, nil, dir).GetTask(task.ID)
			if !ok || !saved.Checkpoint.PreEffectRepair || saved.Checkpoint.PreEffectRepairTool != "task_save_artifact" || len(saved.Checkpoint.Calls) != 0 || saved.Status != TaskInterrupted {
				t.Fatal("restart lost repair")
			}
		})
	}
}

func TestArtifactRepairRequiresOwnedStorageAndNeverRetriesDispatchedFailure(t *testing.T) {
	for _, source := range []string{"workspace", "builtin", "mcp:workspace"} {
		cp := &Checkpoint{PreEffectRepair: true, PreEffectRepairTool: "task_save_artifact", ProposalTool: "task_save_artifact", ProposalUnresolved: true, PreEffectRepairs: 2}
		resolvePreEffectRepair(cp, "task_save_artifact", capabilities.Descriptor{Name: "task_save_artifact", Kind: capabilities.Write, Namespace: "workspace", Source: source})
		if cp.PreEffectRepair == (source == "workspace") {
			t.Fatal("incorrect repair identity", source)
		}
	}
	tools := &observationTestTools{descriptor: capabilities.Descriptor{Name: "task_save_artifact", Kind: capabilities.Write, Namespace: "workspace", Source: "workspace", Risk: capabilities.RiskLow}}
	tools.authorize = func(string, json.RawMessage, ToolExecution) error { return nil }
	// A storage failure after dispatch is not a rejected proposal, even if its
	// text resembles validation feedback.
	tools.execute = func(string, json.RawMessage, ToolExecution) (string, error) {
		return "", errors.New("invalid_task_artifact: storage write outcome unknown")
	}
	calls := 0
	r := NewRunner(NewManagerWithPersistence(nil, nil, nil, t.TempDir()), tools, func(context.Context, *Task, []api.ChatMessage, []api.Tool) (*api.ChatCompletionResponse, error) {
		calls++
		return observationCall("task_save_artifact"), nil
	})
	task, err := r.Create("Save a draft", "model", "alice", observationConfig())
	if err != nil {
		t.Fatal(err)
	}
	got, err := r.Continue(context.Background(), task.ID, task.Actor, "start", "", false)
	if err == nil || got.Status != TaskUncertain || calls != 1 || tools.calls.Load() != 1 {
		t.Fatalf("storage failure replayed: %+v calls=%d err=%v", got, calls, err)
	}
}

func TestArtifactPendingInspectionHonorsExplicitUserCorrection(t *testing.T) {
	tools := &observationTestTools{}
	resolved := time.Now().UTC()
	task := &Task{Config: AgentConfig{ComputerStepOffset: 1}, LastAccess: &InputRequest{Kind: "computer", CallID: "access"}, Steps: []Step{{Type: "input_resolved", Timestamp: resolved}}}
	if !ComputerAccessNeedsObservation(task, tools) {
		t.Fatal("missing pending inspection")
	}
	task.Instructions = []Instruction{{At: resolved.Add(time.Second), Text: "Do not use the application; just answer my question."}}
	if ComputerAccessNeedsObservation(task, tools) {
		t.Fatal("superseded operation forced desktop use")
	}
	task.Steps = append(task.Steps, Step{Type: "input_resolved", Timestamp: resolved.Add(2 * time.Second)})
	task.Config.ComputerStepOffset = 2
	if !ComputerAccessNeedsObservation(task, tools) {
		t.Fatal("new access request inherited stale correction")
	}
}
