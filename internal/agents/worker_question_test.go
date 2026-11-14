package agents

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/takuphilchan/offgrid-llm/internal/inference"
	"github.com/takuphilchan/offgrid-llm/pkg/api"
)

func waitingWorkerQuestion(t *testing.T) (*Manager, *Runner, ControlScope, *Task, *Task) {
	t.Helper()
	m, r, scope, root, child := workerFixture(t)
	if _, err := m.updateTask(child.ID, func(next *Task) error {
		next.Config.WorkerQuestionsVersion, next.Config.TaskFirst = 1, true
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	r.caller = func(_ context.Context, task *Task, _ []api.ChatMessage, tools []api.Tool) (*api.ChatCompletionResponse, error) {
		found := false
		for _, tool := range tools {
			found = found || tool.Function.Name == "worker_question"
		}
		if !found {
			t.Error("worker cannot ask primary for clarification")
		}
		return taskCall("worker_question", `{"question":"Should the comparison include offline runtimes? Existing notes only cover hosted runtimes."}`), nil
	}
	waiting, err := r.Continue(context.Background(), child.ID, scope.Actor, "start", "", false)
	if err != nil || waiting.Status != TaskInput || waiting.WorkerQuestion == nil || waiting.PendingInput.Kind != "worker_question" {
		t.Fatal(waiting, err)
	}
	return m, r, scope, root, waiting
}

func TestWorkerQuestionAnswerKeepsRunBudgetAndAppliesOnce(t *testing.T) {
	m, r, scope, root, child := waitingWorkerQuestion(t)
	worker, err := m.WorkerSession(scope, child.Worker.SessionID)
	if err != nil || worker.Question == nil || worker.Question.ID != child.WorkerQuestion.ID {
		t.Fatal(worker, err)
	}
	command := api.WorkerCommand{RequestID: "answer", TargetID: worker.ID, ExpectedRevision: worker.Revision, Kind: "answer", Payload: api.WorkerCommandPayload{QuestionID: worker.Question.ID, Content: "Include offline runtimes and label the evidence gaps."}}
	accepted, err := r.CommandWorker(scope, command)
	if err != nil || accepted.Worker.Question.State != "answered" || accepted.Message.QuestionID != worker.Question.ID || accepted.Message.State != "accepted" {
		t.Fatal(accepted, err)
	}
	if _, err = r.CommandWorker(scope, command); err != nil {
		t.Fatal(err)
	}
	ready, _ := m.GetTask(child.ID)
	if ready.ID != child.ID || ready.Config.MaxIterations != child.Config.MaxIterations || ready.Checkpoint.Iteration != child.Checkpoint.Iteration || ready.Status != TaskPending || ready.PendingInput != nil {
		t.Fatal("answer reopened budget or recreated worker", ready)
	}
	unchanged, _ := m.GetTask(root.ID)
	if unchanged.Status != TaskChildren || unchanged.Checkpoint.ReservedChildIterations != root.Checkpoint.ReservedChildIterations {
		t.Fatal("answer changed parent budget", unchanged)
	}
	r.caller = func(_ context.Context, _ *Task, messages []api.ChatMessage, _ []api.Tool) (*api.ChatCompletionResponse, error) {
		answers := 0
		for _, message := range messages {
			if message.Role == "tool" && message.Name == "worker_question" && strings.Contains(message.StringContent(), command.Payload.Content) {
				answers++
			}
		}
		if answers != 1 {
			t.Error("question answer missing or duplicated", answers)
		}
		return runAnswer(api.ChatMessage{Role: "assistant", Content: "Comparison includes offline runtimes; evidence gaps are labeled."}), nil
	}
	done, err := r.Continue(context.Background(), child.ID, scope.Actor, "start", "", false)
	if err != nil || done.Status != TaskCompleted || len(done.Instructions) != 1 {
		t.Fatal(done, err)
	}
	page, err := m.WorkerMessages(scope, worker.ID, 0, 10)
	if err != nil || len(page.Items) != 1 || page.Items[0].State != "applied" || page.Items[0].DeliveredAt == nil || page.Items[0].AppliedAt == nil {
		t.Fatal(page, err)
	}
}

func TestWorkerQuestionRejectsWrongQuestionStopAndCorrection(t *testing.T) {
	for _, change := range []string{"question", "stop", "correction", "actor"} {
		t.Run(change, func(t *testing.T) {
			m, r, scope, root, child := waitingWorkerQuestion(t)
			worker, _ := m.WorkerSession(scope, child.Worker.SessionID)
			command := api.WorkerCommand{RequestID: "answer", TargetID: worker.ID, ExpectedRevision: worker.Revision, Kind: "answer", Payload: api.WorkerCommandPayload{QuestionID: worker.Question.ID, Content: "Proceed"}}
			switch change {
			case "question":
				command.Payload.QuestionID = "question-other"
			case "actor":
				scope.Actor = "other"
			case "stop":
				stopped, err := r.CommandWorker(scope, api.WorkerCommand{RequestID: "stop", TargetID: worker.ID, ExpectedRevision: worker.Revision, Kind: "stop"})
				if err != nil {
					t.Fatal(err)
				}
				command.ExpectedRevision = stopped.Revision
			case "correction":
				c, _ := m.Conversation(scope, root.Origin.Conversation)
				_, err := r.AcceptConversationInput(scope, api.ConversationInputCommand{RequestID: "correct", TargetID: c.ID, ExpectedRevision: c.Revision, Kind: "submit_input", Payload: api.ConversationInboxInput{Content: "Only compare the already reviewed material", Intent: "correction"}})
				if err != nil {
					t.Fatal(err)
				}
			}
			_, err := r.CommandWorker(scope, command)
			if !errors.Is(err, ErrRunConflict) && !errors.Is(err, ErrControlNotFound) {
				t.Fatal("unsafe answer accepted", err)
			}
			unchanged, _ := m.GetTask(child.ID)
			if unchanged.WorkerQuestion.State != "pending" || len(unchanged.PendingInstructions) != 0 {
				t.Fatal("rejected answer changed worker", unchanged)
			}
		})
	}
}

func TestWorkerQuestionSurvivesRestartWithoutExecution(t *testing.T) {
	m, _, scope, root, child := waitingWorkerQuestion(t)
	reloaded := NewManagerWithPersistence(nil, nil, nil, m.dataDir)
	if err := reloaded.StorageError(); err != nil {
		t.Fatal(err)
	}
	worker, err := reloaded.WorkerSession(scope, child.Worker.SessionID)
	if err != nil || worker.Question == nil || worker.Question.ID != child.WorkerQuestion.ID || worker.CurrentRunID != child.ID {
		t.Fatal(worker, err)
	}
	r := NewRunner(reloaded, NewToolRegistry(), nil)
	_, err = r.CommandWorker(scope, api.WorkerCommand{RequestID: "late-answer", TargetID: worker.ID, ExpectedRevision: worker.Revision, Kind: "answer", Payload: api.WorkerCommandPayload{QuestionID: worker.Question.ID, Content: "Include offline"}})
	if !errors.Is(err, ErrRunConflict) {
		t.Fatal("answer resumed interrupted parent", err)
	}
	parent, _ := reloaded.GetTask(root.ID)
	if parent.Status != TaskInterrupted {
		t.Fatal("restart revived parent", parent.Status)
	}
	work, _ := reloaded.WorkItem(scope, root.WorkID)
	if _, err = r.ControlWork(scope, api.WorkCommand{RequestID: "explicit-continuation", TargetID: work.ID, ExpectedRevision: work.Revision, Kind: "continue"}); err != nil {
		t.Fatal(err)
	}
	worker, _ = reloaded.WorkerSession(scope, child.Worker.SessionID)
	waiting, _ := reloaded.GetTask(child.ID)
	parent, _ = reloaded.GetTask(root.ID)
	if parent.Status != TaskChildren || waiting.Status != TaskInput || waiting.SchedulerReady || waiting.PendingInput == nil || waiting.WorkerQuestion.State != "pending" || waiting.Checkpoint.Iteration != child.Checkpoint.Iteration {
		t.Fatal("continuation dispatched or discarded pending question")
	}
	if _, err = r.CommandWorker(scope, api.WorkerCommand{RequestID: "after-continuation", TargetID: worker.ID, ExpectedRevision: worker.Revision, Kind: "answer", Payload: api.WorkerCommandPayload{QuestionID: worker.Question.ID, Content: "Include offline"}}); err != nil {
		t.Fatal("explicit continuation stranded question", err)
	}
}

func TestWorkerQuestionContinuationRetainsFences(t *testing.T) {
	_, _, _, _, child := waitingWorkerQuestion(t)
	for _, tc := range []struct {
		name   string
		mutate func(*Task)
		active bool
	}{
		{"active", func(*Task) {}, true},
		{"stopped", func(task *Task) { task.ControlAction = "stop" }, false},
		{"unsettled control", func(task *Task) { task.ControlPending = true }, false},
		{"expired authority", func(task *Task) { task.ComputerSessionExpired = true }, false},
		{"executing", func(task *Task) { task.Checkpoint.ExecutingCall = "unsettled" }, false},
		{"stale instruction", func(task *Task) { task.InstructionVersion++ }, false},
		{"stale assignment", func(task *Task) { task.AssignmentVersion++ }, false},
		{"missing question", func(task *Task) { task.WorkerQuestion = nil }, false},
		{"different input", func(task *Task) { task.PendingInput.Kind = "workspace" }, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			task, err := copyTask(child)
			if err != nil {
				t.Fatal(err)
			}
			tc.mutate(task)
			if err = prepareWorkContinuation(task, tc.active); !errors.Is(err, ErrRunConflict) {
				t.Fatal("unsafe question continuation accepted", err)
			}
		})
	}
}

func TestWorkerQuestionAnswerRollbackAndRestartRetry(t *testing.T) {
	m, r, scope, _, child := waitingWorkerQuestion(t)
	worker, _ := m.WorkerSession(scope, child.Worker.SessionID)
	command := api.WorkerCommand{RequestID: "answer", TargetID: worker.ID, ExpectedRevision: worker.Revision, Kind: "answer", Payload: api.WorkerCommandPayload{QuestionID: worker.Question.ID, Content: "Include offline runtimes"}}
	db, err := m.controlDatabase()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err = db.Exec(`CREATE TRIGGER fail_question BEFORE INSERT ON agent_worker_messages BEGIN SELECT RAISE(ABORT,'fixture'); END`); err != nil {
		t.Fatal(err)
	}
	if _, err = r.CommandWorker(scope, command); err == nil {
		t.Fatal("fault accepted")
	}
	live, _ := m.GetTask(child.ID)
	if live.WorkerQuestion.State != "pending" || live.PendingInput == nil || len(live.PendingInstructions) != 0 {
		t.Fatal("failed transaction published answer")
	}
	if _, err = db.Exec(`DROP TRIGGER fail_question`); err != nil {
		t.Fatal(err)
	}
	if _, err = r.CommandWorker(scope, command); err != nil {
		t.Fatal(err)
	}
	reloaded := NewManagerWithPersistence(nil, nil, nil, m.dataDir)
	if err = reloaded.StorageError(); err != nil {
		t.Fatal(err)
	}
	restarted := NewRunner(reloaded, NewToolRegistry(), nil)
	if _, err = restarted.CommandWorker(scope, command); err != nil {
		t.Fatal("lost acknowledgment could not recover", err)
	}
	live, _ = reloaded.GetTask(child.ID)
	if live.Status != TaskInterrupted || live.WorkerQuestion.State != "answered" || len(live.PendingInstructions) != 1 {
		t.Fatal("retry revived or duplicated answer", live)
	}
	page, err := reloaded.WorkerMessages(scope, worker.ID, 0, 10)
	if err != nil || len(page.Items) != 1 || page.Items[0].State != "accepted" {
		t.Fatal(page, err)
	}
}

func TestWorkerQuestionAdmissionIsExplicitAndDoesNotInheritNativeQualification(t *testing.T) {
	_, _, _, _, child := workerFixture(t)
	child.Config.TaskFirst = true
	for _, tool := range runtimeTools(child) {
		if tool.Function.Name == "worker_question" {
			t.Fatal("question tool admitted before routing qualification")
		}
	}
	child.Config.WorkerQuestionsVersion = 1
	child.Config.ToolAdapter = &inference.AgentToolPin{Adapter: inference.NativeAgentToolAdapter}
	for _, tool := range runtimeTools(child) {
		if tool.Function.Name == "worker_question" {
			t.Fatal("new tool inherited old native qualification")
		}
	}
}
