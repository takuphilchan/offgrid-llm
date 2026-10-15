package server

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/takuphilchan/offgrid-llm/internal/agents"
	"github.com/takuphilchan/offgrid-llm/internal/users"
	"github.com/takuphilchan/offgrid-llm/pkg/api"
)

func profileFixture(t *testing.T) *Server {
	t.Helper()
	s := taskFirstServer(t)
	s.workspaceID = "profile-test-workspace"
	s.toolRegistry = agents.NewToolRegistry()
	s.agentRunner = agents.NewRunner(s.agentManager, &browserRunTools{RunTools: s.toolRegistry, server: s}, func(_ context.Context, task *agents.Task, messages []api.ChatMessage, tools []api.Tool) (*api.ChatCompletionResponse, error) {
		for _, tool := range tools {
			if tool.Function.Name == "write_file" || tool.Function.Name == computerAccessToolName || tool.Function.Name == "delegate_tasks" {
				t.Errorf("unselected authority reached model: %s", tool.Function.Name)
			}
		}
		return &api.ChatCompletionResponse{Choices: []api.ChatCompletionChoice{{Message: api.ChatMessage{Role: "assistant", Content: "Hello from the pinned profile."}, FinishReason: "stop"}}}, nil
	})
	return s
}

func profileHTTP(s *Server, method, path string, payload any) *httptest.ResponseRecorder {
	encoded, _ := json.Marshal(payload)
	w := httptest.NewRecorder()
	s.handleAgentProfiles(w, httptest.NewRequest(method, path, strings.NewReader(string(encoded))))
	return w
}

