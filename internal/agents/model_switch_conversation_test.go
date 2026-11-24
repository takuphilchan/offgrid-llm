package agents

import (
	"context"
	"testing"

	"github.com/takuphilchan/offgrid-llm/pkg/api"
)

func TestConversationAcceptedModelSurvivesQualifiedSwitch(t *testing.T) {
	for _, initial := range []string{"primary", "fallback-one"} {
		t.Run(initial, func(t *testing.T) {
			m, r, scope, conversation := controlFixture(t)
			config := switchFixtureConfig()
			if initial == "fallback-one" {
				config.ModelSelection.Selected = initial
				config.ContextWindow, config.ToolAdapter = config.ModelSelection.Candidates[1].ContextWindow, config.ModelSelection.Candidates[1].ToolAdapter
			}
			config.ConversationConfiguration = &ConversationConfiguration{Version: 1, Model: initial}
			receipt, err := r.AdmitConversationWork(scope, ConversationCommand{RequestID: "conversation-switch", TargetID: conversation.ID, ExpectedRevision: conversation.Revision, Kind: "create_work", Payload: ConversationCommandPayload{Input: &ConversationInput{Content: "Answer from the supplied facts"}}}, initial, config)
			if err != nil {
				t.Fatal(err)
			}
			calls := 0
			r.caller = func(_ context.Context, task *Task, _ []api.ChatMessage, _ []api.Tool) (*api.ChatCompletionResponse, error) {
				calls++
				if calls == 1 {
					return nil, rejectedDecision()
				}
				if task.Config.ConversationConfiguration.Model != initial || task.Model == initial {
					t.Fatal("accepted choice overwritten or switch ignored")
				}
				return runAnswer(api.ChatMessage{Role: "assistant", Content: "Answered from the supplied facts"}), nil
			}
			finished, err := r.Continue(context.Background(), receipt.Work.RunID, scope.Actor, "start", "", false)
			if err != nil || finished.Status != TaskCompleted || calls != 2 {
				t.Fatalf("accepted conversation switch failed: status=%v calls=%d err=%v", finished, calls, err)
			}
			reloaded := NewManagerWithPersistence(nil, nil, nil, m.dataDir)
			if err = reloaded.StorageError(); err != nil {
				t.Fatal(err)
			}
			saved, _ := reloaded.GetTask(finished.ID)
			if saved.Model != finished.Model || saved.Config.ConversationConfiguration.Model != initial || len(saved.Checkpoint.ModelSwitches) != 1 {
				t.Fatal("restart lost accepted versus effective model")
			}
			// A different original choice, an absent switch receipt or a guided
			// policy cannot authorize a stored model mismatch.
			for _, mutate := range []func(*Task){
				func(task *Task) { task.Config.ConversationConfiguration.Model = "unrelated" },
				func(task *Task) { task.Checkpoint.ModelSwitches = nil },
				func(task *Task) { task.Config.ModelSelection.Mode = "guided" },
			} {
				bad, err := copyTask(saved)
				if err != nil {
					t.Fatal(err)
				}
				mutate(bad)
				if validateStoredTask(bad) == nil {
					t.Fatal("unproven conversation model mismatch accepted")
				}
			}
		})
	}
}
