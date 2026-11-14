package server

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/takuphilchan/offgrid-llm/internal/agents"
	"github.com/takuphilchan/offgrid-llm/internal/execution"
	"github.com/takuphilchan/offgrid-llm/pkg/api"
)

func projectFolderFixture(t *testing.T) (*Server, agents.ControlScope, *agents.Conversation, agents.ConversationCommand, string) {
	t.Helper()
	s := profileFixture(t)
	s.config.EnableConversationPreview, s.config.EnableExecutionPreview = true, true
	ref, root := approvedExecutionCopiesFixture(t, s, execution.Permissions{Read: true, Write: true, Processes: true}, execution.WorkingCopyPolicy{}, strings.Repeat("b", 64))
	if err := os.Mkdir(filepath.Join(root, ".offgrid"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".offgrid", "project.toml"), []byte("version=1\n[preferences]\nmodel='project-model'\ndelegation=false\n"), 0600); err != nil {
		t.Fatal(err)
	}
	scope := agents.ControlScope{Workspace: s.workspaceID, Actor: "local-admin"}
	created, err := s.agentManager.ApplyConversationCommand(scope, agents.ConversationCommand{RequestID: "conversation", Kind: "create_conversation", Payload: agents.ConversationCommandPayload{Preferences: &agents.ConversationPreferences{Model: "model", Restrictions: agents.ResourceRestrictions{Execution: &api.ExecutionPermissions{Read: true}}}}})
	if err != nil {
		t.Fatal(err)
	}
	c := created.Conversation
	command := agents.ConversationCommand{RequestID: "bind-folder", TargetID: c.ID, ExpectedRevision: c.Revision, Kind: "use_project_folder", Payload: agents.ConversationCommandPayload{Folder: &api.ExecutionReference{Host: ref.Host, Session: ref.Session, Grant: ref.Grant, Revision: ref.Revision}}}
	return s, scope, c, command, root
}

func TestProjectFolderServiceRunContinuityRevocationAndRetry(t *testing.T) {
	s, scope, c, command, root := projectFolderFixture(t)
	path := "/api/v2/conversations/" + c.ID
	w := controlHTTP(s, "POST", path+"/commands", command)
	if w.Code != 202 {
		t.Fatal(w.Code, w.Body.String())
	}
	var receipt agents.ConversationCommandReceipt
	if err := json.Unmarshal(w.Body.Bytes(), &receipt); err != nil {
		t.Fatal(err)
	}
	if receipt.Project == nil || receipt.Project.Preferences.Model != "project-model" || receipt.Conversation.Preferences.Model != "model" {
		t.Fatal(receipt)
	}
	if strings.Contains(w.Body.String(), command.Payload.Folder.Grant) {
		t.Fatal("public receipt disclosed private folder reference")
	}
	c = receipt.Conversation
	r := httptest.NewRequest("POST", "/", nil)
	input := agents.ConversationCommand{Payload: agents.ConversationCommandPayload{Input: &agents.ConversationInput{Content: "Inspect the folder"}}}
	config, model, ok := s.resolveConversationRun(httptest.NewRecorder(), r, c, input)
	if !ok || model != "model" || config.Execution == nil || config.ExecutionWorkspace.Root != root || config.ExecutionRequestVersion != execution.AccessRequestVersion || !config.DisableDelegation {
		t.Fatal("selected folder/configuration dropped", config, ok)
	}
	if _, found := config.AllowedCapabilities["workspace_read"]; !found {
		t.Fatal("read capability missing")
	}
	if _, found := config.AllowedCapabilities["workspace_write"]; found {
		t.Fatal("conversation write restriction widened")
	}
	if _, found := config.AllowedCapabilities["workspace_process_start"]; found {
		t.Fatal("conversation process restriction widened")
	}
	// GET may inspect live access but cannot reimport the changed file.
	if err := os.WriteFile(filepath.Join(root, ".offgrid", "project.toml"), []byte("invalid = true"), 0600); err != nil {
		t.Fatal(err)
	}
	w = controlHTTP(s, "GET", path+"/configuration", nil)
	if w.Code != 200 || !strings.Contains(w.Body.String(), `"available":true`) {
		t.Fatal(w.Code, w.Body.String())
	}
	project, err := s.agentManager.Project(scope, c.ProjectID)
	if err != nil || project.Revision != 1 {
		t.Fatal("GET changed project", project, err)
	}
	if err = s.executionHosts.Revoke(scope.Actor, scope.Workspace, *config.Execution); err != nil {
		t.Fatal(err)
	}
	w = controlHTTP(s, "POST", path+"/commands", command)
	if w.Code != 200 {
		t.Fatal("accepted retry required live grant/file", w.Code, w.Body.String())
	}
	config, _, ok = s.resolveConversationRun(httptest.NewRecorder(), r, c, input)
	if !ok || config.Execution != nil || config.EffectivePreferences.Folder == nil || config.EffectivePreferences.Folder.Available || !strings.Contains(config.SystemPrompt, "Workspace limitation") {
		t.Fatal("revoked context was restored or blocked text", config, ok)
	}
	// Clearing a selected folder is explicit, leaves project identity intact,
	// and requires no live host.
	w = controlHTTP(s, "POST", path+"/commands", agents.ConversationCommand{RequestID: "clear", TargetID: c.ID, ExpectedRevision: c.Revision, Kind: "clear_project_folder"})
	if w.Code != 202 {
		t.Fatal(w.Code, w.Body.String())
	}
}

