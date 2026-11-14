package agents

import (
	"context"
	"encoding/json"
	"sync/atomic"
	"testing"

	"github.com/takuphilchan/offgrid-llm/pkg/api"
)

func TestWorkerQuestionReviewWorkerStopReleasesParent(t *testing.T) {
	m, r, scope, root, child := waitingWorkerQuestion(t)
	defer r.Shutdown(context.Background())
	enableQuestionReview(t, m, root)
	entered, release := make(chan struct{}), make(chan struct{})
	r.caller = func(_ context.Context, _ *Task, _ []api.ChatMessage, _ []api.Tool) (*api.ChatCompletionResponse, error) {
		close(entered)
		<-release
		return taskCall("worker_answer", `{"content":"Should we include offline runtimes?","needs_user":true}`), nil
	}
	r.advanceDelegation(context.Background(), root.ID)
	<-entered
	worker, _ := m.WorkerSession(scope, child.Worker.SessionID)
	_, err := r.CommandWorker(scope, api.WorkerCommand{RequestID: "stop-question-worker", TargetID: worker.ID, ExpectedRevision: worker.Revision, Kind: "stop"})
	close(release)
	if err != nil {
		t.Fatal(err)
	}
	settleTask(t, r, root.ID)
	r.advanceDelegation(context.Background(), root.ID)
	parent, _ := m.GetTask(root.ID)
	if parent.Status != TaskFailed || parent.ErrorCode != "delegation_partial" || parent.PendingInput != nil {
		t.Fatal("stopped worker stranded parent", parent.Status, parent.ErrorCode)
	}
}

func TestWorkerQuestionReviewRevocationPreventsAnswer(t *testing.T) {
	m, r, _, root, child := waitingWorkerQuestion(t)
	defer r.Shutdown(context.Background())
	enableQuestionReview(t, m, root)
	if _, err := m.updateTask(child.ID, func(task *Task) error { task.Config.KnowledgeDependencies = []string{"revoked-source"}; return nil }); err != nil {
		t.Fatal(err)
	}
	var revoked atomic.Bool
	r.SourceAccess = func(task *Task) bool {
		for _, id := range task.Config.KnowledgeDependencies {
			if id == "revoked-source" && revoked.Load() {
				return false
			}
		}
		return true
	}
	r.caller = func(_ context.Context, task *Task, _ []api.ChatMessage, _ []api.Tool) (*api.ChatCompletionResponse, error) {
		found := false
		for _, id := range task.Config.KnowledgeDependencies {
			found = found || id == "revoked-source"
		}
		if !found {
			t.Error("review dropped source dependency")
		}
		revoked.Store(true)
		return taskCall("worker_answer", `{"content":"Include offline runtimes.","needs_user":false}`), nil
	}
	r.advanceDelegation(context.Background(), root.ID)
	settleTask(t, r, root.ID)
	parent, _ := m.GetTask(root.ID)
	worker, _ := m.GetTask(child.ID)
	if parent.Status != TaskFailed || worker.WorkerQuestion.State != "pending" {
		t.Fatal("revoked context answered worker", parent.Status)
	}
}

func TestWorkerQuestionReviewCannotSpendVerificationReserve(t *testing.T) {
	_, _, _, root, _ := waitingWorkerQuestion(t)
	root.Delegation.QuestionReview = &WorkerQuestionReview{}
	root.Config.MaxIterations = root.Checkpoint.ReservedChildIterations + max(root.Checkpoint.Iteration, root.Checkpoint.BudgetedModelCalls) + 2
	before := root.Checkpoint.BudgetedModelCalls
	if err := reserveModelBudget(root); err == nil || root.Checkpoint.BudgetedModelCalls != before {
		t.Fatal("question spent parent verification reserve", err)
	}
}

