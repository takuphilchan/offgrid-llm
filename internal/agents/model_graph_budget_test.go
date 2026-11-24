package agents

import (
	"context"
	"sync"
	"testing"

	"github.com/takuphilchan/offgrid-llm/internal/inference"
	"github.com/takuphilchan/offgrid-llm/pkg/api"
)

func modelGraphFixture(t *testing.T) (*Manager, *Runner, *Task, []*Task) {
	t.Helper()
	m, r, scope, conversation := controlFixture(t)
	config := switchFixtureConfig()
	config.DisableDelegation, config.MaxIterations = false, 30
	receipt, err := r.AdmitConversationWork(scope, ConversationCommand{RequestID: "model-graph", TargetID: conversation.ID, ExpectedRevision: conversation.Revision, Kind: "create_work", Payload: ConversationCommandPayload{Input: &ConversationInput{Content: "Compare independent findings"}}}, "primary", config)
	if err != nil {
		t.Fatal(err)
	}
	r.caller = func(context.Context, *Task, []api.ChatMessage, []api.Tool) (*api.ChatCompletionResponse, error) {
		return taskCall("delegate_tasks", `{"tasks":[{"key":"one","goal":"Review one","depends_on":[],"tools":[]},{"key":"two","goal":"Review two","depends_on":[],"tools":[]}]}`), nil
	}
	root, err := r.Continue(context.Background(), receipt.Work.RunID, scope.Actor, "start", "", false)
	if err != nil || root.Status != TaskChildren {
		t.Fatalf("admission: %+v %v", root, err)
	}
	var children []*Task
	for _, link := range root.ChildHistory {
		child, _ := m.GetTask(link.ID)
		children = append(children, child)
	}
	return m, r, root, children
}

func TestModelGraphConcurrentRejectionsShareSwitchesAndSurviveRestart(t *testing.T) {
	m, r, root, children := modelGraphFixture(t)
	for i, child := range children {
		var err error
		children[i], err = m.updateTask(child.ID, func(task *Task) error { task.Status = TaskRunning; return nil })
		if err != nil {
			t.Fatal(err)
		}
	}
	start := make(chan struct{})
	results := make(chan error, 2)
	var wg sync.WaitGroup
	for _, child := range children {
		wg.Add(1)
		go func(child *Task) {
			defer wg.Done()
			<-start
			results <- r.rejectModelDecision(child, rejectedDecision().(*inference.DecisionValidationError))
		}(child)
	}
	close(start)
	wg.Wait()
	close(results)
	for err := range results {
		if err != nil {
			t.Fatal(err)
		}
	}
	saved, _ := m.GetTask(root.ID)
	if saved.Checkpoint.ModelBudget.Switches != 2 {
		t.Fatal("lost concurrent switch charge", saved.Checkpoint.ModelBudget)
	}
	for _, child := range children {
		if child.Model != "fallback-one" || len(child.Checkpoint.ModelSwitches) != 1 {
			t.Fatal("child did not switch", child.Model)
		}
	}
	if err := r.rejectModelDecision(children[0], rejectedDecision().(*inference.DecisionValidationError)); err != nil {
		t.Fatal(err)
	}
	if children[0].Status != TaskInterrupted || len(children[0].Checkpoint.ModelSwitches) != 1 {
		t.Fatal("third rejection exceeded shared limit")
	}
	// A stale root snapshot must observe canonical sibling accounting.
	if stopped, err := r.modelGraphBoundary(root); !stopped || err == nil { // root is waiting, so publication must not bypass that fence.
		t.Fatal("stale root ignored shared limit", stopped, err)
	}
	reloaded := NewManagerWithPersistence(nil, nil, nil, m.dataDir)
	if err := reloaded.StorageError(); err != nil {
		t.Fatal(err)
	}
	stored, _ := reloaded.GetTask(root.ID)
	count, err := stored.Checkpoint.ModelBudget.corrections()
	if err != nil || count != 3 || stored.Checkpoint.ModelBudget.Switches != 2 {
		t.Fatal("restart replenished budget", count, err)
	}
}

