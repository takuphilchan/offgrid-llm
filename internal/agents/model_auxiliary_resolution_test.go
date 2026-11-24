package agents

import (
	"context"
	"testing"

	"github.com/takuphilchan/offgrid-llm/internal/execution"
	"github.com/takuphilchan/offgrid-llm/pkg/api"
)

func auxiliaryFixture(t *testing.T) (*Manager, *Runner, *Task) {
	t.Helper()
	m := NewManagerWithPersistence(nil, nil, nil, t.TempDir())
	r := NewRunner(m, &runTestTools{}, nil)
	task, err := r.Create("Preserve all requested work", "primary", "alice", switchFixtureConfig())
	if err != nil {
		t.Fatal(err)
	}
	task, err = m.updateTask(task.ID, func(next *Task) error { next.Status = TaskRunning; return nil })
	if err != nil {
		t.Fatal(err)
	}
	return m, r, task
}

func auxiliaryCount(t *testing.T, m *Manager, id string) int {
	t.Helper()
	task, _ := m.GetTask(id)
	if task.Checkpoint.ModelBudget == nil {
		return 0
	}
	count, err := task.Checkpoint.ModelBudget.corrections()
	if err != nil {
		t.Fatal(err)
	}
	return count
}

func auxiliaryRuntimeCall(t *testing.T, r *Runner, task *Task, name, args string) {
	t.Helper()
	call := api.ToolCall{ID: "fixture", Type: "function", Function: api.FunctionCall{Name: name, Arguments: args}}
	task.Checkpoint.Calls = []api.ToolCall{call}
	if handled, err := r.executeRuntimeTool(context.Background(), task, call); !handled || err != nil {
		t.Fatal(handled, err)
	}
}

func TestModelAuxiliaryMetadataResolutionIsMatchingAndOperationalFailuresDoNotCharge(t *testing.T) {
	m, r, task := auxiliaryFixture(t)
	chargeAuxiliaryModelRepair(task, "workflow_completion")
	if err := r.persist(task); err != nil {
		t.Fatal(err)
	}
	auxiliaryRuntimeCall(t, r, task, "task_plan", `{"items":[]}`)
	if count := auxiliaryCount(t, m, task.ID); count != 2 {
		t.Fatal(count)
	}
	// Missing saved evidence is not proof of a rejected model decision.
	auxiliaryRuntimeCall(t, r, task, "task_history", `{"reference":"result:missing:unavailable","offset":0}`)
	if count := auxiliaryCount(t, m, task.ID); count != 2 {
		t.Fatal("operational evidence failure charged", count)
	}
	auxiliaryRuntimeCall(t, r, task, "task_plan", `{"items":[{"id":"one","title":"Review","state":"active","evidence_steps":[]}]}`)
	if count := auxiliaryCount(t, m, task.ID); count != 1 {
		t.Fatal("metadata refunded unrelated workflow debt", count)
	}
	if task.Checkpoint.AuxiliaryModelRepairs.Active["workflow_completion"] != 1 {
		t.Fatal("lost workflow debt")
	}
}

