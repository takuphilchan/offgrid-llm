package agents

import (
	"context"
	"strings"
	"testing"

	"github.com/takuphilchan/offgrid-llm/pkg/api"
)

func enableQuestionReview(t *testing.T, m *Manager, root *Task) {
	t.Helper()
	if _, err := m.updateTask(root.ID, func(task *Task) error {
		task.Config.WorkerQuestionsVersion = 1
		task.Config.ContextWindow = 16384
		return nil
	}); err != nil {
		t.Fatal(err)
	}
}

func TestWorkerQuestionPrimaryAnswersWithinSameWork(t *testing.T) {
	m, r, scope, root, child := waitingWorkerQuestion(t)
	defer r.Shutdown(context.Background())
	enableQuestionReview(t, m, root)
	r.caller = func(_ context.Context, task *Task, messages []api.ChatMessage, tools []api.Tool) (*api.ChatCompletionResponse, error) {
		if reviewingWorkerQuestion(task) {
			if len(tools) != 1 || tools[0].Function.Name != "worker_answer" {
				t.Error("review widened capability menu", tools)
			}
			found := false
			for _, message := range messages {
				found = found || strings.Contains(message.StringContent(), child.WorkerQuestion.Content)
			}
			if !found {
				t.Error("primary did not receive the question")
			}
			return taskCall("worker_answer", `{"content":"Include offline runtimes and label missing evidence.","needs_user":false}`), nil
		}
		return runAnswer(api.ChatMessage{Role: "assistant", Content: "Offline runtimes included with evidence limitations."}), nil
	}
	r.advanceDelegation(context.Background(), root.ID)
	waiting := awaitTask(t, m, root.ID, TaskChildren)
	settleTask(t, r, root.ID)
	if reviewingWorkerQuestion(waiting) || waiting.Checkpoint.Iteration != root.Checkpoint.Iteration+1 || waiting.Config.MaxIterations != root.Config.MaxIterations || waiting.Checkpoint.ReservedChildIterations != root.Checkpoint.ReservedChildIterations || len(waiting.ChildHistory) != 1 {
		t.Fatal("review reopened work or allowance", waiting)
	}
	worker, _ := m.WorkerSession(scope, child.Worker.SessionID)
	if worker.Question.State != "answered" || worker.CurrentRunID != child.ID {
		t.Fatal(worker)
	}
	if _, err := r.Continue(context.Background(), child.ID, scope.Actor, "start", "", false); err != nil {
		t.Fatal(err)
	}
	r.advanceDelegation(context.Background(), root.ID)
	done := awaitTask(t, m, root.ID, TaskCompleted)
	settleTask(t, r, root.ID)
	if done.Result != "Offline runtimes included with evidence limitations." {
		t.Fatal(done)
	}
	// Each tool invocation gets exactly one reply, including the original
	// delegation call answered early to deliver the worker's question.
	replies := map[string]int{}
	for _, message := range done.Checkpoint.Messages {
		if message.Role == "tool" {
			replies[message.ToolCallID]++
		}
	}
	for id, count := range replies {
		if count != 1 {
			t.Fatal("duplicated tool receipt", id, count)
		}
	}
	page, err := m.WorkerMessages(scope, worker.ID, 0, 10)
	if err != nil || len(page.Items) != 1 || page.Items[0].State != "applied" {
		t.Fatal(page, err)
	}
}

