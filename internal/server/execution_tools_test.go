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
	"github.com/takuphilchan/offgrid-llm/internal/capabilities"
	"github.com/takuphilchan/offgrid-llm/internal/execution"
	"github.com/takuphilchan/offgrid-llm/pkg/api"
)

func approvedExecutionFixture(t *testing.T, s *Server) (execution.Reference, string) {
	return approvedExecutionPermissionsFixture(t, s, execution.Permissions{Read: true, Write: true})
}

func approvedExecutionPermissionsFixture(t *testing.T, s *Server, permissions execution.Permissions) (execution.Reference, string) {
	t.Helper()
	root := t.TempDir()
	b := execution.Binding{Actor: "local-admin", Workspace: s.workspaceID, Host: "fixture-host", Session: "fixture-session", Grant: "fixture-local-grant", Revision: 1, Job: "supervisor"}
	config := execution.Context{Version: 1, Binding: b, Backend: "local", Root: root, Directory: ".", Permissions: permissions}
	session, err := execution.OpenSession(config)
	if err != nil {
		t.Fatal(err)
	}
	journal, err := execution.OpenJournal(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	worker := &execution.Worker{Session: session, Journal: journal, Processes: execution.NewProcessManager(session), Token: strings.Repeat("a", 64)}
	endpoint := execution.NewLocalEndpoint(worker)
	s.executionHosts = execution.NewHosts()
	if err = s.executionHosts.RegisterApproved(context.Background(), config, endpoint); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { endpoint.Close(); session.Close(); journal.Close() })
	return execution.Reference{Host: b.Host, Session: b.Session, Grant: b.Grant, Revision: b.Revision}, root
}

