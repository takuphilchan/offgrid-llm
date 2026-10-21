package server

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/takuphilchan/offgrid-llm/internal/agents"
	"github.com/takuphilchan/offgrid-llm/internal/users"
	"github.com/takuphilchan/offgrid-llm/pkg/api"
)

// Opt-in because this exercises actual CLI, Python and headless Chromium
// processes. All stores, credentials, inference and HTTP endpoints are fixtures;
// it neither contacts an installed service nor opens a personal desktop app.
func TestAgentCrossClientIntegration(t *testing.T) {
	if os.Getenv("OFFGRID_CROSS_CLIENT_TESTS") != "1" {
		t.Skip("set OFFGRID_CROSS_CLIENT_TESTS=1 with Go, Python and Playwright Chromium installed")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	temp := t.TempDir()
	binary := filepath.Join(temp, "offgrid")
	if runtime.GOOS == "windows" {
		binary += ".exe"
	}
	build := exec.CommandContext(ctx, "go", "build", "-o", binary, "./cmd/offgrid")
	build.Dir = root
	if data, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build CLI: %v\n%s", err, data)
	}
	python := "python3"
	if runtime.GOOS == "windows" {
		python = "python"
	}
	if _, err := exec.LookPath(python); err != nil {
		t.Fatal("Python is required for this opt-in test", err)
	}
	f := newConversationFixture(t)
	s := f.s
	s.config.RequireAuth = true
	s.userStore = users.NewUserStore("")
	alice, aliceKey, err := s.userStore.CreateUser("cross-alice", "fixture-password", users.RoleUser)
	if err != nil {
		t.Fatal(err)
	}
	_, bobKey, err := s.userStore.CreateUser("cross-bob", "fixture-password", users.RoleUser)
	if err != nil {
		t.Fatal(err)
	}
	profile, err := s.agentManager.SaveProfile(s.workspaceID, alice.ID, "", 0, agents.ProfileSettings{Name: "Cross-client", Model: "model", MaxIterations: 5})
	if err != nil {
		t.Fatal(err)
	}
	started := make(chan struct{})
	var modelCalls atomic.Int32
	s.agentRunner = agents.NewRunner(s.agentManager, s.toolRegistry, func(ctx context.Context, task *agents.Task, _ []api.ChatMessage, _ []api.Tool) (*api.ChatCompletionResponse, error) {
		if task.AdmissionHeld || task.Origin == nil {
			t.Error("unlinked inference")
		}
		if modelCalls.Add(1) == 1 {
			close(started)
		}
		<-ctx.Done()
		return nil, ctx.Err()
	})
	s.agentRunner.Observer = s.projectConversationTask
	t.Cleanup(func() { _ = s.agentRunner.Shutdown(context.Background()) })
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v2/system", s.handleSystemIdentity)
	mux.HandleFunc("/api/v2/jobs/", s.requirePermissionWhenAuthEnabled(users.PermissionChat, s.profileJobAccess(s.handleJobRead)))
	mux.HandleFunc("/v1/sessions", s.requirePermissionWhenAuthEnabled(users.PermissionChat, s.sessionHandlers.HandleSessions))
	mux.HandleFunc("/v1/sessions/", s.requirePermissionWhenAuthEnabled(users.PermissionChat, s.sessionHandlers.HandleSessions))
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte("<!doctype html><title>Disposable cross-client fixture</title>"))
	})
	identity := users.NewMiddleware(s.userStore)
	identity.SetRequireAuth(true)
	server := httptest.NewServer(identity.Wrap(mux))
	defer server.Close()
	env := []string{}
	for _, pair := range os.Environ() {
		key, _, _ := strings.Cut(pair, "=")
		if strings.HasPrefix(key, "OFFGRID_") || key == "PYTHONPATH" {
			continue
		}
		env = append(env, pair)
	}
	env = append(env, "OFFGRID_SERVER_URL="+server.URL, "OFFGRID_API_KEY="+aliceKey, "OFFGRID_DATA_DIR="+filepath.Join(temp, "data"), "OFFGRID_MODELS_DIR="+filepath.Join(temp, "models"), "OFFGRID_BIN_DIR="+filepath.Join(temp, "bin"), "OFFGRID_CONFIG="+filepath.Join(temp, "config.yaml"), "PYTHONPATH="+filepath.Join(root, "sdk", "python"))
	run := func(name string, args ...string) map[string]any {
		t.Helper()
		command := exec.CommandContext(ctx, name, args...)
		command.Dir, command.Env = temp, env
		data, err := command.CombinedOutput()
		if err != nil {
			t.Fatalf("fixture client %s failed: %v\n%s", filepath.Base(name), err, data)
		}
		var object map[string]any
		if err = json.Unmarshal(data, &object); err != nil {
			t.Fatalf("client JSON: %v\n%s", err, data)
		}
		return object
	}
	run(binary, "session", "create", "cross-client", "--model", "model", "--json")
	saved := run(python, "-c", `import json, os, sys
from offgrid import Client
c = Client(host=os.environ['OFFGRID_SERVER_URL'], api_key=os.environ['OFFGRID_API_KEY'])
print(json.dumps(c.sessions.submit_agent_turn('cross-client', 'Explain the fixture; do not use tools.', request_id='cross-client-turn', profile_id=sys.argv[1], profile_revision=1)))`, profile.ID)
	job, ok := saved["job"].(map[string]any)
	if !ok {
		t.Fatal("SDK lost canonical job", saved)
	}
	id, ok := job["run_id"].(string)
	if !ok || id == "" {
		t.Fatal(saved)
	}
	select {
	case <-started:
	case <-ctx.Done():
		t.Fatal("job did not start")
	}
	browser := run("node", filepath.Join(root, "dev", "scripts", "test-agent-cross-clients.mjs"), server.URL, id, bobKey)
	if browser["job_id"] != id || browser["steered"] != true {
		t.Fatal(browser)
	}
	status := run(binary, "agent", "status", id, "--json")
	if status["run_id"] != id || status["status"] != "running" {
		t.Fatal("CLI observed different work", status)
	}
	run(binary, "agent", "cancel", id, "--json")
	observed := run(python, "-c", `import json, os, sys, time
from offgrid import Client
c = Client(host=os.environ['OFFGRID_SERVER_URL'], api_key=os.environ['OFFGRID_API_KEY'])
for _ in range(100):
    turn = c.sessions.turn('cross-client')
    if turn['job']['status'] == 'cancelled': break
    time.sleep(.02)
assert turn['job']['run_id'] == sys.argv[1]
assert turn['job']['status'] == 'cancelled'
print(json.dumps(turn['job']))`, id)
	if observed["run_id"] != id || modelCalls.Load() != 1 {
		t.Fatal("duplicate inference across clients", observed, modelCalls.Load())
	}
	jobs := s.agentManager.ListTasks()
	if len(jobs) != 1 || jobs[0].ID != id {
		t.Fatal("multiple canonical jobs", jobs)
	}
}
