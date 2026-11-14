package server

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/takuphilchan/offgrid-llm/internal/agents"
	"github.com/takuphilchan/offgrid-llm/internal/execution"
	"github.com/takuphilchan/offgrid-llm/pkg/api"
)

func executionProjectFixture(t *testing.T) (*Server, agents.ControlScope, *agents.Conversation, *agents.Task, execution.Reference, string) {
	t.Helper()
	s := profileFixture(t)
	s.config.EnableConversationPreview, s.config.EnableExecutionPreview = true, true
	ref, root := approvedExecutionCopiesFixture(t, s, execution.Permissions{Read: true}, execution.WorkingCopyPolicy{}, strings.Repeat("b", 64))
	if err := os.Mkdir(filepath.Join(root, ".offgrid"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".offgrid", "project.toml"), []byte("version=1\n[preferences]\nmodel='project-model'\ndelegation=false\n"), 0600); err != nil {
		t.Fatal(err)
	}
	s.agentRunner = agents.NewRunner(s.agentManager, &browserRunTools{RunTools: s.toolRegistry, server: s}, func(_ context.Context, task *agents.Task, _ []api.ChatMessage, _ []api.Tool) (*api.ChatCompletionResponse, error) {
		if task.Config.Profile != nil {
			t.Error("automatic project needs a profile")
		}
		message, finish := api.ChatMessage{Role: "assistant", Content: "Folder available."}, "stop"
		if task.Config.Execution == nil {
			message.Content, finish = "", "tool_calls"
			message.ToolCalls = []api.ToolCall{{ID: "request-folder", Type: "function", Function: api.FunctionCall{Name: execution.AccessRequestTool, Arguments: `{"access":"read"}`}}}
		}
		return &api.ChatCompletionResponse{Choices: []api.ChatCompletionChoice{{FinishReason: finish, Message: message}}}, nil
	})
	t.Cleanup(func() { _ = s.agentRunner.Shutdown(context.Background()) })
	scope := agents.ControlScope{Workspace: s.workspaceID, Actor: "local-admin"}
	created, err := s.agentManager.ApplyConversationCommand(scope, agents.ConversationCommand{RequestID: "create", Kind: "create_conversation", Payload: agents.ConversationCommandPayload{Preferences: &agents.ConversationPreferences{Model: "model", Restrictions: agents.ResourceRestrictions{Execution: &api.ExecutionPermissions{Read: true}}}}})
	if err != nil {
		t.Fatal(err)
	}
	c := created.Conversation
	command := agents.ConversationCommand{RequestID: "work", TargetID: c.ID, ExpectedRevision: c.Revision, Kind: "create_work", Payload: agents.ConversationCommandPayload{Input: &agents.ConversationInput{Content: "Inspect the approved folder"}}}
	w := controlHTTP(s, "POST", "/api/v2/conversations/"+c.ID+"/commands", command)
	if w.Code != 202 {
		t.Fatal(w.Code, w.Body.String())
	}
	var receipt agents.ConversationCommandReceipt
	if err = json.Unmarshal(w.Body.Bytes(), &receipt); err != nil {
		t.Fatal(err)
	}
	task, err := s.agentRunner.Continue(context.Background(), receipt.Work.RunID, scope.Actor, "start", "", false)
	if err != nil || task.Status != agents.TaskInput || task.PendingInput == nil {
		t.Fatal("no workspace request", task, err)
	}
	c, err = s.agentManager.Conversation(scope, c.ID)
	if err != nil {
		t.Fatal(err)
	}
	return s, scope, c, task, ref, root
}

func submitProjectAccess(s *Server, task *agents.Task, ref execution.Reference) *httptest.ResponseRecorder {
	body, _ := json.Marshal(map[string]any{"input_id": task.PendingInput.ID, "execution": ref})
	w := httptest.NewRecorder()
	s.handleJobInput(w, httptest.NewRequest("POST", "/api/v2/jobs/"+task.ID+"/input", strings.NewReader(string(body))), task.ID)
	return w
}