func TestCanonicalConversationExecutesSelectedWorkspaceAndExactEdit(t *testing.T) {
	f := newConversationFixture(t)
	ref, root := approvedExecutionFixture(t, f.s)
	const original = "original café 日本語"
	const wanted = "OffGrid test — café 日本語 12345\n1| genuine content"
	if err := os.WriteFile(filepath.Join(root, "test.txt"), []byte(original), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "untouched.txt"), []byte("must remain"), 0600); err != nil {
		t.Fatal(err)
	}
	tools := &browserRunTools{RunTools: f.s.toolRegistry, server: f.s}
	f.s.agentRunner = agents.NewRunner(f.s.agentManager, tools, func(_ context.Context, task *agents.Task, messages []api.ChatMessage, available []api.Tool) (*api.ChatCompletionResponse, error) {
		if task.Config.Execution == nil || *task.Config.Execution != ref || task.Origin == nil {
			t.Error("execution not pinned to canonical turn")
		}
		for _, tool := range available {
			if serviceFileTool(tool.Function.Name) || tool.Function.Name == "shell" {
				t.Error("service path fallback", tool.Function.Name)
			}
		}
		count := 0
		var last string
		for _, step := range task.Steps {
			if step.Type == "action" {
				count++
			}
		}
		for _, m := range messages {
			if m.Role == "tool" {
				last = m.StringContent()
			}
		}
		name := ""
		args := `{}`
		switch count {
		case 0:
			name = "workspace_list"
		case 1:
			if !strings.Contains(last, "test.txt") || !strings.Contains(last, ref.Host) {
				t.Error("wrong listing", last)
			}
			name = "workspace_read"
			args = `{"path":"test.txt"}`
		case 2:
			var read struct {
				Text    string `json:"text"`
				Receipt struct {
					Result execution.FilePage `json:"result"`
				} `json:"receipt"`
			}
			if json.Unmarshal([]byte(last), &read) != nil || read.Text != original {
				t.Error("inexact raw read", last)
			}
			name = "workspace_write"
			encoded, _ := json.Marshal(map[string]string{"path": "test.txt", "content": wanted, "revision": read.Receipt.Result.Revision})
			args = string(encoded)
		case 3:
			if !strings.Contains(last, `"verified":true`) {
				t.Error("missing independent write check", last)
			}
			name = "workspace_read"
			args = `{"path":"test.txt"}`
		case 4:
			var read struct {
				Text string `json:"text"`
			}
			if json.Unmarshal([]byte(last), &read) != nil || read.Text != wanted {
				t.Error("content changed", last)
			}
		default:
			t.Fatal("unexpected tool loop", count)
		}
		message := api.ChatMessage{Role: "assistant", Content: "Exact edit read back."}
		reason := "stop"
		if name != "" {
			message.Content = ""
			message.ToolCalls = []api.ToolCall{{ID: name + string(rune('a'+count)), Type: "function", Function: api.FunctionCall{Name: name, Arguments: args}}}
			reason = "tool_calls"
		}
		return &api.ChatCompletionResponse{Choices: []api.ChatCompletionChoice{{FinishReason: reason, Message: message}}}, nil
	})
	f.s.agentRunner.Observer = f.s.projectConversationTask
	runner := f.s.agentRunner
	t.Cleanup(func() { runner.Shutdown(context.Background()) })
	f.request.Content = "List files, read test.txt, replace its content exactly as requested and verify the edit."
	f.request.Execution = &ref
	w := f.submit(f.request)
	if w.Code != 202 {
		t.Fatal(w.Code, w.Body.String())
	}
	saved := f.waitComplete(t)
	if saved.Turn.Output != "Exact edit read back." {
		t.Fatal(saved.Turn)
	}
	actual, err := os.ReadFile(filepath.Join(root, "test.txt"))
	if err != nil || string(actual) != wanted {
		t.Fatal(string(actual), err)
	}
	untouched, _ := os.ReadFile(filepath.Join(root, "untouched.txt"))
	if string(untouched) != "must remain" {
		t.Fatal("unrelated file changed")
	}
	task, _ := f.s.agentManager.GetTask(saved.Turn.JobID)
	if task.Config.Execution == nil || *task.Config.Execution != ref || task.Config.ExecutionWorkspace == nil || task.Config.ExecutionWorkspace.Root != root {
		t.Fatal("reference lost")
	}
	// Restart preserves the original context for inspection, not a live grant.
	restored := agents.NewManagerWithPersistence(nil, nil, nil, f.agentDirectory)
	archived, found := restored.GetTask(task.ID)
	if !found || archived.Config.ExecutionWorkspace == nil || archived.Config.ExecutionWorkspace.Root != root {
		t.Fatal("admission context lost after restart", archived)
	}
	if err := f.s.executionHosts.Revoke("local-admin", f.s.workspaceID, ref); err != nil {
		t.Fatal(err)
	}
	// Observing accepted work never reauthorizes or repeats its edit.
	if retry := f.submit(f.request); retry.Code != 202 {
		t.Fatal(retry.Code, retry.Body.String())
	}
	f.request.RequestID = "next-request-002"
	if next := f.submit(f.request); next.Code != 422 {
		t.Fatal("accepted disconnected host", next.Code, next.Body.String())
	}
}

func TestExecutionProfilePolicyFiltersDiscoveryAndIndependentDispatch(t *testing.T) {
	s := profileFixture(t)
	ref, _ := approvedExecutionFixture(t, s)
	config := agents.DefaultAgentConfig()
	config.TaskFirst = true
	config.Execution = &ref
	resource, err := s.executionHosts.Resolve(context.Background(), "local-admin", s.workspaceID, ref)
	if err != nil {
		t.Fatal(err)
	}
	config.ExecutionWorkspace = &resource
	config.ExecutionPolicy = &execution.Permissions{Read: true}
	config.AllowedCapabilities = map[string]capabilities.Descriptor{}
	for _, tool := range executionTools() {
		d, _ := executionDescriptor(tool.Function.Name)
		config.AllowedCapabilities[d.Name] = d
	}
	task := &agents.Task{ID: "policy-fixture", Actor: "local-admin", Config: config, Status: agents.TaskRunning}
	tools := &browserRunTools{RunTools: s.toolRegistry, server: s}
	for _, tool := range tools.workspaceTools(context.Background(), task) {
		if tool.Function.Name == "workspace_write" || strings.Contains(tool.Function.Name, "process") {
			t.Fatal("policy leak", tool.Function.Name)
		}
	}
	d, _ := executionDescriptor("workspace_write")
	err = tools.executionAccess(context.Background(), task, "workspace_write", agents.ToolExecution{Actor: task.Actor, RunID: task.ID, ExpectedCapability: &d})
	if err == nil {
		t.Fatal("dispatch bypassed profile ceiling")
	}
	config.ExecutionPolicy = &execution.Permissions{}
	task.Config = config
	if got := tools.workspaceTools(context.Background(), task); len(got) != 0 {
		t.Fatal("all-false policy restored tools", got)
	}
}

