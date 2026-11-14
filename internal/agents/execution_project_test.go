package agents

import (
	"errors"
	"testing"

	"github.com/takuphilchan/offgrid-llm/internal/execution"
	"github.com/takuphilchan/offgrid-llm/pkg/api"
)

func canonicalWorkspaceInputFixture(t *testing.T) (*Manager, *Runner, ControlScope, *Conversation, *Task, execution.Resource) {
	t.Helper()
	m, r, scope, c := controlFixture(t)
	config := taskConfig()
	config.TaskFirst, config.ExecutionRequestVersion = true, 1
	config.ProjectID, config.ProjectRevision = c.ProjectID, 1
	config.ConversationContextRevision = c.ContextRevision
	config.ExecutionPolicy = &execution.Permissions{Read: true, Write: true}
	accepted, err := r.AdmitConversationWork(scope, ConversationCommand{RequestID: "work", TargetID: c.ID, ExpectedRevision: c.Revision, Kind: "create_work", Payload: ConversationCommandPayload{Input: &ConversationInput{Content: "Read the approved folder"}}}, "model", config)
	if err != nil {
		t.Fatal(err)
	}
	task, err := m.updateTask(accepted.Work.RunID, func(next *Task) error {
		next.Status = TaskInput
		next.PendingInput = &InputRequest{ID: "access", CallID: "access-call", Kind: "workspace", Mode: "read", Target: "Local workspace"}
		next.Checkpoint.Calls = []api.ToolCall{{ID: "access-call", Type: "function", Function: api.FunctionCall{Name: execution.AccessRequestTool, Arguments: `{"access":"read"}`}}}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	c, err = m.Conversation(scope, c.ID)
	if err != nil {
		t.Fatal(err)
	}
	resource := execution.Resource{ProjectContext: true, Reference: execution.Reference{Host: "host", Session: "session", Grant: "grant", Revision: 1}, Backend: "local", Root: t.TempDir(), Directory: ".", Permissions: execution.Permissions{Read: true}}
	return m, r, scope, c, task, resource
}

func TestExecutionProjectBindsAtomicallyWithoutChangingWorkOrBudget(t *testing.T) {
	m, r, scope, c, task, resource := canonicalWorkspaceInputFixture(t)
	emptyFocus := ""
	_, err := r.AcceptConversationInput(scope, api.ConversationInputCommand{RequestID: "side-question", TargetID: c.ID, ExpectedRevision: c.Revision, Kind: "submit_input", Payload: api.ConversationInboxInput{Content: "What does this mean?", FocusedWorkID: &emptyFocus}})
	if err != nil {
		t.Fatal(err)
	}
	project := &ExecutionProjectContext{Snapshot: folderSnapshot(t, "project-model")}
	resolved, err := r.ResolveExecutionInputWithProject(task.ID, scope.Actor, "access", resource, project)
	if err != nil {
		t.Fatal(err)
	}
	current, err := m.Conversation(scope, c.ID)
	if err != nil || current.ProjectID == c.ProjectID || current.ContextRevision != c.ContextRevision+1 || current.FocusedWorkID != task.WorkID {
		t.Fatal("project or focus lost", current, err)
	}
	ref, err := m.ConversationFolder(scope, c.ID)
	if err != nil || ref == nil || *ref != resource.Reference {
		t.Fatal(ref, err)
	}
	work, err := m.WorkItem(scope, task.WorkID)
	if err != nil || work.ProjectID != task.Config.ProjectID || work.RunID != task.ID || resolved.Config.ProjectID != task.Config.ProjectID || resolved.Config.MaxIterations != task.Config.MaxIterations || resolved.Checkpoint.Iteration != task.Checkpoint.Iteration {
		t.Fatal("access rewrote pinned work/budget", work, resolved, err)
	}
	if resolved.Config.ExecutionProject == nil || resolved.Config.ExecutionProject.ProjectID != current.ProjectID {
		t.Fatal("missing future upgrade continuity")
	}
	changed := &ExecutionProjectContext{Snapshot: folderSnapshot(t, "must-not-refresh-on-retry")}
	duplicate, err := r.ResolveExecutionInputWithProject(task.ID, scope.Actor, "access", resource, changed)
	if err != nil || len(duplicate.Steps) != len(resolved.Steps) {
		t.Fatal("duplicate input applied", duplicate, err)
	}
	again, _ := m.Conversation(scope, c.ID)
	if again.Revision != current.Revision {
		t.Fatal("retry changed conversation")
	}
	stored, _ := m.Project(scope, current.ProjectID)
	if stored.Preferences.Model != "project-model" {
		t.Fatal("retry imported new file bytes")
	}
	reloaded := NewManagerWithPersistence(nil, nil, nil, m.dataDir)
	if err = reloaded.StorageError(); err != nil {
		t.Fatal(err)
	}
	rr := NewRunner(reloaded, NewToolRegistry(), nil)
	retry, err := rr.ResolveExecutionInputWithProject(task.ID, scope.Actor, "access", resource, changed)
	if err != nil || retry.Status != TaskInterrupted {
		t.Fatal("restart replayed access or revived work", retry, err)
	}
}

func TestExecutionProjectPreservesNewerSelectionAndStop(t *testing.T) {
	for _, change := range []string{"clear", "project", "stop", "correction"} {
		t.Run(change, func(t *testing.T) {
			m, r, scope, c, task, resource := canonicalWorkspaceInputFixture(t)
			var err error
			switch change {
			case "clear", "project":
				command := ConversationCommand{RequestID: "new-selection", TargetID: c.ID, ExpectedRevision: c.Revision, Kind: "clear_project_folder"}
				if change == "project" {
					command.Kind, command.Payload.ProjectID = "select_project", c.ProjectID
				}
				_, err = m.ApplyConversationCommand(scope, command)
			case "stop":
				_, err = r.Stop(task.ID, scope.Actor, "cancel", "")
			case "correction":
				_, err = r.AcceptConversationInput(scope, api.ConversationInputCommand{RequestID: "correction", TargetID: c.ID, ExpectedRevision: c.Revision, Kind: "submit_input", Payload: api.ConversationInboxInput{Content: "Do not modify anything", Intent: "correction"}})
			}
			if err != nil {
				t.Fatal(err)
			}
			_, err = r.ResolveExecutionInputWithProject(task.ID, scope.Actor, "access", resource, &ExecutionProjectContext{Snapshot: folderSnapshot(t, "model")})
			if change == "stop" || change == "correction" {
				if !errors.Is(err, ErrRunConflict) {
					t.Fatal("late access bypassed control", err)
				}
			} else if err != nil {
				t.Fatal("new selection prevented independent work approval", err)
			}
			current, readErr := m.Conversation(scope, c.ID)
			ref, refErr := m.ConversationFolder(scope, c.ID)
			if readErr != nil || refErr != nil || ref != nil || current.ProjectID != c.ProjectID {
				t.Fatal("late approval overwrote selection", current, ref, readErr, refErr)
			}
		})
	}
}

func TestExecutionProjectRollbackNeverPublishesHalfAnApproval(t *testing.T) {
	for _, failure := range []string{"folder", "task", "event"} {
		t.Run(failure, func(t *testing.T) {
			m, r, scope, c, task, resource := canonicalWorkspaceInputFixture(t)
			db, err := openTaskDatabase(m.dataDir)
			if err != nil {
				t.Fatal(err)
			}
			defer db.Close()
			statement := map[string]string{"folder": `CREATE TRIGGER reject_project BEFORE INSERT ON agent_conversation_folders BEGIN SELECT RAISE(ABORT,'fixture'); END`, "task": `CREATE TRIGGER reject_project BEFORE UPDATE ON agent_tasks BEGIN SELECT RAISE(ABORT,'fixture'); END`, "event": `CREATE TRIGGER reject_project BEFORE INSERT ON agent_control_events BEGIN SELECT RAISE(ABORT,'fixture'); END`}[failure]
			if _, err = db.Exec(statement); err != nil {
				t.Fatal(err)
			}
			if _, err = r.ResolveExecutionInputWithProject(task.ID, scope.Actor, "access", resource, &ExecutionProjectContext{Snapshot: folderSnapshot(t, "model")}); err == nil {
				t.Fatal("fault accepted")
			}
			live, _ := m.GetTask(task.ID)
			if live.Config.Execution != nil || live.PendingInput == nil || live.ResolvedInputID != "" {
				t.Fatal("uncommitted authority published")
			}
			if _, err = db.Exec(`DROP TRIGGER reject_project`); err != nil {
				t.Fatal(err)
			}
			restored := NewManagerWithPersistence(nil, nil, nil, m.dataDir)
			if err = restored.StorageError(); err != nil {
				t.Fatal(err)
			}
			current, err := restored.Conversation(scope, c.ID)
			if err != nil || current.ProjectID != c.ProjectID || current.Revision != c.Revision {
				t.Fatal("partial conversation persisted", current, err)
			}
			stored, _ := restored.GetTask(task.ID)
			if stored.Status != TaskInput || stored.Config.Execution != nil || stored.ResolvedInputID != "" {
				t.Fatal("partial access persisted", stored)
			}
		})
	}
}