func TestServiceExecutionProjectApprovalContinuesSameWorkAndReusesFolder(t *testing.T) {
	s, scope, c, task, ref, _ := executionProjectFixture(t)
	w := submitProjectAccess(s, task, ref)
	if w.Code != 202 {
		t.Fatal(w.Code, w.Body.String())
	}
	deadline := time.Now().Add(5 * time.Second)
	var done *agents.Task
	for time.Now().Before(deadline) {
		done, _ = s.agentManager.GetTask(task.ID)
		if done.Status == agents.TaskCompleted {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}
	if done.Status != agents.TaskCompleted || done.WorkID != task.WorkID || len(s.agentManager.ListTasks()) != 1 {
		t.Fatal("access recreated or did not continue work", done)
	}
	current, err := s.agentManager.Conversation(scope, c.ID)
	if err != nil || current.ProjectID == c.ProjectID || current.FocusedWorkID != task.WorkID {
		t.Fatal(current, err)
	}
	folder, err := s.agentManager.ConversationFolder(scope, c.ID)
	if err != nil || folder == nil || *folder != ref {
		t.Fatal(folder, err)
	}
	config, model, ok := s.resolveConversationRun(httptest.NewRecorder(), httptest.NewRequest("POST", "/", nil), current, agents.ConversationCommand{Payload: agents.ConversationCommandPayload{Input: &agents.ConversationInput{Content: "Continue in this folder"}}})
	if !ok || model != "model" || config.Profile != nil || config.Execution == nil || *config.Execution != ref || !config.DisableDelegation {
		t.Fatal("next work lost model/reusable folder/preferences", config, model, ok)
	}
	if done.Config.ProjectID != task.Config.ProjectID || done.Config.MaxIterations != task.Config.MaxIterations {
		t.Fatal("approval rewrote ongoing work")
	}
	if w = submitProjectAccess(s, task, ref); w.Code != 202 {
		t.Fatal(w.Code, w.Body.String())
	}
	again, _ := s.agentManager.Conversation(scope, c.ID)
	if again.Revision != current.Revision {
		t.Fatal("retry rebound project")
	}
	public := controlHTTP(s, "GET", "/api/v2/conversations/"+c.ID+"/configuration", nil)
	if public.Code != 200 || !strings.Contains(public.Body.String(), `"available":true`) || strings.Contains(public.Body.String(), ref.Grant) {
		t.Fatal("invalid public configuration", public.Code, public.Body.String())
	}
}

func TestServiceExecutionProjectInvalidAndRevokedApprovalIsUnaccepted(t *testing.T) {
	for _, failure := range []string{"invalid-file", "revoked", "stopped"} {
		t.Run(failure, func(t *testing.T) {
			s, scope, c, task, ref, root := executionProjectFixture(t)
			want := 409
			switch failure {
			case "invalid-file":
				want = 422
				if err := os.WriteFile(filepath.Join(root, ".offgrid", "project.toml"), []byte("hooks=['untrusted']"), 0600); err != nil {
					t.Fatal(err)
				}
			case "revoked":
				if err := s.executionHosts.Revoke(scope.Actor, scope.Workspace, ref); err != nil {
					t.Fatal(err)
				}
			case "stopped":
				if _, err := s.agentRunner.Stop(task.ID, scope.Actor, "cancel", ""); err != nil {
					t.Fatal(err)
				}
			}
			w := submitProjectAccess(s, task, ref)
			if w.Code != want {
				t.Fatal(w.Code, w.Body.String())
			}
			current, _ := s.agentManager.Conversation(scope, c.ID)
			folder, err := s.agentManager.ConversationFolder(scope, c.ID)
			saved, _ := s.agentManager.GetTask(task.ID)
			if err != nil || folder != nil || current.ProjectID != c.ProjectID || current.Revision != c.Revision || saved.Config.Execution != nil || saved.ResolvedInputID != "" {
				t.Fatal("failed approval partially persisted", current, saved, err)
			}
		})
	}
}

func TestServiceExecutionProjectDiscoveryHonorsPinnedRestrictions(t *testing.T) {
	for _, ceiling := range []string{"tools", "execution", "profile"} {
		t.Run(ceiling, func(t *testing.T) {
			s, scope, _, task, ref, root := executionProjectFixture(t)
			// A parse error would surface if discovery crossed the pinned ceiling.
			if err := os.WriteFile(filepath.Join(root, ".offgrid", "project.toml"), []byte("forbidden=['hook']"), 0600); err != nil {
				t.Fatal(err)
			}
			switch ceiling {
			case "tools":
				empty := []agents.ProfileToolReference{}
				task.Config.ConversationConfiguration.Restrictions.Tools = &empty
			case "execution":
				task.Config.ExecutionPolicy = &execution.Permissions{}
			case "profile":
				task.Config.Profile = &agents.AgentProfile{Settings: agents.ProfileSettings{Execution: &execution.Permissions{}}}
			}
			resource, err := s.executionHosts.Resolve(context.Background(), scope.Actor, scope.Workspace, ref)
			if err != nil {
				t.Fatal(err)
			}
			w := httptest.NewRecorder()
			project, ok := s.prepareExecutionProject(w, httptest.NewRequest("POST", "/", nil), task, resource)
			if !ok || project != nil || w.Body.Len() != 0 {
				t.Fatal("pinned restriction allowed discovery", project, ok, w.Body.String())
			}
		})
	}
}
