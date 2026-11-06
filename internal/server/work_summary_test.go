package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/takuphilchan/offgrid-llm/internal/agents"
	"github.com/takuphilchan/offgrid-llm/internal/users"
)

func TestWorkSummaryAuthorizationBeforeCounting(t *testing.T) {
	s := profileFixture(t)
	s.config.RequireAuth = true
	s.userStore = users.NewUserStore("")
	alice, key, err := s.userStore.CreateUser("summary-alice", "fixture-password", users.RoleUser)
	if err != nil {
		t.Fatal(err)
	}
	_, foreignKey, err := s.userStore.CreateUser("summary-bob", "fixture-password", users.RoleUser)
	if err != nil {
		t.Fatal(err)
	}
	profile, err := s.agentManager.SaveProfile(s.workspaceID, alice.ID, "", 0, agents.ProfileSettings{Name: "Assistant", Model: "model", MaxIterations: 5})
	if err != nil {
		t.Fatal(err)
	}
	config := agents.DefaultAgentConfig()
	config.Profile = profile
	owned, err := s.agentRunner.Create("Owned profile task", "model", alice.ID, config)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.agentRunner.Create("Legacy private task", "model", alice.ID, agents.DefaultAgentConfig()); err != nil {
		t.Fatal(err)
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v2/work", s.requirePermissionWhenAuthEnabled(users.PermissionChat, s.handleWorkSummary))
	middleware := users.NewMiddleware(s.userStore)
	middleware.SetRequireAuth(true)
	handler := middleware.Wrap(mux)
	for _, tc := range []struct {
		key           string
		count, status int
	}{{key, 1, 200}, {foreignKey, 0, 200}, {"", 0, 401}} {
		req := httptest.NewRequest("GET", "/api/v2/work", nil)
		if tc.key != "" {
			req.Header.Set("Authorization", "Bearer "+tc.key)
		}
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)
		if w.Code != tc.status {
			t.Fatalf("status %d: %s", w.Code, w.Body.String())
		}
		if tc.status != 200 {
			continue
		}
		var page workPage
		if err = json.Unmarshal(w.Body.Bytes(), &page); err != nil {
			t.Fatal(err)
		}
		if page.Counts["all"] != tc.count || len(page.Items) != tc.count {
			t.Fatalf("foreign counts leaked: %+v", page)
		}
		if tc.count == 1 && page.Items[0].ID != owned.ID {
			t.Fatal("wrong owned job")
		}
		if strings.Contains(w.Body.String(), "Legacy private task") {
			t.Fatal("legacy authority expanded")
		}
	}
}

func TestWorkSummaryPaginationCountsAndDeletedRecords(t *testing.T) {
	m := agents.NewManager(nil, nil, nil)
	s := &Server{agentManager: m}
	for i := 0; i < 1005; i++ {
		id := fmt.Sprintf("run-%032d", i)
		if _, err := m.CreateTask(id, fmt.Sprintf("Task %d", i), nil); err != nil {
			t.Fatal(err)
		}
		if i < 1000 {
			if err := m.CompleteTask(id, strings.Repeat("private-output", 1000), nil); err != nil {
				t.Fatal(err)
			}
		}
	}
	read := func(path string) workPage {
		t.Helper()
		w := httptest.NewRecorder()
		s.handleWorkSummary(w, httptest.NewRequest("GET", path, nil))
		if w.Code != 200 {
			t.Fatalf("%d %s", w.Code, w.Body.String())
		}
		var p workPage
		if err := json.Unmarshal(w.Body.Bytes(), &p); err != nil {
			t.Fatal(err)
		}
		if strings.Contains(w.Body.String(), "private-output") {
			t.Fatal("execution content exposed")
		}
		return p
	}
	p := read("/api/v2/work?bucket=history&limit=17")
	if len(p.Items) != 17 || p.Counts["history"] != 1000 || p.Counts["running"] != 5 || p.RemovableCount != 1000 || !p.HasMore {
		t.Fatalf("wrong page: %+v", p)
	}
	seen := map[string]bool{}
	for {
		for _, item := range p.Items {
			if seen[item.ID] {
				t.Fatal("duplicate pagination entry")
			}
			seen[item.ID] = true
		}
		if !p.HasMore {
			break
		}
		p = read("/api/v2/work?bucket=history&limit=17&cursor=" + url.QueryEscape(p.NextCursor))
	}
	if len(seen) != 1000 {
		t.Fatalf("missing history: %d", len(seen))
	}
	if err := m.DeleteTask("run-00000000000000000000000000000000"); err != nil {
		t.Fatal(err)
	}
	p = read("/api/v2/work?bucket=history&query=Task%200")
	if len(p.Items) != 0 || p.Counts["history"] != 999 || p.RemovableCount != 0 {
		t.Fatal("deleted work returned")
	}
}

func TestWorkSummaryRejectsInvalidQueriesAndUnavailableSources(t *testing.T) {
	m := agents.NewManager(nil, nil, nil)
	s := &Server{agentManager: m}
	for _, cfg := range []*agents.AgentConfig{{KnowledgeDependencies: []string{"revoked"}}, {KnowledgeContext: "unscoped private source"}} {
		if _, err := m.CreateTask(fmt.Sprintf("run-%d", len(m.ListWorkSummaries())), "Private title", cfg); err != nil {
			t.Fatal(err)
		}
	}
	w := httptest.NewRecorder()
	s.handleWorkSummary(w, httptest.NewRequest("GET", "/api/v2/work", nil))
	var p workPage
	_ = json.Unmarshal(w.Body.Bytes(), &p)
	if len(p.Items) != 0 || p.Counts["all"] != 0 || strings.Contains(w.Body.String(), "Private title") {
		t.Fatal("source authorization leak")
	}
	for _, query := range []string{"limit=0", "limit=101", "limit=no", "bucket=invalid", "cursor=bad", "query=" + strings.Repeat("x", 513)} {
		w = httptest.NewRecorder()
		s.handleWorkSummary(w, httptest.NewRequest("GET", "/api/v2/work?"+query, nil))
		if w.Code != 400 {
			t.Fatalf("accepted %s", query)
		}
	}
	w = httptest.NewRecorder()
	s.handleWorkSummary(w, httptest.NewRequest("POST", "/api/v2/work", nil))
	if w.Code != 405 {
		t.Fatal(w.Code)
	}
}
