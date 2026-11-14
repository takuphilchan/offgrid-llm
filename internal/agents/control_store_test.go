package agents

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/takuphilchan/offgrid-llm/pkg/api"
)

func controlFixture(t *testing.T) (*Manager, *Runner, ControlScope, *Conversation) {
	t.Helper()
	m := NewManagerWithPersistence(nil, nil, nil, t.TempDir())
	if err := m.StorageError(); err != nil {
		t.Fatal(err)
	}
	r := NewRunner(m, NewToolRegistry(), func(context.Context, *Task, []api.ChatMessage, []api.Tool) (*api.ChatCompletionResponse, error) {
		return runAnswer(api.ChatMessage{Role: "assistant", Content: "Recorded answer"}), nil
	})
	scope := ControlScope{"workspace", "alice"}
	receipt, err := m.ApplyConversationCommand(scope, ConversationCommand{RequestID: "new-conversation", Kind: "create_conversation"})
	if err != nil {
		t.Fatal(err)
	}
	return m, r, scope, receipt.Conversation
}

func TestControlCommandsDedupeConcurrentRevisionsAndScopes(t *testing.T) {
	m, _, scope, c := controlFixture(t)
	command := ConversationCommand{RequestID: "rename-once", TargetID: c.ID, ExpectedRevision: c.Revision, Kind: "rename_conversation", Payload: ConversationCommandPayload{Title: "Renamed"}}
	var wg sync.WaitGroup
	for i := 0; i < 16; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			receipt, err := m.ApplyConversationCommand(scope, command)
			if err != nil || receipt.Revision != 2 {
				t.Errorf("retry: %#v %v", receipt, err)
			}
		}()
	}
	wg.Wait()
	changed := command
	changed.Payload.Title = "Different"
	if _, err := m.ApplyConversationCommand(scope, changed); !errors.Is(err, ErrRunConflict) {
		t.Fatal("conflicting reuse accepted", err)
	}
	var accepted atomic.Int32
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			next := command
			next.RequestID = runID()
			next.ExpectedRevision = 2
			_, err := m.ApplyConversationCommand(scope, next)
			if err == nil {
				accepted.Add(1)
			} else if !errors.Is(err, ErrRunConflict) {
				t.Error(err)
			}
		}()
	}
	wg.Wait()
	if accepted.Load() != 1 {
		t.Fatal("concurrent writers did not conflict", accepted.Load())
	}
	for _, foreign := range []ControlScope{{"workspace", "bob"}, {"other", "alice"}} {
		if _, err := m.Conversation(foreign, c.ID); !errors.Is(err, ErrControlNotFound) {
			t.Fatal("cross-scope read", err)
		}
		if _, err := m.ControlEvents(foreign, c.ID, "conversation", 0, 10); !errors.Is(err, ErrControlNotFound) {
			t.Fatal("cross-scope events", err)
		}
	}
	page, err := m.ControlEvents(scope, c.ID, "conversation", 0, 1)
	if err != nil || len(page.Events) != 1 || !page.HasMore {
		t.Fatal("bounded first page", page, err)
	}
	next, err := m.ControlEvents(scope, c.ID, "conversation", page.NextCursor, 10)
	if err != nil || len(next.Events) != 2 || next.HasMore {
		t.Fatal("ordered next page", next, err)
	}
}

func TestControlAdmissionRollsBackBeforeDispatchAndRetrySurvivesRestart(t *testing.T) {
	m, r, scope, c := controlFixture(t)
	command := ConversationCommand{RequestID: "work-once", TargetID: c.ID, ExpectedRevision: 1, Kind: "create_work", Payload: ConversationCommandPayload{Input: &ConversationInput{Content: "Do the accepted work"}}}
	db, err := m.controlDatabase()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	// Fail the final record after the work, task, input, event and outbox writes.
	_, err = db.Exec(`CREATE TRIGGER fail_control_commit BEFORE INSERT ON agent_control_commands WHEN NEW.request_id='work-once' BEGIN SELECT RAISE(ABORT,'injected receipt failure'); END`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = r.AdmitConversationWork(scope, command, "model", taskConfig()); err == nil {
		t.Fatal("injection did not fail")
	}
	for _, table := range []string{"agent_tasks", "agent_conversation_messages", "agent_control_links"} {
		var count int
		if err = db.QueryRow(`SELECT COUNT(*) FROM ` + table).Scan(&count); err != nil || count != 0 {
			t.Fatal("partial transaction", table, count, err)
		}
	}
	if len(m.tasks) != 0 {
		t.Fatal("rolled-back run published")
	}
	if _, err = db.Exec(`DROP TRIGGER fail_control_commit`); err != nil {
		t.Fatal(err)
	}
	accepted, err := r.AdmitConversationWork(scope, command, "model", taskConfig())
	if err != nil {
		t.Fatal(err)
	}
	if accepted.Work == nil || accepted.Work.InstructionVersion != 1 {
		t.Fatal("missing work", accepted)
	}
	task, exists := m.GetTask(accepted.Work.RunID)
	if !exists || task.AdmissionHeld || !task.SchedulerReady || task.WorkID != accepted.Work.ID {
		t.Fatal("unpublished dispatch", task)
	}
	// No model invocation or projection is required to accept a command.
	reloaded := NewManagerWithPersistence(nil, nil, nil, m.dataDir)
	if err = reloaded.StorageError(); err != nil {
		t.Fatal(err)
	}
	other := NewRunner(reloaded, NewToolRegistry(), nil)
	retry, err := other.AdmitConversationWork(scope, command, "removed-model", AgentConfig{})
	if err != nil || retry.Work.RunID != task.ID {
		t.Fatal("retry re-resolved or re-created work", retry, err)
	}
	loaded, _ := reloaded.GetTask(task.ID)
	if loaded.Status != TaskInterrupted {
		t.Fatal("restart silently dispatched", loaded.Status)
	}
	work, err := reloaded.WorkItem(scope, task.WorkID)
	if err != nil || work.Status != TaskInterrupted {
		t.Fatal("work and run recovery disagree", work, err)
	}
	var count int
	if err = db.QueryRow(`SELECT COUNT(*) FROM agent_tasks`).Scan(&count); err != nil || count != 1 {
		t.Fatal("duplicate run", count, err)
	}
}

func TestControlRunStateAndReceiptsCommitTogether(t *testing.T) {
	m, r, scope, c := controlFixture(t)
	command := ConversationCommand{RequestID: "work-result", TargetID: c.ID, ExpectedRevision: 1, Kind: "create_work", Payload: ConversationCommandPayload{Input: &ConversationInput{Content: "Answer"}}}
	receipt, err := r.AdmitConversationWork(scope, command, "model", taskConfig())
	if err != nil {
		t.Fatal(err)
	}
	task, err := r.Continue(context.Background(), receipt.Work.RunID, scope.Actor, "start", "", false)
	if err != nil {
		t.Fatal(err)
	}
	work, err := m.WorkItem(scope, receipt.Work.ID)
	if err != nil || work.Status != TaskCompleted || work.ResultReference != resultReference(task) || work.IndependentlyVerified {
		t.Fatal("incorrect durable outcome", work, err, task.ErrorCode, task.Error)
	}
	page, err := m.ControlEvents(scope, work.ID, "work", 0, 100)
	if err != nil {
		t.Fatal(err)
	}
	var last WorkItem
	if err = json.Unmarshal(page.Events[len(page.Events)-1].Payload, &last); err != nil || last.Status != TaskCompleted {
		t.Fatal("missing committed completion", last, err)
	}
}
