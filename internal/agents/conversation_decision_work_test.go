package agents

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/takuphilchan/offgrid-llm/pkg/api"
)

func finishNewWorkDecision(t *testing.T, m *Manager, scope ControlScope, c *Conversation, input *api.ConversationInboxEntry) *Runner {
	t.Helper()
	r := NewRunner(m, NewToolRegistry(), func(_ context.Context, task *Task, _ []api.ChatMessage, _ []api.Tool) (*api.ChatCompletionResponse, error) {
		if len(task.Steps) == 0 {
			return taskCall("conversation_decide", `{"kind":"new_work","instruction_version":0,"expected_result":"A sourced comparison with explicit limitations"}`), nil
		}
		return runAnswer(api.ChatMessage{Role: "assistant", Content: "The new work request is recorded."}), nil
	})
	finishTestConversationTurn(t, r, scope, c, input)
	return r
}

func TestConversationDecisionNewWorkAtomicRetryAndRestart(t *testing.T) {
	m, r, scope, c := controlFixture(t)
	old := admitControlTestWork(t, r, scope, c)
	c, _ = m.Conversation(scope, c.ID)
	input := acceptTestConversationInput(t, r, scope, c, "new-objective")
	other := acceptTestConversationInput(t, r, scope, c, "another-input")
	r = finishNewWorkDecision(t, m, scope, c, input)
	c, _ = m.Conversation(scope, c.ID)
	var group sync.WaitGroup
	identities := make(chan string, 8)
	for i := 0; i < 8; i++ {
		group.Add(1)
		go func() {
			defer group.Done()
			entry, err := r.AdmitConversationInputWork(scope, c.ID, input.ID, c.Revision, "model", taskConfig())
			if err != nil {
				t.Error(err)
				return
			}
			identities <- entry.CreatedWorkID
		}()
	}
	group.Wait()
	close(identities)
	createdID := ""
	for id := range identities {
		if id == "" || createdID != "" && createdID != id {
			t.Fatal("retry created different work", createdID, id)
		}
		createdID = id
	}
	created, err := m.WorkItem(scope, createdID)
	if err != nil || created.Objective != input.Input.Content || len(created.Acceptance) != 1 || created.Acceptance[0] != "A sourced comparison with explicit limitations" {
		t.Fatal(created, err)
	}
	task, _ := m.GetTask(created.RunID)
	if task.ConversationTurn != nil || task.Config.Profile != nil || task.Config.Execution != nil || task.Config.ComputerSession != "" || task.Origin.TurnID != input.ID || task.Checkpoint.BudgetedModelCalls != 0 || !task.SchedulerReady || task.AdmissionHeld {
		t.Fatal("new work inherited foreground/grants or dispatched prematurely", task)
	}
	previous, _ := m.GetTask(old.RunID)
	if previous.InstructionVersion != 1 || previous.Checkpoint.BudgetedModelCalls != 0 || !hasConversationHold(previous) || len(previous.ConversationHold.InputIDs) != 1 || previous.ConversationHold.InputIDs[0] != other.ID {
		t.Fatal("new work reset old allowance or released another input", previous)
	}
	db, err := m.controlDatabase()
	if err != nil {
		t.Fatal(err)
	}
	var count int
	err = db.QueryRow(`SELECT count(*) FROM agent_conversation_messages WHERE workspace=? AND actor=? AND conversation_id=? AND json_extract(payload,'$.role')='user'`, scope.Workspace, scope.Actor, c.ID).Scan(&count)
	db.Close()
	if err != nil || count != 3 { // original explicit work plus the two inputs
		t.Fatal("work admission duplicated user history", count, err)
	}
	runs, err := m.ConversationRuns(scope, c.ID)
	if err != nil || len(runs) != 2 {
		t.Fatal("duplicate/unlinked run", runs, err)
	}
	m = NewManagerWithPersistence(nil, nil, nil, m.dataDir)
	if err = m.StorageError(); err != nil {
		t.Fatal(err)
	}
	r = NewRunner(m, NewToolRegistry(), nil)
	retry, err := r.AdmitConversationInputWork(scope, c.ID, input.ID, 0, "", AgentConfig{})
	if err != nil || retry.CreatedWorkID != createdID {
		t.Fatal("retry depended on new preparation", retry, err)
	}
	recovered, _ := m.GetTask(created.RunID)
	if recovered.Status != TaskInterrupted || recovered.SchedulerReady {
		t.Fatal("observation/retry restarted work", recovered)
	}
}

func TestConversationDecisionNewWorkRejectsUnsafePreparation(t *testing.T) {
	m, r, scope, c := controlFixture(t)
	input := acceptTestConversationInput(t, r, scope, c, "new-objective")
	r = finishNewWorkDecision(t, m, scope, c, input)
	c, _ = m.Conversation(scope, c.ID)
	if _, err := r.AdmitConversationInputWork(ControlScope{scope.Workspace, "foreign"}, c.ID, input.ID, c.Revision, "model", taskConfig()); !errors.Is(err, ErrControlNotFound) {
		t.Fatal("foreign work admission", err)
	}
	if _, err := r.AdmitConversationInputWork(scope, c.ID, input.ID, c.Revision-1, "model", taskConfig()); !errors.Is(err, ErrRunConflict) {
		t.Fatal("stale config admitted", err)
	}
	config := taskConfig()
	config.ComputerSession = "must-not-transfer"
	if _, err := r.AdmitConversationInputWork(scope, c.ID, input.ID, c.Revision, "model", config); !errors.Is(err, ErrControlCommand) {
		t.Fatal("host grant transferred", err)
	}
	r.SourceAccess = func(*Task) bool { return false }
	if _, err := r.AdmitConversationInputWork(scope, c.ID, input.ID, c.Revision, "model", taskConfig()); !errors.Is(err, ErrSourceAccess) {
		t.Fatal("revoked context admitted", err)
	}
	entry, _ := m.ConversationInput(scope, c.ID, input.ID)
	if entry.AppliedAt != nil || entry.CreatedWorkID != "" {
		t.Fatal("rejected work published", entry)
	}
}

func TestConversationDecisionNewWorkRetainsRequiredHostInput(t *testing.T) {
	m, r, scope, c := controlFixture(t)
	input := acceptTestConversationInput(t, r, scope, c, "new-objective")
	r = finishNewWorkDecision(t, m, scope, c, input)
	c, _ = m.Conversation(scope, c.ID)
	config := taskConfig()
	config.RequestedComputerScope = "desktop"
	config.RequestedComputerApprovalMode = "action"
	entry, err := r.AdmitConversationInputWork(scope, c.ID, input.ID, c.Revision, "model", config)
	if err != nil {
		t.Fatal(err)
	}
	work, _ := m.WorkItem(scope, entry.CreatedWorkID)
	task, _ := m.GetTask(work.RunID)
	if work.Status != TaskInput || task.Status != TaskInput || task.PendingInput == nil || task.PendingInput.Kind != "computer" || task.SchedulerReady || task.Config.ComputerSession != "" {
		t.Fatal("required host access was dropped or converted to a grant", task, work)
	}
	if _, err = r.Continue(context.Background(), task.ID, scope.Actor, "start", "", false); !errors.Is(err, ErrRunConflict) {
		t.Fatal("work ran before host access", err)
	}
}
