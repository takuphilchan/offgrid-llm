package agents

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/takuphilchan/offgrid-llm/pkg/api"
)

func TestRejectedLaunchCatalogLoopCannotResetRepairBudget(t *testing.T) {
	calls := 0
	tools := &runTestTools{authorize: func(name string, _ json.RawMessage, _ ToolExecution) error {
		if name != "computer_applications" {
			t.Fatal("rejected launch reached dispatch", name)
		}
		return nil
	}, execute: func(string, json.RawMessage, ToolExecution) (string, error) { return `{"applications":[]}`, nil }}
	r := NewRunner(NewManagerWithPersistence(nil, nil, nil, t.TempDir()), tools, func(context.Context, *Task, []api.ChatMessage, []api.Tool) (*api.ChatCompletionResponse, error) {
		calls++
		if calls%2 == 1 {
			return computerBatchAnswer("computer_open_application", "computer_documents"), nil
		}
		return computerBatchAnswer("computer_applications"), nil
	})
	task := newComputerBatchTask(t, r)
	got, err := r.Continue(context.Background(), task.ID, "alice", "start", "", false)
	if err != nil || got.Status != TaskInterrupted || got.ErrorCode != proposalRepairLimitCode || calls != 7 || tools.calls.Load() != 3 {
		t.Fatalf("catalog reset rejected launch repair: status=%s calls=%d executed=%d err=%v", got.Status, calls, tools.calls.Load(), err)
	}
	if got.Checkpoint.SequenceRepairTool != "computer_open_application" || !got.Checkpoint.ProposalUnresolved {
		t.Fatal("lost repair identity")
	}
}

func TestSequenceRepairKeepsCompletedDependencyAcrossReadRepairAndRestart(t *testing.T) {
	cp := &Checkpoint{ProposalTool: computerToolSequenceRepair, SequenceRepairTool: "computer_open_application", ProposalRepairs: 2, ProposalUnresolved: true, NativeReadRepair: &NativeReadRepair{}}
	resolveComputerToolSequence(cp, "computer_applications")
	if cp.SequenceRepairCompleted {
		t.Fatal("unrelated catalog claimed dependency completion")
	}
	resolveComputerToolSequence(cp, "computer_open_application")
	if !cp.SequenceRepairCompleted || !cp.ProposalUnresolved || cp.ProposalRepairs != 2 {
		t.Fatal("strong read guard was cleared or valid launch progress lost")
	}
	raw, _ := json.Marshal(cp)
	var restored Checkpoint
	if json.Unmarshal(raw, &restored) != nil {
		t.Fatal("checkpoint decode failed")
	}
	restored.NativeReadRepair = nil // independently verified readiness
	resolveComputerToolSequence(&restored, "computer_verify_application")
	if restored.ProposalUnresolved || restored.ProposalRepairs != 0 || restored.SequenceRepairCompleted || restored.SequenceRepairTool != "" {
		t.Fatal("valid readiness required replaying completed launch")
	}
}
