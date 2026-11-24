package agents

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/takuphilchan/offgrid-llm/pkg/api"
)

func TestModelProgressPreemptionRetainsCanonicalDebt(t *testing.T) {
	for _, boundary := range []string{"inbox", "steering"} {
		t.Run(boundary, func(t *testing.T) {
			m := NewManagerWithPersistence(nil, nil, nil, t.TempDir())
			r := NewRunner(m, &runTestTools{}, nil)
			task, err := r.Create("Preserve the accepted request", "primary", "alice", switchFixtureConfig())
			if err != nil {
				t.Fatal(err)
			}
			task, err = m.updateTask(task.ID, func(next *Task) error {
				next.Status = TaskRunning
				// Local control cleanup has already cleared the local counter.
				// Only accepted progress can retire the retained aggregate charge.
				next.Checkpoint.ModelBudget = &ModelGraphBudget{Version: 1, Repairs: map[string]int{next.ID: 2}}
				return nil
			})
			if err != nil {
				t.Fatal(err)
			}
			_, err = m.updateTask(task.ID, func(next *Task) error {
				if boundary == "inbox" {
					return holdForConversationInput(next, "input-new")
				}
				next.PendingInstructions = []Instruction{{Text: "Use the revised request"}}
				return nil
			})
			if err != nil {
				t.Fatal(err)
			}
			finishTask(task, TaskCompleted, "")
			task.Result = "Superseded answer"
			err = r.persistModelProgress(task)
			want := errSteeringPending
			if boundary == "inbox" {
				want = errConversationInputPending
			}
			if !errors.Is(err, want) {
				t.Fatalf("preemption lost: %v", err)
			}
			saved, _ := m.GetTask(task.ID)
			if saved.Status == TaskCompleted || saved.Result != "" || saved.Checkpoint.ModelBudget.Repairs[task.ID] != 2 {
				t.Fatal("superseded result refunded canonical debt", saved.Status, saved.Checkpoint.ModelBudget)
			}
		})
	}
}