func TestModelGraphChildSwitchExecutesThroughRunner(t *testing.T) {
	m, r, root, children := modelGraphFixture(t)
	calls := 0
	r.caller = func(_ context.Context, task *Task, _ []api.ChatMessage, _ []api.Tool) (*api.ChatCompletionResponse, error) {
		calls++
		if calls == 1 {
			return nil, rejectedDecision()
		}
		saved, _ := m.GetTask(root.ID)
		if task.Model != "fallback-one" || saved.Checkpoint.ModelBudget.Switches != 1 || saved.Checkpoint.ModelBudget.Repairs[task.ID] != 1 {
			t.Fatal("generation preceded atomic charge")
		}
		return runAnswer(api.ChatMessage{Role: "assistant", Content: "Reviewed the supplied facts"}), nil
	}
	child, err := r.Continue(context.Background(), children[0].ID, root.Actor, "start", "", false)
	if err != nil || child.Status != TaskCompleted || calls != 2 {
		t.Fatalf("worker switch: %+v %v", child, err)
	}
	parent, _ := m.GetTask(root.ID)
	if len(parent.Checkpoint.ModelBudget.Repairs) != 0 || parent.Checkpoint.ModelBudget.Switches != 1 {
		t.Fatal("success refunded switch or kept resolved rejection")
	}
}

func TestModelGraphParentWriteFailureRollsBackChildSwitch(t *testing.T) {
	m, r, root, children := modelGraphFixture(t)
	child, err := m.updateTask(children[0].ID, func(task *Task) error { task.Status = TaskRunning; return nil })
	if err != nil {
		t.Fatal(err)
	}
	db, err := openTaskDatabase(m.dataDir)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err = db.Exec(`CREATE TRIGGER reject_graph_budget BEFORE UPDATE ON agent_tasks WHEN NEW.id='` + root.ID + `' BEGIN SELECT RAISE(ABORT,'fixture graph write failure'); END`); err != nil {
		t.Fatal(err)
	}
	if err = r.rejectModelDecision(child, rejectedDecision().(*inference.DecisionValidationError)); err == nil {
		t.Fatal("failed graph write accepted")
	}
	saved, _ := m.GetTask(child.ID)
	parent, _ := m.GetTask(root.ID)
	if saved.Model != "primary" || len(saved.Checkpoint.ModelSwitches) != 0 || parent.Checkpoint.ModelBudget.Switches != 0 {
		t.Fatal("partial graph publication")
	}
}

func TestModelGraphSiblingSuccessCannotEraseOtherRepair(t *testing.T) {
	m, r, root, children := modelGraphFixture(t)
	for i, child := range children {
		var err error
		children[i], err = m.updateTask(child.ID, func(task *Task) error { task.Status = TaskRunning; return nil })
		if err != nil {
			t.Fatal(err)
		}
		if err = r.rejectModelDecision(children[i], rejectedDecision().(*inference.DecisionValidationError)); err != nil {
			t.Fatal(err)
		}
	}
	children[0].Checkpoint.DecisionRepairs = 0
	children[0].Checkpoint.Iteration++ // Merely queued output is not acceptance.
	if err := r.persist(children[0]); err != nil {
		t.Fatal(err)
	}
	unchanged, _ := m.GetTask(root.ID)
	if count, _ := unchanged.Checkpoint.ModelBudget.corrections(); count != 2 {
		t.Fatal("queued output refunded unresolved corrections")
	}
	if err := r.persistModelProgress(children[0]); err != nil {
		t.Fatal(err)
	}
	parent, _ := m.GetTask(root.ID)
	budget := parent.Checkpoint.ModelBudget
	count, err := budget.corrections()
	if err != nil || count != 1 || budget.Repairs[children[1].ID] != 1 || budget.Switches != 2 {
		t.Fatal("unrelated success cleared repair or switch charge", budget, err)
	}
}