func TestExecutionDiscoveryRejectsRepublishedRootAndNarrowedPermission(t *testing.T) {
	s := profileFixture(t)
	ref, root := approvedExecutionFixture(t, s)
	resource, err := s.executionHosts.Resolve(context.Background(), "local-admin", s.workspaceID, ref)
	if err != nil {
		t.Fatal(err)
	}
	config := agents.DefaultAgentConfig()
	config.TaskFirst = true
	config.Execution = &ref
	config.ExecutionWorkspace = &resource
	task := &agents.Task{ID: "fixture", Actor: "local-admin", Status: agents.TaskRunning, Config: config}
	tools := &browserRunTools{RunTools: s.toolRegistry, server: s}
	if len(tools.workspaceTools(context.Background(), task)) == 0 {
		t.Fatal("approved scope unavailable")
	}
	changed := resource
	changed.Root = root + "-different"
	task.Config.ExecutionWorkspace = &changed
	if len(tools.workspaceTools(context.Background(), task)) != 0 {
		t.Fatal("same reference accepted another root")
	}
	changed = resource
	changed.Permissions.Write = false
	task.Config.ExecutionWorkspace = &changed
	for _, tool := range tools.workspaceTools(context.Background(), task) {
		if tool.Function.Name == "workspace_write" {
			t.Fatal("narrowing lost")
		}
	}
}

func TestWorkspaceAuthorityAndServiceFallbackAreIndependent(t *testing.T) {
	s := profileFixture(t)
	ref, _ := approvedExecutionFixture(t, s)
	config := agents.DefaultAgentConfig()
	config.TaskFirst = true
	config.Execution = &ref
	resource, resolveErr := s.executionHosts.Resolve(context.Background(), "local-admin", s.workspaceID, ref)
	if resolveErr != nil {
		t.Fatal(resolveErr)
	}
	config.ExecutionWorkspace = &resource
	task, err := s.agentRunner.Create("fixture", "model", "local-admin", config)
	if err != nil {
		t.Fatal(err)
	}
	tools := &browserRunTools{RunTools: s.toolRegistry, server: s}
	d, _ := s.toolRegistry.Capability("list_files")
	if err = tools.Authorize(context.Background(), "list_files", json.RawMessage(`{"path":"."}`), agents.ToolExecution{Actor: task.Actor, RunID: task.ID, ExpectedCapability: &d}); err == nil {
		t.Fatal("allowed service fallback")
	}
	for _, name := range []string{"workspace_list", "workspace_write"} {
		d, _ := executionDescriptor(name)
		if err = tools.Authorize(context.Background(), name, json.RawMessage(`{}`), agents.ToolExecution{Actor: "other", RunID: task.ID, ExpectedCapability: &d}); err == nil {
			t.Fatal("cross-actor execution")
		}
	}
	w := httptest.NewRecorder()
	s.handleJobSubmit(w, httptest.NewRequest("POST", "/api/v2/jobs", strings.NewReader(`{"prompt":"list files","model":"model","request_id":"bad-grant-0001","execution":{"host":"fixture-host","session":"fixture-session","grant":"fixture-local-grant","revision":2}}`)))
	if w.Code != 422 {
		t.Fatal(w.Code, w.Body.String())
	}
	w = httptest.NewRecorder()
	s.handleExecutionWorkspaces(w, httptest.NewRequest("GET", "/api/v2/execution/workspaces", nil))
	if w.Code != 200 || !strings.Contains(w.Body.String(), ref.Session) || strings.Contains(w.Body.String(), strings.Repeat("a", 64)) {
		t.Fatal(w.Code, w.Body.String())
	}
	w = httptest.NewRecorder()
	s.handleExecutionWorkspaces(w, httptest.NewRequest("POST", "/api/v2/execution/workspaces", strings.NewReader(`{"root":"/"}`)))
	if w.Code != 405 {
		t.Fatal("HTTP minted root authority", w.Code)
	}
}