func TestModelProgressChildResolutionRollsBackWithRoot(t *testing.T) {
	m, r, root, children := modelGraphFixture(t)
	child, err := m.updateTask(children[0].ID, func(next *Task) error {
		next.Status, next.Checkpoint.DecisionRepairs = TaskRunning, 1
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	_, err = m.updateTask(root.ID, func(next *Task) error {
		next.Checkpoint.ModelBudget.Repairs = map[string]int{child.ID: 1, children[1].ID: 1}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	db, err := openTaskDatabase(m.dataDir)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err = db.Exec(`CREATE TRIGGER reject_progress BEFORE UPDATE ON agent_tasks WHEN NEW.id='` + root.ID + `' BEGIN SELECT RAISE(ABORT,'fixture progress rollback'); END`); err != nil {
		t.Fatal(err)
	}
	finishTask(child, TaskCompleted, "")
	child.Result = "Accepted worker findings"
	if err = r.persistModelProgress(child); err == nil {
		t.Fatal("failed resolution transaction accepted")
	}
	saved, _ := m.GetTask(child.ID)
	parent, _ := m.GetTask(root.ID)
	if saved.Status != TaskRunning || saved.Checkpoint.DecisionRepairs != 1 || parent.Checkpoint.ModelBudget.Repairs[child.ID] != 1 {
		t.Fatal("failed child result partially refunded debt")
	}
	if _, err = db.Exec(`DROP TRIGGER reject_progress`); err != nil {
		t.Fatal(err)
	}
	// Persistence failure deliberately latches the manager unavailable. Reopen
	// isolated state before retrying; do not clear that production fence.
	m = NewManagerWithPersistence(nil, nil, nil, m.dataDir)
	if err = m.StorageError(); err != nil {
		t.Fatal(err)
	}
	if _, err = m.updateTask(root.ID, func(next *Task) error { next.Status = TaskChildren; return nil }); err != nil {
		t.Fatal(err)
	}
	if _, err = m.updateTask(child.ID, func(next *Task) error { next.Status = TaskRunning; return nil }); err != nil {
		t.Fatal(err)
	}
	r = NewRunner(m, NewToolRegistry(), nil)
	if err = r.persistModelProgress(child); err != nil {
		t.Fatal(err)
	}
	parent, _ = m.GetTask(root.ID)
	if parent.Checkpoint.ModelBudget.Repairs[child.ID] != 0 || parent.Checkpoint.ModelBudget.Repairs[children[1].ID] != 1 {
		t.Fatal("accepted child result changed sibling debt", parent.Checkpoint.ModelBudget)
	}
	loaded := NewManagerWithPersistence(nil, nil, nil, m.dataDir)
	if err = loaded.StorageError(); err != nil {
		t.Fatal(err)
	}
	parent, _ = loaded.GetTask(root.ID)
	if parent.Checkpoint.ModelBudget.Repairs[child.ID] != 0 || parent.Checkpoint.ModelBudget.Repairs[children[1].ID] != 1 {
		t.Fatal("restart lost atomic resolution")
	}
}

func TestModelProgressNewDelegationFailureDoesNotResolveDebt(t *testing.T) {
	m := NewManagerWithPersistence(nil, nil, nil, t.TempDir())
	calls := 0
	r := NewRunner(m, NewToolRegistry(), func(context.Context, *Task, []api.ChatMessage, []api.Tool) (*api.ChatCompletionResponse, error) {
		calls++
		if calls == 1 {
			return nil, rejectedDecision()
		}
		return taskCall("delegate_tasks", `{"tasks":[{"key":"one","goal":"Review supplied facts","depends_on":[],"tools":[]}]}`), nil
	})
	config := switchFixtureConfig()
	config.DisableDelegation, config.MaxIterations = false, 30
	task, err := r.Create("Review independent findings", "primary", "alice", config)
	if err != nil {
		t.Fatal(err)
	}
	db, err := openTaskDatabase(m.dataDir)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err = db.Exec(`CREATE TRIGGER reject_new_worker BEFORE INSERT ON agent_tasks WHEN NEW.id!='` + task.ID + `' BEGIN SELECT RAISE(ABORT,'fixture rejected child persistence'); END`); err != nil {
		t.Fatal(err)
	}
	_, err = r.Continue(context.Background(), task.ID, task.Actor, "start", "", false)
	if err == nil {
		t.Fatal("failed delegation commit accepted")
	}
	saved, _ := m.GetTask(task.ID)
	if len(m.ListTasks()) != 1 || len(saved.ChildHistory) != 0 || saved.Checkpoint.DecisionRepairs != 1 || saved.Checkpoint.ModelBudget.Repairs[task.ID] != 1 || saved.Checkpoint.ModelBudget.Switches != 1 {
		t.Fatal("failed delegation publication refunded prior rejection", saved.Checkpoint.ModelBudget)
	}
}

func TestModelProgressContinuesHistoricalWorkerFromCurrentAttempt(t *testing.T) {
	m, r, root, children := modelGraphFixture(t)
	defer r.Shutdown(context.Background())
	scope := ControlScope{root.Origin.Workspace, root.Actor}
	r.caller = func(context.Context, *Task, []api.ChatMessage, []api.Tool) (*api.ChatCompletionResponse, error) {
		return runAnswer(api.ChatMessage{Role: "assistant", Content: "Reviewed supplied findings"}), nil
	}
	for _, child := range children {
		if _, err := r.Continue(context.Background(), child.ID, root.Actor, "start", "", false); err != nil {
			t.Fatal(err)
		}
	}
	r.advanceDelegation(context.Background(), root.ID)
	root = awaitTask(t, m, root.ID, TaskCompleted)
	settleTask(t, r, root.ID)
	first, err := m.WorkerSession(scope, children[0].Worker.SessionID)
	if err != nil {
		t.Fatal(err)
	}
	followup, err := r.CommandWorker(scope, api.WorkerCommand{RequestID: "new-attempt", TargetID: first.ID, ExpectedRevision: first.Revision, Kind: "continue", Payload: api.WorkerCommandPayload{Content: "Expand the first review"}})
	if err != nil {
		t.Fatal(err)
	}
	currentID := followup.Worker.RootRunID
	if currentID == root.ID {
		t.Fatal("fixture did not create a new work attempt")
	}
	if _, err = r.Continue(context.Background(), followup.Worker.CurrentRunID, root.Actor, "start", "", false); err != nil {
		t.Fatal(err)
	}
	historical, err := m.WorkerSession(scope, children[1].Worker.SessionID)
	if err != nil {
		t.Fatal(err)
	}
	if historical.RootRunID != root.ID {
		t.Fatal("fixture lost historical worker")
	}
	_, err = m.updateTask(currentID, func(next *Task) error {
		next.Checkpoint.DecisionRepairs = 1
		next.Checkpoint.ModelBudget.Repairs = map[string]int{next.ID: 1}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	args, _ := json.Marshal(map[string]any{"worker_id": historical.ID, "expected_revision": historical.Revision, "content": "Expand the retained second review"})
	r.caller = func(context.Context, *Task, []api.ChatMessage, []api.Tool) (*api.ChatCompletionResponse, error) {
		return taskCall("worker_continue", string(args)), nil
	}
	r.advanceDelegation(context.Background(), currentID)
	settleTask(t, r, currentID)
	continued, err := m.WorkerSession(scope, historical.ID)
	if err != nil {
		t.Fatal(err)
	}
	current, _ := m.GetTask(currentID)
	if continued.RootRunID != currentID || continued.CurrentRunID == children[1].ID || current.Checkpoint.DecisionRepairs != 0 || current.Checkpoint.ModelBudget.Repairs[currentID] != 0 || current.Status != TaskChildren {
		t.Fatalf("model continuation did not use current root: current=%s worker_root=%s status=%s error=%s debt=%+v", currentID, continued.RootRunID, current.Status, current.Error, current.Checkpoint.ModelBudget)
	}
}