func TestModelAuxiliaryPureArgumentRejectionExcludesDiscoveryCancellation(t *testing.T) {
	m, r, task := auxiliaryFixture(t)
	auxiliaryRuntimeCall(t, r, task, "task_find_tools", `{"query":null,"offset":0}`)
	if count := auxiliaryCount(t, m, task.ID); count != 1 {
		t.Fatal("invalid discovery query not charged", count)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	call := api.ToolCall{ID: "cancelled-discovery", Type: "function", Function: api.FunctionCall{Name: "task_find_tools", Arguments: `{"query":"","offset":0}`}}
	task.Checkpoint.Calls = []api.ToolCall{call}
	if handled, err := r.executeRuntimeTool(ctx, task, call); !handled || err != nil {
		t.Fatal(handled, err)
	}
	if count := auxiliaryCount(t, m, task.ID); count != 1 {
		t.Fatal("cancelled discovery charged or resolved", count)
	}
	auxiliaryRuntimeCall(t, r, task, "task_find_tools", `{"query":"","offset":0}`)
	if count := auxiliaryCount(t, m, task.ID); count != 0 {
		t.Fatal("matching discovery failed to resolve", count)
	}
	auxiliaryRuntimeCall(t, r, task, "worker_request_budget", `{"model_calls":0,"reason":"invalid range"}`)
	if count := auxiliaryCount(t, m, task.ID); count != 1 {
		t.Fatal("pure budget argument rejection not charged", count)
	}
	auxiliaryRuntimeCall(t, r, task, "worker_request_budget", `{"model_calls":1,"reason":"valid arguments but unavailable worker authority"}`)
	if count := auxiliaryCount(t, m, task.ID); count != 1 {
		t.Fatal("authority/liveness conflict charged or resolved", count)
	}
}

func TestModelAuxiliaryMatchingAnswerResolutionRetainsDecisionAndSiblingDebt(t *testing.T) {
	m, r, root, children := modelGraphFixture(t)
	for i := range children {
		var err error
		children[i], err = m.updateTask(children[i].ID, func(next *Task) error { next.Status = TaskRunning; attachContractFixture(next); return nil })
		if err != nil {
			t.Fatal(err)
		}
		if _, err = r.repairSkillAnswer(children[i], "incomplete"); err != nil {
			t.Fatal(err)
		}
	}
	child := children[0]
	child.Checkpoint.DecisionRepairs = 1
	if err := r.persist(child); err != nil {
		t.Fatal(err)
	}
	if count := auxiliaryCount(t, m, root.ID); count != 3 {
		t.Fatal(count)
	}
	if repaired, err := r.repairSkillAnswer(child, "DONE\nNext: café 日本語"); repaired || err != nil {
		t.Fatal(repaired, err)
	}
	// A matching answer-format check can pass even if a later answer validator
	// rejects it. Publish that resolution without generic decision acceptance.
	if err := r.persist(child); err != nil {
		t.Fatal(err)
	}
	if count := auxiliaryCount(t, m, root.ID); count != 2 {
		t.Fatal("matching category did not resolve exactly once", count)
	}
	if child.Checkpoint.SkillAnswerRepairs != 1 || child.Checkpoint.DecisionRepairs != 1 {
		t.Fatal("historical or unrelated debt changed")
	}
	finishTask(child, TaskCompleted, "")
	if err := r.persistModelProgress(child); err != nil {
		t.Fatal(err)
	}
	if count := auxiliaryCount(t, m, root.ID); count != 1 {
		t.Fatal("final result refunded sibling", count)
	}
	reloaded := NewManagerWithPersistence(nil, nil, nil, m.dataDir)
	if err := reloaded.StorageError(); err != nil {
		t.Fatal(err)
	}
	if count := auxiliaryCount(t, reloaded, root.ID); count != 1 {
		t.Fatal("restart refunded sibling", count)
	}
}

func TestModelAuxiliaryWorkflowNeedsNewVerifiedProgress(t *testing.T) {
	m, r, task := auxiliaryFixture(t)
	tools := &assessedTestTools{states: map[string]string{"first": "pending", "second": "pending"}}
	r.tools = tools
	task.Config = workflowConfig()
	if err := initializeWorkflow(task); err != nil {
		t.Fatal(err)
	}
	if err := declareExpectations(task, []OutcomeExpectation{{ID: "first", Title: "First", Kind: "application_open", ApplicationID: "one"}, {ID: "second", Title: "Second", Kind: "application_open", ApplicationID: "two"}}, "model_proposed"); err != nil {
		t.Fatal(err)
	}
	chargeAuxiliaryModelRepair(task, "workflow_completion")
	chargeAuxiliaryModelRepair(task, "metadata:task_plan")
	if err := r.persist(task); err != nil {
		t.Fatal(err)
	}
	task.Checkpoint.CompletionRepairs = 0 // Historical reset/steering is not proof.
	r.assessWorkflow(task)
	if err := r.persistModelProgress(task); err != nil {
		t.Fatal(err)
	}
	if count := auxiliaryCount(t, m, task.ID); count != 2 {
		t.Fatal("unverified reset refunded debt", count)
	}
	tools.states["first"] = "verified"
	r.assessWorkflow(task)
	if err := r.persist(task); err != nil {
		t.Fatal(err)
	}
	if count := auxiliaryCount(t, m, task.ID); count != 1 {
		t.Fatal("new typed workflow progress failed to resolve", count)
	}
	chargeAuxiliaryModelRepair(task, "workflow_completion")
	if err := r.persist(task); err != nil {
		t.Fatal(err)
	}
	tools.states["first"] = "pending"
	r.assessWorkflow(task)
	tools.states["first"] = "verified"
	r.assessWorkflow(task)
	if err := r.persistModelProgress(task); err != nil {
		t.Fatal(err)
	}
	if count := auxiliaryCount(t, m, task.ID); count != 2 {
		t.Fatal("same proof refunded new debt", count)
	}
}

type auxiliaryCompletionTools struct {
	runTestTools
	pending bool
}

func (t *auxiliaryCompletionTools) ValidateExecutionCompletion(*Task) error {
	if t.pending {
		return &ExecutionPending{Handle: "owned"}
	}
	return nil
}

func TestModelAuxiliaryExecutionResolutionUsesTypedCompletedEvidence(t *testing.T) {
	m, r, task := auxiliaryFixture(t)
	tools := &auxiliaryCompletionTools{pending: true}
	r.tools = tools
	chargeAuxiliaryModelRepair(task, "workspace_completion")
	if err := r.persist(task); err != nil {
		t.Fatal(err)
	}
	wait, _ := execution.Descriptor("workspace_process_wait")
	r.resolveAuxiliaryExecutionProgress(task, wait)
	if err := r.persistModelProgress(task); err != nil {
		t.Fatal(err)
	}
	if count := auxiliaryCount(t, m, task.ID); count != 1 {
		t.Fatal("still-running observation refunded completion", count)
	}
	tools.pending = false
	foreign := wait
	foreign.Namespace = "mcp"
	r.resolveAuxiliaryExecutionProgress(task, foreign)
	if task.Checkpoint.AuxiliaryModelRepairs.Active["workspace_completion"] != 1 {
		t.Fatal("foreign evidence resolved debt")
	}
	r.resolveAuxiliaryExecutionProgress(task, wait)
	if err := r.persistModelProgress(task); err != nil {
		t.Fatal(err)
	}
	if count := auxiliaryCount(t, m, task.ID); count != 0 {
		t.Fatal("typed completed evidence did not resolve", count)
	}
}

func TestModelAuxiliaryWorkspaceReadPreservesLifetimeAllowance(t *testing.T) {
	m, r, task := auxiliaryFixture(t)
	descriptor, _ := execution.Descriptor("workspace_read")
	if handled, err := r.repairWorkspaceReadFailure(task, descriptor, &ToolFailure{Code: "execution_rejected", Recovery: "Inspect the selected path"}); !handled || err != nil {
		t.Fatal(handled, err)
	}
	if count := auxiliaryCount(t, m, task.ID); count != 1 {
		t.Fatal(count)
	}
	other, _ := execution.Descriptor("workspace_list")
	r.resolveAuxiliaryExecutionProgress(task, other)
	if err := r.persistModelProgress(task); err != nil {
		t.Fatal(err)
	}
	if count := auxiliaryCount(t, m, task.ID); count != 1 {
		t.Fatal("different read refunded debt", count)
	}
	r.resolveAuxiliaryExecutionProgress(task, descriptor)
	if err := r.persistModelProgress(task); err != nil {
		t.Fatal(err)
	}
	if count := auxiliaryCount(t, m, task.ID); count != 0 || task.Checkpoint.WorkspaceReadFailures != 1 {
		t.Fatal("matching read altered historical allowance", count)
	}
}
