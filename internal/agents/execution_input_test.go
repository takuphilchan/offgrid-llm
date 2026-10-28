package agents

import (
	"errors"
	"testing"

	"github.com/takuphilchan/offgrid-llm/internal/execution"
	"github.com/takuphilchan/offgrid-llm/pkg/api"
)

func waitingWorkspaceFixture(t *testing.T) (*Manager, *Runner, *Task, execution.Resource, string) {
	t.Helper()
	dir := t.TempDir()
	m := NewManagerWithPersistence(nil, nil, nil, dir)
	r := NewRunner(m, nil, nil)
	cfg := DefaultAgentConfig()
	cfg.TaskFirst = true
	cfg.ExecutionRequestVersion = 1
	ceiling := execution.Permissions{Read: true, Write: true}
	cfg.ExecutionPolicy = &ceiling
	task, err := r.Create("Read the selected folder", "model", "alice", cfg)
	if err != nil {
		t.Fatal(err)
	}
	task, err = m.updateTask(task.ID, func(next *Task) error {
		next.Status = TaskInput
		next.PendingInput = &InputRequest{ID: "input-workspace", CallID: "call-workspace", Kind: "workspace", Mode: "read", Target: "Local workspace"}
		next.Checkpoint.Calls = []api.ToolCall{{ID: "call-workspace", Type: "function", Function: api.FunctionCall{Name: execution.AccessRequestTool, Arguments: `{"access":"read"}`}}}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	resource := execution.Resource{Reference: execution.Reference{Host: "host", Session: "session", Grant: "grant", Revision: 1}, Backend: "local", Root: t.TempDir(), Directory: ".", Permissions: execution.Permissions{Read: true}}
	return m, r, task, resource, dir
}

func TestWorkspaceInputSurvivesRestartAndAttachesOnce(t *testing.T) {
	_, _, task, resource, dir := waitingWorkspaceFixture(t)
	m := NewManagerWithPersistence(nil, nil, nil, dir)
	r := NewRunner(m, nil, nil)
	resolved, err := r.ResolveExecutionInput(task.ID, task.Actor, task.PendingInput.ID, resource)
	if err != nil {
		t.Fatal(err)
	}
	if resolved.ID != task.ID || resolved.Status != TaskPending || resolved.PendingInput != nil || resolved.Config.Execution == nil || *resolved.Config.Execution != resource.Reference || len(resolved.Checkpoint.Calls) != 0 || len(resolved.Steps) != 1 {
		t.Fatal("incorrect attachment", resolved)
	}
	if _, allowed := resolved.Config.AllowedCapabilities["workspace_write"]; allowed {
		t.Fatal("read request gained writes")
	}
	again, err := r.ResolveExecutionInput(task.ID, task.Actor, task.PendingInput.ID, resource)
	if err != nil || len(again.Steps) != 1 || len(again.Checkpoint.Messages) != len(resolved.Checkpoint.Messages) {
		t.Fatal("duplicate attachment", err)
	}
}

func TestWorkspaceInputRejectsChangedScopeAndUnsafeBoundary(t *testing.T) {
	for _, kind := range []string{"actor", "input", "permission", "cancelled", "executing", "attached", "legacy", "child", "descriptor"} {
		t.Run(kind, func(t *testing.T) {
			m, r, task, resource, _ := waitingWorkspaceFixture(t)
			actor, input := task.Actor, task.PendingInput.ID
			if kind == "actor" {
				actor = "bob"
			}
			if kind == "input" {
				input = "other"
			}
			if kind == "permission" {
				resource.Permissions.Processes = true
			}
			_, err := m.updateTask(task.ID, func(next *Task) error {
				switch kind {
				case "cancelled":
					next.Status = TaskCancelled
				case "executing":
					next.Checkpoint.ExecutingCall = "old-mutation"
				case "attached":
					ref := resource.Reference
					next.Config.Execution = &ref
				case "legacy":
					next.Config.ExecutionRequestVersion = 0
				case "child":
					next.ParentID = "parent"
				case "descriptor":
					next.Checkpoint.Calls[0].Function.Name = "foreign_tool"
				}
				return nil
			})
			if err != nil {
				// Invalid checkpoints must be rejected before they can become
				// recoverable jobs. Check the durable state, not just the error.
				switch kind {
				case "executing", "attached", "legacy", "child", "descriptor":
					saved, found := m.GetTask(task.ID)
					if !found || saved.Config.Execution != nil || saved.Config.ExecutionRequestVersion != 1 || saved.ParentID != "" || saved.Checkpoint.ExecutingCall != "" || saved.Checkpoint.Calls[0].Function.Name != execution.AccessRequestTool || len(saved.Steps) != 0 {
						t.Fatal("rejected checkpoint changed durable state")
					}
					return
				default:
					t.Fatal(err)
				}
			}
			_, err = r.ResolveExecutionInput(task.ID, actor, input, resource)
			if !errors.Is(err, ErrRunConflict) && !errors.Is(err, ErrTaskNotFound) {
				t.Fatal("accepted invalid boundary", err)
			}
			saved, _ := m.GetTask(task.ID)
			if len(saved.Steps) != 0 || saved.PendingInput == nil {
				t.Fatal("rejection altered saved input")
			}
		})
	}
}

func TestWorkspacePermissionUpgradeKeepsRootAndCeilings(t *testing.T) {
	for _, kind := range []string{"valid", "folder", "host", "backend", "ceiling"} {
		t.Run(kind, func(t *testing.T) {
			m, r, task, resource, _ := waitingWorkspaceFixture(t)
			old := resource
			_, err := m.updateTask(task.ID, func(next *Task) error {
				next.Config.Execution = &old.Reference
				next.Config.ExecutionWorkspace = &old
				next.PendingInput.Mode = "edit"
				next.Checkpoint.Calls[0].Function.Arguments = `{"access":"edit"}`
				return nil
			})
			if err != nil {
				t.Fatal(err)
			}
			resource.Reference.Session = "replacement-session"
			resource.Reference.Grant = "replacement-grant"
			resource.Permissions.Write = true
			switch kind {
			case "folder":
				resource.Root = t.TempDir()
			case "host":
				resource.Reference.Host = "other-host"
			case "backend":
				resource.Backend = "isolated_linux"
			case "ceiling":
				resource.Permissions.Processes = true
			}
			resolved, err := r.ResolveExecutionInput(task.ID, task.Actor, task.PendingInput.ID, resource)
			if kind == "valid" {
				if err != nil || resolved.Config.ExecutionWorkspace.Permissions != resource.Permissions || resolved.ID != task.ID {
					t.Fatal(resolved, err)
				}
				prior, ok := executionReferenceAtStep(resolved, len(resolved.Steps)-2)
				if !ok || prior != old.Reference {
					t.Fatal("upgrade lost prior receipt identity", prior, ok)
				}
			} else {
				if !errors.Is(err, ErrRunConflict) {
					t.Fatal("invalid upgrade accepted", err)
				}
				saved, _ := m.GetTask(task.ID)
				if saved.Config.ExecutionWorkspace.Permissions.Write || saved.PendingInput == nil {
					t.Fatal("rejection changed authority")
				}
			}
		})
	}
}
