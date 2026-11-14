package agents

import (
	"context"
	"errors"
	"github.com/takuphilchan/offgrid-llm/internal/execution"
	"sync"
	"testing"

	"github.com/takuphilchan/offgrid-llm/pkg/api"
)

func conversationQuestionFixture(t *testing.T) (*Manager, *Runner, ControlScope, *Task, *Task, *Conversation) {
	t.Helper()
	m, r, scope, root, child := waitingWorkerQuestion(t)
	t.Cleanup(func() { r.Shutdown(context.Background()) })
	enableQuestionReview(t, m, root)
	r.caller = func(context.Context, *Task, []api.ChatMessage, []api.Tool) (*api.ChatCompletionResponse, error) {
		return taskCall("worker_answer", `{"content":"Should we include offline runtimes?","needs_user":true}`), nil
	}
	r.advanceDelegation(context.Background(), root.ID)
	root = awaitTask(t, m, root.ID, TaskInput)
	settleTask(t, r, root.ID)
	c, err := m.Conversation(scope, root.Origin.Conversation)
	if err != nil {
		t.Fatal(err)
	}
	return m, r, scope, root, child, c
}

func questionInputCommand(c *Conversation, root *Task) api.ConversationInputCommand {
	command := inboxCommand(c, "question-answer", "Include offline runtimes.")
	command.Payload.Intent, command.Payload.QuestionID = "answer", root.PendingInput.ID
	return command
}

func TestConversationQuestionAnswerAtomicRetryAndRestart(t *testing.T) {
	m, r, scope, root, child, c := conversationQuestionFixture(t)
	r.caller = nil
	command := questionInputCommand(c, root)
	var wg sync.WaitGroup
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			receipt, err := r.AcceptConversationInput(scope, command)
			if err != nil || receipt.Input.Incorporation != "worker_answer" || receipt.Input.AppliedAt == nil {
				t.Error(receipt, err)
			}
		}()
	}
	wg.Wait()
	parent, _ := m.GetTask(root.ID)
	worker, _ := m.GetTask(child.ID)
	if parent.Status != TaskChildren || worker.Status != TaskPending || reviewingWorkerQuestion(parent) || len(worker.PendingInstructions) != 1 || parent.Checkpoint.BudgetedModelCalls != root.Checkpoint.BudgetedModelCalls || len(m.ListTasks()) != 2 {
		t.Fatal("answer dispatched, duplicated or reset work")
	}
	page, err := m.ConversationInbox(scope, c.ID, 0, 10)
	if err != nil || len(page.Items) != 1 || page.Items[0].RunID != "" || page.Items[0].State != "responded" {
		t.Fatal(page, err)
	}
	mail, err := m.WorkerMessages(scope, child.Worker.SessionID, 0, 10)
	if err != nil || len(mail.Items) != 1 || mail.Items[0].State != "accepted" || mail.Items[0].ID != page.Items[0].ID {
		t.Fatal(mail, err)
	}
	reloaded := NewManagerWithPersistence(nil, nil, nil, m.dataDir)
	if err = reloaded.StorageError(); err != nil {
		t.Fatal(err)
	}
	restarted := NewRunner(reloaded, NewToolRegistry(), nil)
	if _, err = restarted.AcceptConversationInput(scope, command); err != nil {
		t.Fatal("lost acknowledgment unrecoverable", err)
	}
	parent, _ = reloaded.GetTask(root.ID)
	if parent.Status != TaskInterrupted {
		t.Fatal("retry resumed interrupted work")
	}
	command.Payload.Content = "Different answer"
	if _, err = restarted.AcceptConversationInput(scope, command); !errors.Is(err, ErrRunConflict) {
		t.Fatal("conflicting identity accepted", err)
	}
}

func TestConversationQuestionAnswerRejectsStaleAndStoppedQuestions(t *testing.T) {
	for _, mode := range []string{"wrong-question", "stop", "pause", "correction", "foreign", "host-approval"} {
		t.Run(mode, func(t *testing.T) {
			m, r, scope, root, child, c := conversationQuestionFixture(t)
			command := questionInputCommand(c, root)
			switch mode {
			case "wrong-question":
				command.Payload.QuestionID = "old-question"
			case "foreign":
				scope.Actor = "other"
			case "host-approval":
				if _, err := m.updateTask(root.ID, func(task *Task) error {
					task.PendingInput.Kind, task.PendingInput.Mode, task.PendingInput.CallID = "workspace", "read", "access-call"
					task.Delegation = nil
					task.Config.ExecutionRequestVersion = execution.AccessRequestVersion
					task.Checkpoint.Calls = []api.ToolCall{{ID: "access-call", Type: "function", Function: api.FunctionCall{Name: execution.AccessRequestTool, Arguments: `{"access":"read"}`}}}
					return nil
				}); err != nil {
					t.Fatal(err)
				}
			case "correction":
				if _, err := r.AcceptConversationInput(scope, inboxCommand(c, "correction", "Only hosted runtimes")); err != nil {
					t.Fatal(err)
				}
				c, _ = m.Conversation(scope, c.ID)
				command.ExpectedRevision = c.Revision
			default:
				work, _ := m.WorkItem(scope, root.WorkID)
				if _, err := r.ControlWork(scope, api.WorkCommand{RequestID: mode, TargetID: work.ID, ExpectedRevision: work.Revision, Kind: mode}); err != nil {
					t.Fatal(err)
				}
			}
			if _, err := r.AcceptConversationInput(scope, command); err == nil {
				t.Fatal("unsafe answer accepted")
			}
			worker, _ := m.GetTask(child.ID)
			if worker.WorkerQuestion.State != "pending" {
				t.Fatal("rejection changed worker")
			}
		})
	}
}

func TestConversationQuestionAnswerRollbackIncludesHistory(t *testing.T) {
	m, r, scope, root, child, c := conversationQuestionFixture(t)
	db, err := m.controlDatabase()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err = db.Exec(`CREATE TRIGGER reject_answer_history BEFORE INSERT ON agent_conversation_messages BEGIN SELECT RAISE(ABORT,'fixture'); END`); err != nil {
		t.Fatal(err)
	}
	command := questionInputCommand(c, root)
	if _, err = r.AcceptConversationInput(scope, command); err == nil {
		t.Fatal("fault committed")
	}
	parent, _ := m.GetTask(root.ID)
	worker, _ := m.GetTask(child.ID)
	page, _ := m.ConversationInbox(scope, c.ID, 0, 10)
	mail, _ := m.WorkerMessages(scope, child.Worker.SessionID, 0, 10)
	if parent.Status != TaskInput || worker.WorkerQuestion.State != "pending" || len(page.Items)+len(mail.Items) != 0 {
		t.Fatal("failed transaction published partial answer")
	}
	if _, err = db.Exec(`DROP TRIGGER reject_answer_history`); err != nil {
		t.Fatal(err)
	}
	if _, err = r.AcceptConversationInput(scope, command); err != nil {
		t.Fatal(err)
	}
}