func TestModelGraphFollowupRetainsSpentSwitches(t *testing.T) {
	m, r, scope, root, child := completedWorkerWork(t)
	_, err := m.updateTask(root.ID, func(task *Task) error {
		task.Checkpoint.ModelBudget = &ModelGraphBudget{Version: 1, Switches: 2, Repairs: map[string]int{child.ID: 1}}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	worker, _ := m.WorkerSession(scope, child.Worker.SessionID)
	receipt, err := r.CommandWorker(scope, api.WorkerCommand{RequestID: "retain-model-allowance", TargetID: worker.ID, ExpectedRevision: worker.Revision, Kind: "continue", Payload: api.WorkerCommandPayload{Content: "Expand findings"}})
	if err != nil {
		t.Fatal(err)
	}
	next, _ := m.GetTask(receipt.Worker.RootRunID)
	assignment, _ := m.GetTask(receipt.Worker.CurrentRunID)
	if next.ID == root.ID || next.Checkpoint.ModelBudget == nil || next.Checkpoint.ModelBudget.Switches != 2 || next.Checkpoint.ModelBudget.Repairs[assignment.ID] != 1 || next.Checkpoint.ModelBudget.Repairs[child.ID] != 0 || assignment.Checkpoint.DecisionRepairs != 1 {
		t.Fatal("followup replenished shared allowance")
	}
	reloaded := NewManagerWithPersistence(nil, nil, nil, m.dataDir)
	if err := reloaded.StorageError(); err != nil {
		t.Fatal(err)
	}
	saved, _ := reloaded.GetTask(next.ID)
	if saved.Checkpoint.ModelBudget.Switches != 2 {
		t.Fatal("restart lost inherited switches")
	}
}

func TestModelGraphDoesNotSwitchAcrossCanonicalReceiptOrSteering(t *testing.T) {
	for _, boundary := range []string{"receipt", "steering", "root_stop"} {
		t.Run(boundary, func(t *testing.T) {
			m, r, root, children := modelGraphFixture(t)
			child, err := m.updateTask(children[0].ID, func(task *Task) error {
				task.Status = TaskRunning
				if boundary == "receipt" {
					task.Checkpoint.ComputerReceipt = &ComputerReceipt{CallID: "retained"}
				}
				if boundary == "steering" {
					task.PendingInstructions = []Instruction{{Text: "Change the approach"}}
				}
				return nil
			})
			if err != nil {
				t.Fatal(err)
			}
			if boundary == "root_stop" {
				if _, err = m.updateTask(root.ID, func(task *Task) error { task.ControlAction = "stop"; return nil }); err != nil {
					t.Fatal(err)
				}
			}
			err = r.rejectModelDecision(child, rejectedDecision().(*inference.DecisionValidationError))
			if boundary == "root_stop" && err == nil {
				t.Fatal("stopped root admitted child correction")
			}
			saved, _ := m.GetTask(child.ID)
			parent, _ := m.GetTask(root.ID)
			if saved.Model != "primary" || len(saved.Checkpoint.ModelSwitches) != 0 || parent.Checkpoint.ModelBudget.Switches != 0 {
				t.Fatal("unsafe boundary spent fallback")
			}
		})
	}
}

func TestModelGraphExhaustionWinsInferenceReservationRace(t *testing.T) {
	m, r, root, children := modelGraphFixture(t)
	child, err := m.updateTask(children[0].ID, func(task *Task) error { task.Status = TaskRunning; return nil })
	if err != nil {
		t.Fatal(err)
	}
	if stopped, err := r.modelGraphBoundary(child); stopped || err != nil {
		t.Fatal(stopped, err)
	}
	_, err = m.updateTask(root.ID, func(task *Task) error {
		task.Checkpoint.ModelBudget.Repairs = map[string]int{children[1].ID: 3}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if err = reserveModelBudget(child); err != nil {
		t.Fatal(err)
	}
	if err = r.persist(child); err == nil {
		t.Fatal("stale admission reserved new inference")
	}
	saved, _ := m.GetTask(child.ID)
	if saved.Checkpoint.BudgetedModelCalls != 0 {
		t.Fatal("rejected reservation published")
	}
}