func TestWorkerQuestionReviewLostParentReceiptResumesWithoutReanswer(t *testing.T) {
	m, r, scope, root, child := waitingWorkerQuestion(t)
	defer r.Shutdown(context.Background())
	enableQuestionReview(t, m, root)
	db, err := m.controlDatabase()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err = db.Exec(`CREATE TRIGGER fail_review_wait BEFORE UPDATE OF snapshot ON agent_tasks
WHEN json_extract(CAST(OLD.snapshot AS TEXT),'$.checkpoint.worker_answer_command') IS NOT NULL
AND json_extract(CAST(NEW.snapshot AS TEXT),'$.status')='waiting_for_children'
BEGIN SELECT RAISE(ABORT,'fixture'); END`); err != nil {
		t.Fatal(err)
	}
	r.caller = func(_ context.Context, _ *Task, _ []api.ChatMessage, _ []api.Tool) (*api.ChatCompletionResponse, error) {
		return taskCall("worker_answer", `{"content":"Include offline runtimes.","needs_user":false}`), nil
	}
	r.advanceDelegation(context.Background(), root.ID)
	settleTask(t, r, root.ID)
	if _, err = db.Exec(`DROP TRIGGER fail_review_wait`); err != nil {
		t.Fatal(err)
	}
	reloaded := NewManagerWithPersistence(nil, nil, nil, m.dataDir)
	if err = reloaded.StorageError(); err != nil {
		t.Fatal(err)
	}
	parent, _ := reloaded.GetTask(root.ID)
	answered, _ := reloaded.GetTask(child.ID)
	if parent.Status != TaskInterrupted || parent.Checkpoint.WorkerAnswerCommand == nil || answered.WorkerQuestion.State != "answered" {
		t.Fatal("fault did not retain split acknowledgement", parent.Status, answered.WorkerQuestion.State)
	}
	restarted := NewRunner(reloaded, NewToolRegistry(), func(context.Context, *Task, []api.ChatMessage, []api.Tool) (*api.ChatCompletionResponse, error) {
		t.Error("receipt recovery called the model")
		return nil, nil
	})
	work, _ := reloaded.WorkItem(scope, root.WorkID)
	if _, err = restarted.ControlWork(scope, api.WorkCommand{RequestID: "resume-review", TargetID: work.ID, ExpectedRevision: work.Revision, Kind: "continue"}); err != nil {
		t.Fatal(err)
	}
	waiting, err := restarted.Continue(context.Background(), root.ID, scope.Actor, "start", "", false)
	if err != nil || waiting.Status != TaskChildren || reviewingWorkerQuestion(waiting) || waiting.Checkpoint.BudgetedModelCalls != parent.Checkpoint.BudgetedModelCalls {
		t.Fatal(waiting, err)
	}
	page, err := reloaded.WorkerMessages(scope, child.Worker.SessionID, 0, 10)
	if err != nil || len(page.Items) != 1 {
		t.Fatal("answer replay duplicated mailbox", page, err)
	}
}

func TestWorkerQuestionReviewCorrectionInvalidatesQuestion(t *testing.T) {
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
	_, err := r.ControlWork(scope, api.WorkCommand{RequestID: "correct-review", TargetID: work.ID, ExpectedRevision: work.Revision, Kind: "correct", Payload: api.WorkCommandPayload{Instruction: "Only compare hosted runtimes."}})
	close(release)
	if err != nil {
		t.Fatal(err)
	}
	settleTask(t, r, root.ID)
	r.advanceDelegation(context.Background(), root.ID)
	updated, _ := m.GetTask(root.ID)
	obsolete, _ := m.GetTask(child.ID)
	if updated.Delegation != nil || updated.Status != TaskPending || obsolete.Status != TaskCancelled || obsolete.ErrorCode != "instruction_superseded" || obsolete.WorkerQuestion.State != "pending" {
		t.Fatal("correction retained live obsolete question", updated.Status, obsolete.Status)
	}
	if err = NewManagerWithPersistence(nil, nil, nil, m.dataDir).StorageError(); err != nil {
		t.Fatal(err)
	}
}

func TestWorkerQuestionReviewUpgradeFrom22RetainsQuestions(t *testing.T) {
	m, _, scope, _, child := waitingWorkerQuestion(t)
	db, err := m.controlDatabase()
	if err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec(`UPDATE agent_schema SET version=22`); err != nil {
		t.Fatal(err)
	}
	db.Close()
	reloaded := NewManagerWithPersistence(nil, nil, nil, m.dataDir)
	if err = reloaded.StorageError(); err != nil {
		t.Fatal(err)
	}
	worker, err := reloaded.WorkerSession(scope, child.Worker.SessionID)
	if err != nil || worker.Question.ID != child.WorkerQuestion.ID {
		t.Fatal(worker, err)
	}
	db, err = reloaded.controlDatabase()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	var raw []byte
	if err = db.QueryRow(`SELECT manifest FROM agent_migrations WHERE version=?`, taskSchemaVersion).Scan(&raw); err != nil {
		t.Fatal(err)
	}
	var manifest struct {
		From   int    `json:"from"`
		To     int    `json:"to"`
		Backup string `json:"backup"`
	}
	if err = json.Unmarshal(raw, &manifest); err != nil || manifest.From != 22 || manifest.To != taskSchemaVersion || manifest.Backup == "" {
		t.Fatal(manifest, err)
	}
}
