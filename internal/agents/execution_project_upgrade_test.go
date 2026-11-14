package agents

import (
	"testing"

	"github.com/takuphilchan/offgrid-llm/internal/execution"
	"github.com/takuphilchan/offgrid-llm/pkg/api"
)

func TestExecutionProjectUpgradeRetainsIdentityAndNewerSelection(t *testing.T) {
	for _, clear := range []bool{false, true} {
		t.Run(map[bool]string{false: "upgrade", true: "newer-clear"}[clear], func(t *testing.T) {
			m, r, scope, c, task, resource := canonicalWorkspaceInputFixture(t)
			project := &ExecutionProjectContext{Snapshot: folderSnapshot(t, "model")}
			if _, err := r.ResolveExecutionInputWithProject(task.ID, scope.Actor, "access", resource, project); err != nil {
				t.Fatal(err)
			}
			bound, _ := m.Conversation(scope, c.ID)
			if clear {
				_, err := m.ApplyConversationCommand(scope, ConversationCommand{RequestID: "clear", TargetID: c.ID, ExpectedRevision: bound.Revision, Kind: "clear_project_folder"})
				if err != nil {
					t.Fatal(err)
				}
			}
			_, err := m.updateTask(task.ID, func(next *Task) error {
				next.Status = TaskInput
				next.PendingInput = &InputRequest{ID: "upgrade", CallID: "upgrade-call", Kind: "workspace", Mode: "edit", Target: "Local workspace"}
				next.Checkpoint.Calls = []api.ToolCall{{ID: "upgrade-call", Type: "function", Function: api.FunctionCall{Name: execution.AccessRequestTool, Arguments: `{"access":"edit"}`}}}
				return nil
			})
			if err != nil {
				t.Fatal(err)
			}
			resource.Reference.Session, resource.Reference.Grant = "upgraded-session", "upgraded-grant"
			resource.Permissions.Write = true
			resolved, err := r.ResolveExecutionInputWithProject(task.ID, scope.Actor, "upgrade", resource, project)
			if err != nil || resolved.Config.ProjectID != task.Config.ProjectID {
				t.Fatal(resolved, err)
			}
			current, _ := m.Conversation(scope, c.ID)
			ref, err := m.ConversationFolder(scope, c.ID)
			if err != nil || current.ProjectID != bound.ProjectID {
				t.Fatal(current, err)
			}
			if clear {
				if ref != nil {
					t.Fatal("upgrade undid explicit clear", ref)
				}
			} else if ref == nil || *ref != resource.Reference || current.ContextRevision != bound.ContextRevision+1 {
				t.Fatal("upgrade did not retain current folder access", ref, current)
			}
			if _, err = r.Stop(task.ID, scope.Actor, "cancel", ""); err != nil {
				t.Fatal(err)
			}
			retry, err := r.ResolveExecutionInputWithProject(task.ID, scope.Actor, "upgrade", resource, project)
			if err != nil || retry.Status != TaskCancelled {
				t.Fatal("duplicate revived stopped work", retry, err)
			}
		})
	}
}

func TestWorkspaceInputPreservesExactToolRestrictions(t *testing.T) {
	for _, mode := range []string{"read-only", "none", "wrong-revision"} {
		t.Run(mode, func(t *testing.T) {
			m, r, _, _, task, resource := canonicalWorkspaceInputFixture(t)
			resource.Permissions.Write = true
			d, _ := execution.Descriptor("workspace_read")
			allowed := []ProfileToolReference{}
			if mode != "none" {
				allowed = append(allowed, ProfileToolReference{Provider: d.Source, Tool: d.Name, SchemaRevision: d.Revision, Location: d.Location})
				if mode == "wrong-revision" {
					allowed[0].SchemaRevision = "obsolete"
				}
			}
			_, err := m.updateTask(task.ID, func(next *Task) error {
				next.Config.ConversationConfiguration = &ConversationConfiguration{Version: 1, Model: "model", Restrictions: ResourceRestrictions{Tools: &allowed}}
				return nil
			})
			if err != nil {
				t.Fatal(err)
			}
			resolved, err := r.ResolveExecutionInput(task.ID, task.Actor, task.PendingInput.ID, resource)
			if err != nil {
				t.Fatal(err)
			}
			for name := range resolved.Config.AllowedCapabilities {
				if mode != "read-only" || name != "workspace_read" {
					t.Fatal("access widened tool restrictions", name)
				}
			}
			if mode == "read-only" {
				if _, ok := resolved.Config.AllowedCapabilities["workspace_read"]; !ok {
					t.Fatal("allowed read missing")
				}
			}
		})
	}
}