func TestProfileAPIValidatesResourcesAndPinsAdmission(t *testing.T) {
	s := profileFixture(t)
	d, _ := s.toolRegistry.Capability("calculator")
	settings := agents.ProfileSettings{Name: "Research", Model: "model", Instructions: "Keep replies concise.", MaxIterations: 8, Tools: []agents.ProfileToolReference{{Provider: d.Source, Tool: d.Name, SchemaRevision: d.Revision, Location: d.Location}}}
	w := profileHTTP(s, "POST", "/api/v2/agent-profiles", map[string]any{"settings": settings})
	if w.Code != 201 {
		t.Fatal(w.Code, w.Body.String())
	}
	var profile agents.AgentProfile
	if err := json.Unmarshal(w.Body.Bytes(), &profile); err != nil {
		t.Fatal(err)
	}
	payload := fmt.Sprintf(`{"prompt":"hello","request_id":"profile-turn-001","profile_id":%q,"profile_revision":1}`, profile.ID)
	post := func() *httptest.ResponseRecorder {
		w := httptest.NewRecorder()
		s.handleJobSubmit(w, httptest.NewRequest("POST", "/api/v2/jobs", strings.NewReader(payload)))
		return w
	}
	accepted := post()
	if accepted.Code != 202 {
		t.Fatal(accepted.Code, accepted.Body.String())
	}
	var response struct {
		ID string `json:"task_id"`
	}
	json.Unmarshal(accepted.Body.Bytes(), &response)
	settings.Instructions = "A later edit must not change accepted work."
	w = profileHTTP(s, "PUT", "/api/v2/agent-profiles/"+profile.ID, map[string]any{"expected_revision": 1, "settings": settings})
	if w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	deadline := time.Now().Add(5 * time.Second)
	var task *agents.Task
	for time.Now().Before(deadline) {
		task, _ = s.agentManager.GetTask(response.ID)
		if task.Status == agents.TaskCompleted {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if task.Status != agents.TaskCompleted || task.Config.Profile.Revision != 1 || task.Config.Instructions != "Keep replies concise." || task.PendingInput != nil {
		t.Fatalf("pinned configuration lost: %+v", task)
	}
	w = profileHTTP(s, "POST", "/api/v2/agent-profiles/"+profile.ID+"/archive", map[string]any{"expected_revision": 2})
	if w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	retry := post()
	if retry.Code != 202 || retry.Header().Get("Location") != accepted.Header().Get("Location") {
		t.Fatal("retry did not observe canonical job", retry.Body.String())
	}
	for _, mutate := range []func(*agents.ProfileSettings){
		func(p *agents.ProfileSettings) { p.Model = "not-installed" },
		func(p *agents.ProfileSettings) { p.Knowledge = []string{"missing-document"} },
		func(p *agents.ProfileSettings) {
			p.Tools = []agents.ProfileToolReference{{Provider: "mcp:forged", Tool: d.Name, SchemaRevision: d.Revision, Location: "remote"}}
		},
	} {
		invalid := settings
		mutate(&invalid)
		w := profileHTTP(s, "POST", "/api/v2/agent-profiles", map[string]any{"settings": invalid})
		if w.Code != 422 {
			t.Fatal(w.Code, w.Body.String())
		}
		var rejected map[string]json.RawMessage
		if json.Unmarshal(w.Body.Bytes(), &rejected) != nil || rejected["profile"] != nil || rejected["issues"] == nil {
			t.Fatal("rejected draft was presented as a saved revision", w.Body.String())
		}
	}
	if err := s.toolRegistry.DisableTool("calculator"); err != nil {
		t.Fatal(err)
	}
	w = profileHTTP(s, "GET", "/api/v2/agent-profiles/"+profile.ID+"/effective?revision=1", nil)
	var effective effectiveAgentProfile
	if w.Code != 200 || json.Unmarshal(w.Body.Bytes(), &effective) != nil || effective.Ready || len(effective.Capabilities) != 0 {
		t.Fatal("revoked tools remained effective", w.Body.String())
	}
}

func TestProfileDelegationRejectsInvalidGraphsAndPinsChildren(t *testing.T) {
	s := profileFixture(t)
	actor := s.agentActor(httptest.NewRequest("GET", "/", nil))
	save := func(id string, revision int, delegates []string, computer bool) *agents.AgentProfile {
		t.Helper()
		p, err := s.agentManager.SaveProfile(s.workspaceID, actor, id, revision, agents.ProfileSettings{
			Name: "Research", Model: "model", Instructions: "Read and summarize.", MaxIterations: 10, Delegates: delegates, Computer: computer,
		})
		if err != nil {
			t.Fatal(err)
		}
		return p
	}
	child := save("", 0, nil, false)
	parent := save("", 0, []string{child.ID}, false)
	r := httptest.NewRequest("POST", "/api/v2/jobs", nil)
	config := agents.DefaultAgentConfig()
	w := httptest.NewRecorder()
	req := jobSubmitRequest{Prompt: "Summarize", ProfileID: parent.ID, ProfileRevision: 1}
	if !s.applyJobProfile(w, r, &req, &config) {
		t.Fatal(w.Code, w.Body.String())
	}
	if config.DelegateProfiles[child.ID].Profile.Revision != 1 {
		t.Fatal("child revision not pinned")
	}
	child = save(child.ID, 1, nil, false)
	if config.DelegateProfiles[child.ID].Profile.Revision != 1 {
		t.Fatal("child edit changed admitted configuration")
	}
	assertIssue := func(profile *agents.AgentProfile, code string) {
		t.Helper()
		resolved := s.resolveProfile(r, profile)
		if resolved.Ready {
			t.Fatal("invalid delegation accepted")
		}
		for _, issue := range resolved.Issues {
			if issue.Code == code {
				return
			}
		}
		t.Fatalf("missing %s in %+v", code, resolved.Issues)
	}
	// A persisted cycle can arise from independently valid revisions; resolve
	// the entire graph again at admission instead of trusting saved settings.
	child = save(child.ID, child.Revision, []string{parent.ID}, false)
	assertIssue(parent, "delegate_unavailable")
	grandchild := save("", 0, nil, false)
	child = save(child.ID, child.Revision, []string{grandchild.ID}, false)
	assertIssue(parent, "delegate_unavailable")
	child = save(child.ID, child.Revision, nil, true)
	assertIssue(parent, "delegate_computer_denied")
	if err := s.agentManager.ArchiveProfile(s.workspaceID, actor, child.ID, child.Revision); err != nil {
		t.Fatal(err)
	}
	assertIssue(parent, "delegate_unavailable")
	// Bound unsaved API drafts too, not just the storage validator.
	draft := *parent
	draft.Settings.Delegates = nil
	for i := 0; i < 5; i++ {
		draft.Settings.Delegates = append(draft.Settings.Delegates, save("", 0, nil, false).ID)
	}
	assertIssue(&draft, "delegation_limit")
}

func TestProfileAPIOwnershipAndPermissions(t *testing.T) {
	s := profileFixture(t)
	s.config.RequireAuth = true
	store := users.NewUserStore("")
	keys := map[string]string{}
	for _, name := range []string{"alice", "bob"} {
		_, key, err := store.CreateUser(name, "test-password", users.RoleUser)
		if err != nil {
			t.Fatal(err)
		}
		keys[name] = key
	}
	middleware := users.NewMiddleware(store)
	middleware.SetRequireAuth(true)
	handler := middleware.Wrap(s.requirePermissionWhenAuthEnabled(users.PermissionChat, s.handleAgentProfiles))
	do := func(user, method, path, body string) *httptest.ResponseRecorder {
		r := httptest.NewRequest(method, path, strings.NewReader(body))
		if user != "" {
			r.Header.Set("Authorization", "Bearer "+keys[user])
		}
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, r)
		return w
	}
	body := `{"settings":{"name":"Personal","model":"model","max_iterations":5}}`
	if w := do("", "POST", "/api/v2/agent-profiles", body); w.Code != 401 {
		t.Fatal(w.Code)
	}
	w := do("alice", "POST", "/api/v2/agent-profiles", body)
	if w.Code != 201 {
		t.Fatal(w.Code, w.Body.String())
	}
	var p agents.AgentProfile
	json.Unmarshal(w.Body.Bytes(), &p)
	for _, method := range []string{http.MethodGet, http.MethodPut} {
		if w := do("bob", method, "/api/v2/agent-profiles/"+p.ID, `{"expected_revision":1,"settings":{"name":"Foreign","model":"model","max_iterations":5}}`); w.Code != 404 {
			t.Fatal(w.Code, w.Body.String())
		}
	}
	if w := do("bob", "POST", "/api/v2/agent-profiles/"+p.ID+"/archive", `{"expected_revision":1}`); w.Code != 404 {
		t.Fatal(w.Code)
	}
	if w := do("alice", "POST", "/api/v2/agent-profiles", `{"settings":{"name":"Computer","model":"model","max_iterations":5,"computer":true}}`); w.Code != 422 {
		t.Fatal("Chat permission granted computer administration", w.Code)
	}
	w = do("alice", "POST", "/api/v2/agent-profiles", `{"actor":"forged","settings":{"name":"Bad","model":"model","max_iterations":5}}`)
	if w.Code != 400 {
		t.Fatal("accepted forged owner", w.Code)
	}
}
