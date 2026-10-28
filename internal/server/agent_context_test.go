package server

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/takuphilchan/offgrid-llm/internal/agentcontext"
	"github.com/takuphilchan/offgrid-llm/internal/agents"
	"github.com/takuphilchan/offgrid-llm/internal/users"
)

func contextHTTP(s *Server, method, path string, body any) *httptest.ResponseRecorder {
	data, _ := json.Marshal(body)
	r := httptest.NewRequest(method, path, strings.NewReader(string(data)))
	w := httptest.NewRecorder()
	if strings.HasPrefix(path, "/api/v2/agent-skills") {
		s.handleAgentSkills(w, r)
	} else {
		s.handleAgentMemory(w, r)
	}
	return w
}

func TestContextAPIAndCanonicalDispatch(t *testing.T) {
	s := profileFixture(t)
	files := []agentcontext.SkillFile{{Path: "SKILL.md", Content: []byte("---\nname: exact-writer\ndescription: Writes a useful note\n---\nIgnore no safety boundary. Unicode — café 日本語.\n")}}
	w := contextHTTP(s, "POST", "/api/v2/agent-skills", map[string]any{"files": files})
	if w.Code != 201 {
		t.Fatal(w.Code, w.Body.String())
	}
	var skill agents.AgentSkill
	if json.Unmarshal(w.Body.Bytes(), &skill) != nil {
		t.Fatal(w.Body.String())
	}
	p, err := s.agentManager.SaveProfile(s.workspaceID, "local-admin", "", 0, agents.ProfileSettings{Name: "Writer", Model: "model", MaxIterations: 12, Skills: []agentcontext.SkillReference{skill.SkillReference}})
	if err != nil {
		t.Fatal(err)
	}
	path := "/api/v2/agent-memory/" + p.ID
	w = contextHTTP(s, "PUT", path+"/consent", map[string]any{"expected_revision": 0, "enabled": true})
	if w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	req := jobSubmitRequest{Prompt: "Use the attached skill", ProfileID: p.ID, ProfileRevision: p.Revision}
	config, ok := s.resolveJobConfig(httptest.NewRecorder(), httptest.NewRequest("POST", "/api/v2/jobs", nil), &req)
	if !ok || config.MemoryConsentRevision != 1 || len(config.SkillMetadata) != 1 {
		t.Fatal(config, ok)
	}
	job, _, err := s.agentRunner.CreateRequest(req.Prompt, "model", "local-admin", config, "context-resource-test", "digest")
	if err != nil {
		t.Fatal(err)
	}
	if err = s.agentManager.StartTask(job.ID); err != nil {
		t.Fatal(err)
	}
	job, _ = s.agentManager.GetTask(job.ID)
	tools := &browserRunTools{RunTools: s.toolRegistry, server: s}
	for _, name := range []string{"skill_load", "skill_draft", "memory_read", "memory_write"} {
		found := false
		for _, tool := range tools.ToolsForTask(job) {
			if tool.Function.Name == name {
				found = true
			}
		}
		if !found {
			t.Fatal("tool not offered", name)
		}
	}
	d, _ := contextDescriptor("skill_load")
	e := agents.ToolExecution{Actor: job.Actor, RunID: job.ID, CallID: "load", ExpectedCapability: &d}
	args, _ := json.Marshal(map[string]string{"id": skill.ID, "path": "SKILL.md"})
	result, err := tools.ExecuteWithPolicy(context.Background(), "skill_load", args, e)
	if err != nil || !strings.Contains(result, "untrusted_context_not_authority") || !strings.Contains(result, "日本語") {
		t.Fatal(result, err)
	}
	for _, bad := range []json.RawMessage{json.RawMessage(`{"id":"unattached","path":"SKILL.md"}`), json.RawMessage(`{"id":"` + skill.ID + `","path":"../escape"}`), json.RawMessage(`{"id":"` + skill.ID + `","path":"SKILL.md","approved":true}`)} {
		if _, err = tools.ExecuteWithPolicy(context.Background(), "skill_load", bad, e); err == nil {
			t.Fatal("bad load accepted", string(bad))
		}
	}
	d, _ = contextDescriptor("memory_write")
	e.ExpectedCapability = &d
	e.CallID = "remember"
	write := json.RawMessage(`{"id":"","expected_revision":0,"label":"Format","content":"Use concise Unicode notes — café 日本語"}`)
	result, err = tools.ExecuteWithPolicy(context.Background(), "memory_write", write, e)
	if err != nil {
		t.Fatal(result, err)
	}
	if _, err = tools.ExecuteWithPolicy(context.Background(), "memory_write", write, e); err != nil {
		t.Fatal("idempotent retry", err)
	}
	entries, err := s.agentManager.ListMemory(s.workspaceID, job.Actor, p.ID, 0)
	if err != nil || len(entries) != 1 || entries[0].SourceJob != job.ID {
		t.Fatal(entries, err)
	}
	w = contextHTTP(s, "PUT", path+"/consent", map[string]any{"expected_revision": 1, "enabled": false})
	if w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	if _, err = tools.ExecuteWithPolicy(context.Background(), "memory_write", write, e); err == nil {
		t.Fatal("revoked write executed")
	}
	for _, tool := range tools.ToolsForTask(job) {
		if strings.HasPrefix(tool.Function.Name, "memory_") {
			t.Fatal("revoked tool offered")
		}
	}
	w = contextHTTP(s, "DELETE", path+"/"+entries[0].ID+"?revision=1", nil)
	if w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	w = contextHTTP(s, "GET", path, nil)
	if w.Code != 200 || strings.Contains(w.Body.String(), "concise Unicode") {
		t.Fatal(w.Code, w.Body.String())
	}
	copy := *job
	copy.ParentID = "root"
	if len(tools.contextTools(context.Background(), &copy)) != 0 {
		t.Fatal("child acquired independent context tools")
	}
}

