package server

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/takuphilchan/offgrid-llm/internal/agents"
	"github.com/takuphilchan/offgrid-llm/internal/inference"
	"github.com/takuphilchan/offgrid-llm/internal/users"
	"github.com/takuphilchan/offgrid-llm/pkg/api"
)

// Explicitly opted-in, disposable Chromium and Node utility transport. There is
// no installed desktop/browser access or model download. Fixture routes supply
// controlled agent requests; all enrollment, task continuation and companion
// transport below use the production owners and handlers.
func TestBrowserManualHostIntegration(t *testing.T) {
	if os.Getenv("OFFGRID_BROWSER_MANUAL_INTEGRATION") != "1" {
		t.Skip("set OFFGRID_BROWSER_MANUAL_INTEGRATION=1 with Node and installed test Chromium")
	}
	s := taskFirstServer(t)
	s.workspaceID, s.config.RequireAuth = "manual-host-fixture", true
	s.config.MaxContextSize = 32768
	s.userStore = users.NewUserStore("")
	actor, key, err := s.userStore.CreateUser("manual-fixture", "synthetic-fixture-password", users.RoleAdmin)
	if err != nil {
		t.Fatal(err)
	}
	gate := inference.NewLifecycleGate(1)
	release, err := gate.AcquireWithContext(context.Background(), "model", 32768, func(context.Context, string) error { return nil })
	if err != nil {
		t.Fatal(err)
	}
	release()
	s.engine, s.inferenceLifecycle = &probeEngine{mode: "good"}, gate
	var modelCalls atomic.Int32
	s.agentRunner = agents.NewRunner(s.agentManager, &browserRunTools{RunTools: agents.NewToolRegistry(), server: s}, func(ctx context.Context, _ *agents.Task, _ []api.ChatMessage, _ []api.Tool) (*api.ChatCompletionResponse, error) {
		modelCalls.Add(1)
		<-ctx.Done()
		return nil, ctx.Err()
	})
	s.agentRunner.Observer = func(task *agents.Task) {
		if task.Status == agents.TaskCancelled || task.Status == agents.TaskCompleted || task.Status == agents.TaskFailed {
			s.browserHub.StopTask(task.Actor, task.ID)
		}
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v2/system", s.handleSystemIdentity)
	mux.HandleFunc("/v1/users/me", s.handleCurrentUser)
	mux.HandleFunc("/api/v2/jobs/", s.requirePermissionWhenAuthEnabled(users.PermissionChat, s.profileJobAccess(s.handleJobRead)))
	mux.HandleFunc("/api/v2/computer/pairing", s.requirePermissionWhenAuthEnabled(users.PermissionAdmin, s.handleBrowserPairing))
	mux.HandleFunc("/api/v2/computer/sessions", s.requirePermissionWhenAuthEnabled(users.PermissionAdmin, s.handleBrowserSessions))
	mux.HandleFunc("/fixture/task", func(w http.ResponseWriter, r *http.Request) {
		var request struct {
			Session string `json:"session"`
		}
		if r.Method != "POST" || json.NewDecoder(r.Body).Decode(&request) != nil || request.Session == "" {
			http.Error(w, "invalid fixture request", 400)
			return
		}
		cfg := agents.DefaultAgentConfig()
		cfg.TaskFirst, cfg.ContextWindow = true, 32768
		cfg.ComputerSession, cfg.ComputerDriver = request.Session, "browser"
		cfg.ComputerApprovalMode, cfg.RequestedComputerApprovalMode = "full_task", "full_task"
		task, err := s.agentRunner.Create("Record one fixture effect, allow manual sign-in, then inspect the preserved account page", "model", s.agentActor(r), cfg)
		if err == nil {
			err = s.browserHub.ReserveTask(task.Actor, request.Session, task.ID)
		}
		if err != nil {
			http.Error(w, "fixture task could not attach", 409)
			return
		}
		writeJSON(w, 200, taskResponse(task))
	})
	mux.HandleFunc("/fixture/tool", func(w http.ResponseWriter, r *http.Request) {
		var request struct {
			Session, Run, ID, Tool string
			Arguments              json.RawMessage
		}
		if r.Method != "POST" || json.NewDecoder(r.Body).Decode(&request) != nil {
			http.Error(w, "invalid fixture tool", 400)
			return
		}
		ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
		defer cancel()
		result, err := s.browserHub.ExecuteAuthorized(ctx, s.agentActor(r), request.Session, request.Run, request.ID, request.Tool, request.Arguments, "auto")
		if err != nil {
			writeJSON(w, 409, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, 200, map[string]string{"result": result})
	})
	identity := users.NewMiddleware(s.userStore)
	identity.SetRequireAuth(true)
	service := httptest.NewServer(s.browserTransport(identity.Wrap(mux)))
	defer service.Close()
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	command := exec.CommandContext(ctx, "node", filepath.Join(root, "apps", "desktop", "test", "fixtures", "browser-manual-integration.mjs"))
	command.Dir = root
	for _, item := range os.Environ() {
		name, _, _ := strings.Cut(item, "=")
		if !strings.HasPrefix(name, "OFFGRID_") && name != "DEBUG" {
			command.Env = append(command.Env, item)
		}
	}
	command.Env = append(command.Env, "OFFGRID_BROWSER_MANUAL_INTEGRATION=1", "OFFGRID_FIXTURE_SERVICE="+service.URL, "OFFGRID_FIXTURE_KEY="+key, "OFFGRID_FIXTURE_DIRECTORY="+t.TempDir())
	var stdout, stderr bytes.Buffer
	command.Stdout, command.Stderr = &stdout, &stderr
	if diagnostics := os.Getenv("OFFGRID_BROWSER_MANUAL_DIAGNOSTICS"); diagnostics == "1" || diagnostics == "process" {
		command.Env = append(command.Env, "OFFGRID_BROWSER_MANUAL_DIAGNOSTICS="+diagnostics, "DEBUG=pw:browser")
		command.Stderr = io.MultiWriter(&stderr, os.Stderr)
	}
	err = command.Run()
	output := stdout.Bytes()
	if err != nil {
		t.Fatalf("composed manual browser journey: %v\n%s\n%s", err, output, stderr.Bytes())
	}
	var evidence struct {
		Run           string `json:"run"`
		Effects       int    `json:"effects"`
		Pairs         int    `json:"pairs"`
		SamePage      bool   `json:"same_page"`
		Authenticated bool   `json:"authenticated"`
		Stopped       bool   `json:"stopped"`
		StopMS        int    `json:"stop_ms"`
	}
	if err = json.Unmarshal(output, &evidence); err != nil {
		t.Fatalf("fixture evidence: %v\n%s", err, output)
	}
	task, ok := s.agentManager.GetTask(evidence.Run)
	if !ok || task.Actor != actor.ID || task.Status != agents.TaskCancelled || task.ResolvedInputID == "" || evidence.Effects != 1 || evidence.Pairs != 2 || !evidence.SamePage || !evidence.Authenticated || !evidence.Stopped || modelCalls.Load() != 1 {
		t.Fatalf("missing composed acceptance evidence: %+v task=%+v fixture model calls=%d", evidence, task, modelCalls.Load())
	}
	t.Logf("actual service/host/Chromium journey: one effect, two pairings, same tab, synthetic authenticated cookie, resolved input, confirmed Stop (%d ms); fixture model calls=%d", evidence.StopMS, modelCalls.Load())
}
