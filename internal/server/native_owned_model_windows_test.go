//go:build windows && amd64

package server

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/takuphilchan/offgrid-llm/internal/agents"
	"github.com/takuphilchan/offgrid-llm/internal/capabilities"
	"github.com/takuphilchan/offgrid-llm/internal/computer"
	"github.com/takuphilchan/offgrid-llm/pkg/api"
)

// Explicit opt-in legacy selected-window diagnostic, NOT the desktop-session
// Full access path. Each standalone invocation has its own native consent.
// Public research uses a newly created signed-out Edge profile. No personal
// service jobs, browser profiles or documents are used. A pass is NOT general
// model/platform qualification.
func TestNativeOwnedEdgeSelectedWindowResearch(t *testing.T) {
	if os.Getenv("OFFGRID_TEST_OWNED_EDGE") != "public-tourism-with-local-consent" {
		t.Skip("requires explicit public tourism/native consent test scope")
	}
	service, model := os.Getenv("OFFGRID_TEST_COMPLETION_URL"), os.Getenv("OFFGRID_TEST_RUNTIME_MODEL")
	helper, _ := filepath.Abs("../../scripts/test/test-owned-edge-companion.mjs")
	if service == "" || model == "" {
		t.Fatal("missing explicitly configured test runtime")
	}
	s := taskFirstServer(t)
	endpoint := httptest.NewServer(s.browserTransport(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v2/system" {
			http.NotFound(w, r)
			return
		}
		writeJSON(w, 200, map[string]any{"product": "offgrid", "api_version": 2, "workspace_id": "owned-edge-fixture", "capabilities": []string{"native-computer-sessions-v2"}})
	})))
	defer endpoint.Close()
	code, err := s.browserHub.PairCode("local-admin")
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, "node", helper)
	cmd.Env = append(os.Environ(), "OFFGRID_FIXTURE_SERVICE="+endpoint.URL, "OFFGRID_FIXTURE_PAIR="+code, "OFFGRID_FIXTURE_STATE="+t.TempDir())
	cmd.Dir, _ = filepath.Abs("../..")
	output, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	input, err := cmd.StdinPipe()
	if err != nil {
		t.Fatal(err)
	}
	cmd.Stderr = os.Stderr
	if err = cmd.Start(); err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	ready := make(chan string, 1)
	go func() {
		scan := bufio.NewScanner(output)
		for scan.Scan() {
			var message struct {
				Session string `json:"session"`
				State   string `json:"state"`
				Error   string `json:"error"`
			}
			if json.Unmarshal(scan.Bytes(), &message) == nil {
				if message.Session != "" {
					select {
					case ready <- message.Session:
					default:
					}
				}
				t.Logf("companion state=%s error=%s", message.State, message.Error)
			}
		}
		done <- cmd.Wait()
	}()
	defer func() {
		_ = input.Close() // graceful local stop before profile process shutdown
		select {
		case <-done:
		case <-time.After(10 * time.Second):
			cancel()
			<-done
		}
	}()
	var session string
	select {
	case session = <-ready:
	case err = <-done:
		done <- err
		t.Fatalf("test companion failed: %v", err)
	case <-ctx.Done():
		t.Fatal(ctx.Err())
	}
	registry := agents.NewToolRegistry()
	registry.SetCapabilityBroker(capabilities.NewBroker(nil))
	tools := &browserRunTools{RunTools: registry, server: s}
	runner := agents.NewRunner(s.agentManager, tools, func(ctx context.Context, task *agents.Task, messages []api.ChatMessage, offered []api.Tool) (*api.ChatCompletionResponse, error) {
		parallel, zero, max := false, float32(0), 768
		body, err := json.Marshal(&api.ChatCompletionRequest{Model: model, Messages: messages, Tools: offered, ParallelToolCalls: &parallel, Temperature: &zero, MaxTokens: &max})
		if err != nil {
			return nil, err
		}
		req, err := http.NewRequestWithContext(ctx, "POST", service, bytes.NewReader(body))
		if err != nil {
			return nil, err
		}
		req.Header.Set("Content-Type", "application/json")
		response, err := http.DefaultClient.Do(req)
		if err != nil {
			return nil, err
		}
		defer response.Body.Close()
		if response.StatusCode != 200 {
			t.Errorf("model HTTP %d", response.StatusCode)
			return nil, context.Canceled
		}
		var result api.ChatCompletionResponse
		err = json.NewDecoder(response.Body).Decode(&result)
		for _, choice := range result.Choices {
			for _, call := range choice.Message.ToolCalls {
				t.Logf("model proposed %s %s", call.Function.Name, call.Function.Arguments)
			}
		}
		return &result, err
	})
	defer runner.Shutdown(context.Background())
	cfg := agents.DefaultAgentConfig()
	cfg.TaskFirst, cfg.SystemPrompt = true, taskFirstPrompt
	cfg.Profile = &agents.AgentProfile{ID: "owned-edge-fixture", Actor: "local-admin", Workspace: "owned-edge-fixture", Revision: 1, Settings: agents.ProfileSettings{Name: "Owned Edge test", Model: model, Computer: true, MaxIterations: 30}}
	cfg.ComputerSession, cfg.ComputerDriver, cfg.ComputerApprovalMode = session, "windows-uia", "full_task"
	cfg.ContextWindow, cfg.MaxIterations, cfg.MaxTokens = 8192, 30, 768
	configureComputerTask(&cfg)
	task, err := runner.Create("In this temporary signed-out Microsoft Edge window, search for tourism in Zimbabwe. Read three useful public sources and summarize their findings with links. Do not sign in, submit forms, download files, change profiles or open other applications. Only the address/search bar may receive text or Enter; public source navigation must use that bar. If a profile, login or permission prompt appears, stop and ask me.", model, "local-admin", cfg)
	if err != nil {
		t.Fatal(err)
	}
	if err = s.browserHub.Reserve(task.Actor, session, task.ID); err != nil {
		t.Fatal(err)
	}
	result, err := runner.Continue(ctx, task.ID, task.Actor, "start", "", false)
	if err != nil {
		t.Fatal(err)
	}
	for _, step := range result.Steps {
		t.Logf("step %d %s %s", step.ID, step.Type, step.ToolName)
	}
	t.Logf("final status=%s error=%s answer=%s", result.Status, result.Error, result.Result)
	if result.Status != agents.TaskCompleted {
		t.Fatalf("native flow did not complete: %s %s", result.Status, result.Error)
	}
	// Completion alone is insufficient. The native completion validator must
	// accept the independent reads; a human still reviews source quality.
	if err = validateNativeCompletion(result); err != nil {
		t.Fatal(err)
	}
	// Independent minimum oracle: real search URL and three source pages with
	// substantive captured text, with their observed URLs cited in the answer.
	// Model prose and successful typing alone cannot satisfy this test.
	searched := false
	sources := map[string]bool{}
	for _, step := range result.Steps {
		if step.Type != "action" || step.ToolName != "computer_observe" {
			continue
		}
		var view computer.NativeView
		if json.Unmarshal([]byte(step.ToolResult), &view) != nil {
			continue
		}
		var address string
		textBytes := 0
		for _, e := range view.Elements {
			if e.Text == nil {
				continue
			}
			if e.Writable && strings.Contains(strings.ToLower(e.Name), "address") {
				address = *e.Text
			} else {
				textBytes += len(*e.Text)
			}
		}
		u, err := url.Parse(address)
		if err != nil || u.Scheme != "https" {
			continue
		}
		if u.Hostname() == "www.bing.com" || u.Hostname() == "www.google.com" {
			query := strings.ToLower(u.Query().Get("q"))
			searched = searched || strings.Contains(query, "tourism") && strings.Contains(query, "zimbabwe")
		} else if textBytes >= 120 && strings.Contains(result.Result, address) {
			sources[address] = true
		}
	}
	if !searched || len(sources) < 3 {
		t.Fatalf("missing independent research evidence: search=%v cited observed source pages=%d", searched, len(sources))
	}
}
