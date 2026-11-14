package agents

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/takuphilchan/offgrid-llm/pkg/api"
)

func inboxCommand(c *Conversation, id, content string) api.ConversationInputCommand {
	return api.ConversationInputCommand{RequestID: id, TargetID: c.ID, ExpectedRevision: c.Revision, Kind: "submit_input", Payload: api.ConversationInboxInput{Content: content, Origin: "typed"}}
}

func TestConversationInboxAcceptsDuringInferenceAndFencesLateProposal(t *testing.T) {
	m, r, scope, c := controlFixture(t)
	entered, release := make(chan struct{}), make(chan struct{})
	r.caller = func(context.Context, *Task, []api.ChatMessage, []api.Tool) (*api.ChatCompletionResponse, error) {
		close(entered)
		<-release
		response := runAnswer(api.ChatMessage{Role: "assistant", Content: "Outdated final answer"})
		response.Usage = api.Usage{PromptTokens: 100, CompletionTokens: 12, TotalTokens: 112}
		return response, nil
	}
	work := admitControlTestWork(t, r, scope, c)
	done := make(chan *Task, 1)
	failure := make(chan error, 1)
	go func() {
		task, err := r.Continue(context.Background(), work.RunID, scope.Actor, "start", "", false)
		done <- task
		failure <- err
	}()
	select {
	case <-entered:
	case <-time.After(5 * time.Second):
		t.Fatal("model did not start")
	}
	current, _ := m.Conversation(scope, c.ID)
	command := inboxCommand(current, "input", "Use the corrected source")
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			receipt, err := r.AcceptConversationInput(scope, command)
			if err != nil || receipt.Input == nil || receipt.Input.State != "accepted" {
				t.Error(receipt, err)
			}
		}()
	}
	wg.Wait()
	held, _ := m.GetTask(work.RunID)
	if !hasConversationHold(held) || len(held.ConversationHold.InputIDs) != 1 || held.ControlEpoch != 1 || held.Checkpoint.BudgetedModelCalls != 1 {
		t.Fatal("input duplicated or reset budget", held)
	}
	close(release)
	select {
	case stopped := <-done:
		if stopped == nil || stopped.Status != TaskConversationInput || stopped.Result != "" || stopped.Checkpoint.ChargedCompletionTokens != 12 {
			t.Fatal("late answer bypassed inbox or lost usage", stopped)
		}
		if stopped.Checkpoint.InputBudget == nil || stopped.Checkpoint.InputBudget.ChargedTokens != 100 || stopped.Checkpoint.InputBudget.MeasuredTokens != 100 || stopped.Checkpoint.ContextPreparations != 1 {
			t.Fatal("conversation hold lost input or preparation settlement", stopped.Checkpoint)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("run did not yield")
	}
	if err := <-failure; err != nil {
		t.Fatal(err)
	}
	r.mu.Lock()
	active := len(r.active)
	r.mu.Unlock()
	if active != 0 {
		t.Fatal("waiting work held runner slot", active)
	}
	if _, err := r.Continue(context.Background(), work.RunID, scope.Actor, "resume", "", false); !errors.Is(err, ErrRunConflict) {
		t.Fatal("manual resume bypassed unincorporated input", err)
	}
	page, err := m.ConversationInbox(scope, c.ID, 0, 10)
	if err != nil || len(page.Items) != 1 || page.Items[0].WorkID != work.ID {
		t.Fatal(page, err)
	}
	loaded := NewManagerWithPersistence(nil, nil, nil, m.dataDir)
	if err = loaded.StorageError(); err != nil {
		t.Fatal(err)
	}
	restored, _ := loaded.GetTask(work.RunID)
	if restored.Status != TaskInterrupted || !hasConversationHold(restored) || !restored.ConversationHold.RecoveryRequired {
		t.Fatal("restart lost hold or authorized resume", restored)
	}
}