func TestProjectFolderServiceRespectsProfileAndToolCeilings(t *testing.T) {
	for _, ceiling := range []string{"profile", "tools"} {
		t.Run(ceiling, func(t *testing.T) {
			s, scope, c, command, _ := projectFolderFixture(t)
			prefs := c.Preferences
			if ceiling == "profile" {
				profile, err := s.agentManager.SaveProfile(scope.Workspace, scope.Actor, "", 0, agents.ProfileSettings{Name: "No files", Model: "model", MaxIterations: 10, Execution: &execution.Permissions{}})
				if err != nil {
					t.Fatal(err)
				}
				prefs.ProfileID, prefs.ProfileRevision = profile.ID, profile.Revision
			} else {
				denied := []agents.ProfileToolReference{}
				prefs.Restrictions.Tools = &denied
			}
			changed, err := s.agentManager.ApplyConversationCommand(scope, agents.ConversationCommand{RequestID: "restrict", TargetID: c.ID, ExpectedRevision: c.Revision, Kind: "configure_conversation", Payload: agents.ConversationCommandPayload{Preferences: &prefs}})
			if err != nil {
				t.Fatal(err)
			}
			command.ExpectedRevision = changed.Revision
			w := controlHTTP(s, "POST", "/api/v2/conversations/"+c.ID+"/commands", command)
			if w.Code != 403 {
				t.Fatal("folder bypassed ceiling", w.Code, w.Body.String())
			}
		})
	}
}

func TestProjectFolderKeepsForegroundResponsiveWithoutCopyingExecution(t *testing.T) {
	s, scope, c, command, _ := projectFolderFixture(t)
	w := controlHTTP(s, "POST", "/api/v2/conversations/"+c.ID+"/commands", command)
	if w.Code != 202 {
		t.Fatal(w.Code, w.Body.String())
	}
	input := acceptContextInput(t, s, scope, c.ID, "question", "Explain what happens next")
	task, err := s.prepareConversationTurn(context.Background(), scope, c.ID, input.ID)
	if err != nil {
		t.Fatal(err)
	}
	if task.Config.Execution != nil || task.Config.ExecutionWorkspace != nil || task.Config.ExecutionRequestVersion != 0 || task.Config.InputWriteVersion != 0 || len(task.Config.AllowedCapabilities) != 0 {
		t.Fatal("foreground inherited execution authority", task.Config)
	}
	if task.Config.EffectivePreferences.Folder == nil || !task.Config.EffectivePreferences.Folder.Available {
		t.Fatal("foreground lost availability context")
	}
}