func TestWorkerQuestionPrimaryEscalationAnswerIsAtomic(t *testing.T) {
	m, r, scope, root, child := waitingWorkerQuestion(t)
	defer r.Shutdown(context.Background())
	enableQuestionReview(t, m, root)
	r.caller = func(_ context.Context, _ *Task, _ []api.ChatMessage, _ []api.Tool) (*api.ChatCompletionResponse, error) {
		return taskCall("worker_answer", `{"content":"Should we include offline runtimes in this comparison?","needs_user":true}`), nil
	}
	r.advanceDelegation(context.Background(), root.ID)
	waiting := awaitTask(t, m, root.ID, TaskInput)
	settleTask(t, r, root.ID)
	if waiting.PendingInput.Kind != "worker_clarification" || waiting.PendingInput.ID != child.WorkerQuestion.ID {
		t.Fatal(waiting)
	}
	for _, kind := range []string{"pause", "continue"} {
		work, _ := m.WorkItem(scope, root.WorkID)
		if _, err := r.ControlWork(scope, api.WorkCommand{RequestID: kind + "-clarification", TargetID: work.ID, ExpectedRevision: work.Revision, Kind: kind}); err != nil {
			t.Fatal("clarification control stranded work", kind, err)
		}
	}
	resumed, _ := m.GetTask(root.ID)
	if resumed.Status != TaskInput || resumed.PendingInput.ID != waiting.PendingInput.ID || resumed.SchedulerReady {
		t.Fatal("continuation dispatched or lost clarification")
	}
	worker, _ := m.WorkerSession(scope, child.Worker.SessionID)
	command := api.WorkerCommand{RequestID: "user-answer", TargetID: worker.ID, ExpectedRevision: worker.Revision, Kind: "answer", Payload: api.WorkerCommandPayload{QuestionID: worker.Question.ID, Content: "Yes, include offline runtimes."}}
	db, err := m.controlDatabase()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err = db.Exec(`CREATE TRIGGER fail_primary_answer BEFORE INSERT ON agent_worker_messages BEGIN SELECT RAISE(ABORT,'fixture'); END`); err != nil {
		t.Fatal(err)
	}
	if _, err = r.CommandWorker(scope, command); err == nil {
		t.Fatal("fault committed answer")
	}
	unchanged, _ := m.GetTask(root.ID)
	if unchanged.Status != TaskInput || !reviewingWorkerQuestion(unchanged) {
		t.Fatal("failed answer resumed parent")
	}
	if _, err = db.Exec(`DROP TRIGGER fail_primary_answer`); err != nil {
		t.Fatal(err)
	}
	if _, err = r.CommandWorker(scope, command); err != nil {
		t.Fatal(err)
	}
	if _, err = r.CommandWorker(scope, command); err != nil {
		t.Fatal("lost acknowledgement retry", err)
	}
	updated, _ := m.GetTask(root.ID)
	answered, _ := m.GetTask(child.ID)
	if updated.Status != TaskChildren || reviewingWorkerQuestion(updated) || updated.PendingInput != nil || updated.Checkpoint.Iteration != waiting.Checkpoint.Iteration || answered.Status != TaskPending || answered.WorkerQuestion.State != "answered" {
		t.Fatal("answer did not atomically release wait")
	}
	if err = NewManagerWithPersistence(nil, nil, nil, m.dataDir).StorageError(); err != nil {
		t.Fatal("saved review invalid after restart", err)
	}
}

func TestWorkerQuestionPrimaryStopWinsLateAnswer(t *testing.T) {
	m, r, scope, root, child := waitingWorkerQuestion(t)
	defer r.Shutdown(context.Background())
	enableQuestionReview(t, m, root)
	entered, release := make(chan struct{}), make(chan struct{})
	r.caller = func(_ context.Context, _ *Task, _ []api.ChatMessage, _ []api.Tool) (*api.ChatCompletionResponse, error) {
		close(entered)
		<-release
		return taskCall("worker_answer", `{"content":"Include offline runtimes.","needs_user":false}`), nil
	}
	r.advanceDelegation(context.Background(), root.ID)
	<-entered
	work, _ := m.WorkItem(scope, root.WorkID)
	_, err := r.ControlWork(scope, api.WorkCommand{RequestID: "stop-review", TargetID: work.ID, ExpectedRevision: work.Revision, Kind: "stop"})
	close(release)
	if err != nil {
		t.Fatal(err)
	}
	settleTask(t, r, root.ID)
	stopped, _ := m.GetTask(root.ID)
	worker, _ := m.WorkerSession(scope, child.Worker.SessionID)
	if stopped.Status != TaskCancelled || worker.Question.State != "pending" || !worker.Stopped {
		t.Fatal("late answer defeated Stop", stopped.Status, worker)
	}
}
