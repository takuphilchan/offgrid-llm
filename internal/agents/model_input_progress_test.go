package agents

import (
	"testing"

	"github.com/takuphilchan/offgrid-llm/internal/execution"
)

func modelInputDebt(t *testing.T, m *Manager, id string) *Task {
	t.Helper()
	task, err := m.updateTask(id, func(next *Task) error {
		next.Checkpoint.DecisionRepairs = 2
		next.Checkpoint.ProposalRepairs, next.Checkpoint.ProposalUnresolved, next.Checkpoint.ProposalTool = 1, true, "workspace_write"
		next.Checkpoint.ModelBudget = &ModelGraphBudget{Version: 1, Repairs: map[string]int{id: 3}}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return task
}

func modelInputFixture(t *testing.T, kind string) (*Manager, *Runner, *Task, func(*Runner, bool) (*Task, error)) {
	t.Helper()
	if kind == "computer" {
		m, r, task, _, _ := waitingInputFixture(t)
		task = modelInputDebt(t, m, task.ID)
		config := task.Config
		config.ComputerSession, config.ComputerDriver, config.ComputerApprovalMode = "fixture-session", "windows-uia", "scoped_changes"
		return m, r, task, func(r *Runner, invalid bool) (*Task, error) {
			choice := config
			if invalid {
				choice.RequestedComputerScope = "desktop"
			}
			return r.ResolveComputerInput(task.ID, task.Actor, task.PendingInput.ID, choice)
		}
	}
	var m *Manager
	var r *Runner
	var task *Task
	var resource execution.Resource
	var project *ExecutionProjectContext
	if kind == "project" {
		m, r, _, _, task, resource = canonicalWorkspaceInputFixture(t)
		project = &ExecutionProjectContext{Snapshot: folderSnapshot(t, "fixture-model")}
	} else {
		m, r, task, resource, _ = waitingWorkspaceFixture(t)
	}
	task = modelInputDebt(t, m, task.ID)
	return m, r, task, func(r *Runner, invalid bool) (*Task, error) {
		choice := resource
		if invalid {
			choice.Permissions.Processes = true
		}
		return r.ResolveExecutionInputWithProject(task.ID, task.Actor, task.PendingInput.ID, choice, project)
	}
}

func TestModelInputProgressResolvesOnlyConsumedDecisionOnce(t *testing.T) {
	for _, kind := range []string{"computer", "workspace", "project"} {
		t.Run(kind, func(t *testing.T) {
			m, r, task, resolve := modelInputFixture(t, kind)
			if _, err := resolve(r, true); err == nil {
				t.Fatal("invalid binding accepted")
			}
			unchanged, _ := m.GetTask(task.ID)
			if unchanged.Checkpoint.DecisionRepairs != 2 || unchanged.Checkpoint.ModelBudget.Repairs[task.ID] != 3 || unchanged.PendingInput == nil {
				t.Fatal("invalid binding refunded debt")
			}
			resolved, err := resolve(r, false)
			if err != nil {
				t.Fatal(err)
			}
			cp := resolved.Checkpoint
			if cp.DecisionRepairs != 0 || cp.ProposalRepairs != 1 || !cp.ProposalUnresolved || cp.ProposalTool != "workspace_write" || cp.ModelBudget.Repairs[task.ID] != 1 || resolved.PendingInput != nil || len(cp.Calls) != 0 {
				t.Fatalf("access resolution changed wrong debt: decisions=%d proposals=%d unresolved=%v tool=%s budget=%+v", cp.DecisionRepairs, cp.ProposalRepairs, cp.ProposalUnresolved, cp.ProposalTool, cp.ModelBudget)
			}
			messages, steps := len(cp.Messages), len(resolved.Steps)
			if _, err = m.updateTask(task.ID, func(next *Task) error {
				next.Checkpoint.DecisionRepairs = 1
				next.Checkpoint.ModelBudget.Repairs[task.ID] = 2
				return nil
			}); err != nil {
				t.Fatal(err)
			}
			again, err := resolve(r, false)
			if err != nil || again.Checkpoint.DecisionRepairs != 1 || again.Checkpoint.ModelBudget.Repairs[task.ID] != 2 || len(again.Steps) != steps || len(again.Checkpoint.Messages) != messages {
				t.Fatal("duplicate resolution refunded later debt or duplicated input", err)
			}
		})
	}
}

func TestModelInputProgressRollsBackWithBinding(t *testing.T) {
	for _, kind := range []string{"computer", "workspace", "project"} {
		t.Run(kind, func(t *testing.T) {
			m, r, task, resolve := modelInputFixture(t, kind)
			db, err := openTaskDatabase(m.dataDir)
			if err != nil {
				t.Fatal(err)
			}
			defer db.Close()
			if _, err = db.Exec(`CREATE TRIGGER reject_access_progress BEFORE UPDATE ON agent_tasks WHEN NEW.id='` + task.ID + `' BEGIN SELECT RAISE(ABORT,'fixture access rollback'); END`); err != nil {
				t.Fatal(err)
			}
			if _, err = resolve(r, false); err == nil {
				t.Fatal("failed binding commit accepted")
			}
			saved, _ := m.GetTask(task.ID)
			if saved.Checkpoint.DecisionRepairs != 2 || saved.Checkpoint.ModelBudget.Repairs[task.ID] != 3 || saved.PendingInput == nil || saved.Config.Execution != nil || saved.Config.ComputerSession != "" {
				t.Fatal("failed commit partially published binding or debt")
			}
			if _, err = db.Exec(`DROP TRIGGER reject_access_progress`); err != nil {
				t.Fatal(err)
			}
			loaded := NewManagerWithPersistence(nil, nil, nil, m.dataDir)
			if err = loaded.StorageError(); err != nil {
				t.Fatal(err)
			}
			saved, _ = loaded.GetTask(task.ID)
			if saved.Checkpoint.ModelBudget.Repairs[task.ID] != 3 || saved.PendingInput == nil {
				t.Fatal("restart lost failed resolution debt")
			}
			if kind == "project" {
				scope := ControlScope{task.Origin.Workspace, task.Actor}
				c, err := loaded.Conversation(scope, task.Origin.Conversation)
				if err != nil || c.ProjectID != task.Config.ProjectID {
					t.Fatal("failed binding published project selection", err)
				}
			}
			accepted, err := resolve(NewRunner(loaded, NewToolRegistry(), nil), false)
			if err != nil || accepted.Checkpoint.DecisionRepairs != 0 || accepted.Checkpoint.ModelBudget.Repairs[task.ID] != 1 {
				t.Fatal("reopened valid binding did not resolve only decision debt", err)
			}
		})
	}
}

func TestModelInputInitialAttachmentAndOtherCallsCannotResolveDebt(t *testing.T) {
	for _, kind := range []string{"initial", "other_call"} {
		t.Run(kind, func(t *testing.T) {
			m, r, task, _, _ := waitingInputFixture(t)
			task = modelInputDebt(t, m, task.ID)
			var err error
			task, err = m.updateTask(task.ID, func(next *Task) error {
				if kind == "initial" {
					next.Config.RequestedComputerScope, next.PendingInput.Mode = "desktop", "desktop"
					next.Config.RequestedComputerApprovalMode = "scoped_changes"
					next.PendingInput.CallID, next.Checkpoint.Calls, next.Checkpoint.Iteration = "", nil, 0
				} else {
					next.Checkpoint.Calls[0].Function.Name = "other_call"
				}
				return nil
			})
			if err != nil {
				t.Fatal(err)
			}
			config := task.Config
			config.ComputerSession, config.ComputerDriver, config.ComputerApprovalMode = "fixture-session", "windows-uia", "scoped_changes"
			if kind == "initial" {
				config.ComputerScope = "desktop"
			}
			resolved, err := r.ResolveComputerInput(task.ID, task.Actor, task.PendingInput.ID, config)
			if err != nil {
				t.Fatal(err)
			}
			if resolved.Checkpoint.DecisionRepairs != 2 || resolved.Checkpoint.ProposalRepairs != 1 || resolved.Checkpoint.ModelBudget.Repairs[task.ID] != 3 {
				t.Fatal("non-model access progress refunded corrections", resolved.Checkpoint.ModelBudget)
			}
		})
	}
}

func TestModelInputMatchingComputerProposalResolvesWithAccess(t *testing.T) {
	m, r, task, _, _ := waitingInputFixture(t)
	task = modelInputDebt(t, m, task.ID)
	task, err := m.updateTask(task.ID, func(next *Task) error {
		next.Checkpoint.ProposalTool = "request_computer_access"
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	config := task.Config
	config.ComputerSession, config.ComputerDriver, config.ComputerApprovalMode = "fixture-session", "windows-uia", "scoped_changes"
	resolved, err := r.ResolveComputerInput(task.ID, task.Actor, task.PendingInput.ID, config)
	if err != nil {
		t.Fatal(err)
	}
	if cp := resolved.Checkpoint; cp.DecisionRepairs != 0 || cp.ProposalRepairs != 0 || cp.ProposalUnresolved || cp.ModelBudget.Repairs[task.ID] != 0 {
		t.Fatal("matching access proposal remained unresolved", cp.ModelBudget)
	}
}

func TestModelInputMatchingAccessPreservesStrongerRepair(t *testing.T) {
	for _, repair := range []string{"read", "workflow", "pre_effect"} {
		t.Run(repair, func(t *testing.T) {
			m, r, task, _, _ := waitingInputFixture(t)
			task = modelInputDebt(t, m, task.ID)
			task, err := m.updateTask(task.ID, func(next *Task) error {
				cp := next.Checkpoint
				cp.ProposalTool = "request_computer_access"
				switch repair {
				case "read":
					cp.NativeReadRepair = &NativeReadRepair{Tool: "computer_observe"}
				case "workflow":
					cp.WorkflowEditPending = true
				case "pre_effect":
					cp.PreEffectRepair, cp.PreEffectRepairTool = true, "workspace_write"
				}
				return nil
			})
			if err != nil {
				t.Fatal(err)
			}
			config := task.Config
			config.ComputerSession, config.ComputerDriver, config.ComputerApprovalMode = "fixture-session", "windows-uia", "scoped_changes"
			resolved, err := r.ResolveComputerInput(task.ID, task.Actor, task.PendingInput.ID, config)
			if err != nil {
				t.Fatal(err)
			}
			cp := resolved.Checkpoint
			if cp.DecisionRepairs != 0 || cp.ProposalRepairs != 1 || !cp.ProposalUnresolved || cp.ModelBudget.Repairs[task.ID] != 1 || repair == "read" && cp.NativeReadRepair == nil || repair == "workflow" && !cp.WorkflowEditPending || repair == "pre_effect" && (!cp.PreEffectRepair || cp.PreEffectRepairTool != "workspace_write") {
				t.Fatal("accepted access erased stronger repair", repair, cp.ModelBudget)
			}
		})
	}
}