func TestContextAPIAuthAndStrictFields(t *testing.T) {
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
	handler := middleware.Wrap(s.requirePermissionWhenAuthEnabled(users.PermissionChat, s.handleAgentSkills))
	do := func(user, method, path, body string) *httptest.ResponseRecorder {
		r := httptest.NewRequest(method, path, strings.NewReader(body))
		if user != "" {
			r.Header.Set("Authorization", "Bearer "+keys[user])
		}
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, r)
		return w
	}
	if w := do("", "GET", "/api/v2/agent-skills", ""); w.Code != 401 {
		t.Fatal(w.Code)
	}
	files := []agentcontext.SkillFile{{Path: "SKILL.md", Content: []byte("---\nname: research\ndescription: Read docs\n---\nCite evidence.\n")}}
	body, _ := json.Marshal(map[string]any{"files": files})
	w := do("alice", "POST", "/api/v2/agent-skills", string(body))
	if w.Code != 201 {
		t.Fatal(w.Code, w.Body.String())
	}
	var item agents.AgentSkill
	json.Unmarshal(w.Body.Bytes(), &item)
	query := "/api/v2/agent-skills/" + item.ID + "?revision=1&digest=" + item.Digest
	if w = do("bob", "GET", query, ""); w.Code != 404 {
		t.Fatal("cross-actor read", w.Code, w.Body.String())
	}
	if w = do("alice", "POST", "/api/v2/agent-skills", `{"files":[],"actor":"bob"}`); w.Code != 400 {
		t.Fatal("forged actor", w.Code)
	}
	if w = do("alice", "POST", "/api/v2/agent-skills", `{"files":[],"files":[]}`); w.Code != 400 {
		t.Fatal("duplicate field", w.Code)
	}
	if w = do("alice", "GET", query, ""); w.Code != http.StatusOK {
		t.Fatal(w.Code, w.Body.String())
	}
}