func TestConversationInboxRollbackScopeFocusAndPaging(t *testing.T) {
	m, r, scope, c := controlFixture(t)
	work := admitControlTestWork(t, r, scope, c)
	c, _ = m.Conversation(scope, c.ID)
	db, err := m.controlDatabase()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err = db.Exec(`CREATE TRIGGER reject_inbox BEFORE INSERT ON agent_control_commands WHEN NEW.request_id='fault' BEGIN SELECT RAISE(ABORT,'injected'); END`); err != nil {
		t.Fatal(err)
	}
	command := inboxCommand(c, "fault", "Do not lose this input")
	if _, err = r.AcceptConversationInput(scope, command); err == nil {
		t.Fatal("ignored receipt failure")
	}
	unchanged, _ := m.GetTask(work.RunID)
	if hasConversationHold(unchanged) || unchanged.ControlEpoch != 0 {
		t.Fatal("uncommitted hold published")
	}
	page, err := m.ConversationInbox(scope, c.ID, 0, 10)
	if err != nil || len(page.Items) != 0 {
		t.Fatal("uncommitted inbox published", page, err)
	}
	command.RequestID = "first"
	command.Payload.Intent = "new_work"
	first, err := r.AcceptConversationInput(scope, command)
	if err != nil || first.Input.WorkID != "" {
		t.Fatal("new-work input inherited focus", first, err)
	}
	unchanged, _ = m.GetTask(work.RunID)
	if hasConversationHold(unchanged) {
		t.Fatal("separate work held focused task")
	}
	command.RequestID, command.ExpectedRevision, command.Payload.Intent = "second", first.Revision, "message"
	second, err := r.AcceptConversationInput(scope, command)
	if err != nil || second.Input.WorkID != work.ID {
		t.Fatal(second, err)
	}
	page, err = m.ConversationInbox(scope, c.ID, 0, 1)
	if err != nil || len(page.Items) != 1 || !page.HasMore || page.Items[0].ID != first.Input.ID {
		t.Fatal(page, err)
	}
	page, err = m.ConversationInbox(scope, c.ID, page.NextCursor, 1)
	if err != nil || len(page.Items) != 1 || page.HasMore || page.Items[0].ID != second.Input.ID {
		t.Fatal(page, err)
	}
	if _, err = m.ConversationInbox(ControlScope{scope.Workspace, "other"}, c.ID, 0, 10); !errors.Is(err, ErrControlNotFound) {
		t.Fatal("foreign inbox exposed", err)
	}
	command.Payload.Content = "different"
	if _, err = r.AcceptConversationInput(scope, command); !errors.Is(err, ErrRunConflict) {
		t.Fatal("conflicting request reused", err)
	}
}

func TestConversationInboxFencesWorkersWithoutStoppingIndependentWork(t *testing.T) {
	m, r, scope, root, child := workerFixture(t)
	c, err := m.Conversation(scope, root.Origin.Conversation)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = r.AcceptConversationInput(scope, inboxCommand(c, "question", "Why is this approach useful?")); err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{root.ID, child.ID} {
		held, _ := m.GetTask(id)
		if !hasConversationHold(held) || held.ControlAction != "" {
			t.Fatal("input stopped work instead of holding it", held)
		}
	}
	if candidates := m.coordinationCandidates(); len(candidates) != 0 {
		t.Fatal("held graph remained schedulable", candidates)
	}
	if _, err = r.Continue(context.Background(), child.ID, scope.Actor, "start", "", false); !errors.Is(err, ErrRunConflict) {
		t.Fatal("child bypassed hold", err)
	}
	work, _ := m.WorkItem(scope, root.WorkID)
	if _, err = r.ControlWork(scope, api.WorkCommand{RequestID: "stop", TargetID: work.ID, ExpectedRevision: work.Revision, Kind: "stop"}); err != nil {
		t.Fatal("inbox blocked Stop", err)
	}
	stopped, _ := m.GetTask(root.ID)
	if stopped.Status != TaskCancelled {
		t.Fatal(stopped.Status)
	}
}
