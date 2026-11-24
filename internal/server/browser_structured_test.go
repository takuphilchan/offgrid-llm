package server

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/takuphilchan/offgrid-llm/internal/agents"
	"github.com/takuphilchan/offgrid-llm/internal/capabilities"
	"github.com/takuphilchan/offgrid-llm/internal/computer"
)

func TestBrowserStructuredServiceNegotiationAndPairing(t *testing.T) {
	for _, raw := range []string{``, `,"browser_features":{"discovery":1,"workspace":1}`, `,"browser_features":{"discovery":1,"workspace":1,"structured":1}`} {
		hub := computer.NewBrowserHub()
		code, _ := hub.PairCode("alice")
		s := &Server{browserHub: hub}
		identity := httptest.NewRecorder()
		s.handleSystemIdentity(identity, httptest.NewRequest("GET", "/api/v2/system", nil))
		if !strings.Contains(identity.Body.String(), `"browser-structured-v1"`) {
			t.Fatal("missing negotiation", identity.Body.String())
		}
		body := `{"code":"` + code + `","origin":"https://example.com","protocol_version":1` + raw + `}`
		req := httptest.NewRequest("POST", "/api/v2/computer/companion/pair", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		response := httptest.NewRecorder()
		s.browserTransport(http.NotFoundHandler()).ServeHTTP(response, req)
		if response.Code != 200 {
			t.Fatal(response.Code, response.Body.String())
		}
		var result struct {
			Session computer.BrowserSession `json:"session"`
		}
		if err := json.Unmarshal(response.Body.Bytes(), &result); err != nil {
			t.Fatal(err)
		}
		if (result.Session.BrowserFeatures.Structured == 1) != strings.Contains(raw, "structured") {
			t.Fatal("feature changed", result)
		}
		if raw == "" && strings.Contains(response.Body.String(), `"structured"`) {
			t.Fatal("legacy pair shape expanded")
		}
	}
	for _, raw := range []string{
		`{"protocol_version":1,"browser_features":{"structured":1}}`,
		`{"protocol_version":1,"browser_features":{"discovery":1,"structured":1}}`,
		`{"protocol_version":2,"browser_features":{"discovery":1,"workspace":1,"structured":1}}`,
	} {
		s := &Server{browserHub: computer.NewBrowserHub()}
		req := httptest.NewRequest("POST", "/api/v2/computer/companion/pair", strings.NewReader(raw))
		req.Header.Set("Content-Type", "application/json")
		response := httptest.NewRecorder()
		s.browserTransport(http.NotFoundHandler()).ServeHTTP(response, req)
		if response.Code != 400 {
			t.Fatal("invalid pairing accepted", response.Code, response.Body.String())
		}
	}
}

func TestBrowserStructuredSchemasAndAuthorization(t *testing.T) {
	for _, structured := range []bool{false, true} {
		hub := computer.NewBrowserHub()
		code, _ := hub.PairCode("alice")
		features := computer.BrowserFeatures{Discovery: 1, Workspace: 1}
		if structured {
			features.Structured = 1
		}
		session, token, err := hub.PairBrowserFeatures(code, "https://example.com", computer.ProtocolVersion, computer.ApprovalAskEveryTime, features)
		if err != nil {
			t.Fatal(err)
		}
		manager := agents.NewManagerWithPersistence(nil, nil, nil, t.TempDir())
		registry := agents.NewToolRegistry()
		cfg := agents.DefaultAgentConfig()
		cfg.ComputerSession = session.ID
		cfg.ComputerDriver = "browser"
		cfg.ComputerApprovalMode = string(computer.ApprovalAskEveryTime)
		task, err := agents.NewRunner(manager, registry, nil).Create("Inspect browser frames", "model", "alice", cfg)
		if err != nil {
			t.Fatal(err)
		}
		adapter := &browserRunTools{RunTools: registry, server: &Server{browserHub: hub, agentManager: manager}}
		menu := map[string]bool{}
		for _, tool := range adapter.ToolsForTask(task) {
			menu[tool.Function.Name] = true
		}
		for name, args := range map[string]string{"browser_search": `{"role":"button","name":"café 語😀","text":""}`, "browser_frames": `{}`, "browser_frame_select": `{"frame_id":"frame","document_id":"doc"}`} {
			if menu[name] != structured {
				t.Fatal("tool visibility escaped negotiation", name, menu)
			}
			d, exists := browserDescriptor(name)
			if !exists || !capabilities.TrustedBrowserRead(d) {
				t.Fatal("missing read identity", name, d)
			}
			call := agents.ToolExecution{Actor: "alice", RunID: task.ID, CallID: name, ExpectedCapability: &d}
			err = adapter.Authorize(context.Background(), name, json.RawMessage(args), call)
			if (err == nil) != structured {
				t.Fatal("authorization escaped negotiation", name, err)
			}
			if action, _ := hub.Poll(token); action != nil {
				t.Fatal("read authorization prepared or dispatched input")
			}
			if !structured {
				continue
			}
			wrong := d
			wrong.Revision = "future"
			call.ExpectedCapability = &wrong
			if adapter.Authorize(context.Background(), name, json.RawMessage(args), call) == nil {
				t.Fatal("changed identity authorized")
			}
			call.ExpectedCapability = &d
			call.Actor = "bob"
			if !errors.Is(adapter.Authorize(context.Background(), name, json.RawMessage(args), call), agents.ErrTaskNotFound) {
				t.Fatal("foreign actor authorized")
			}
			call.Actor = "alice"
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			done := make(chan error, 1)
			go func() { _, e := adapter.ExecuteWithPolicy(ctx, name, json.RawMessage(args), call); done <- e }()
			var action *computer.BrowserAction
			for action == nil && ctx.Err() == nil {
				action, _ = hub.Poll(token)
				if action == nil {
					time.Sleep(time.Millisecond)
				}
			}
			if action == nil {
				cancel()
				t.Fatal("no structured dispatch")
			}
			if action.Kind != name || string(action.Arguments) != args || action.ID != task.ID+":"+name {
				t.Error("dispatch identity changed", action)
			}
			if err = hub.Reply(token, computer.BrowserReply{ID: action.ID, Error: "computer_browser_observation_failed"}); err != nil {
				t.Fatal(err)
			}
			err = <-done
			cancel()
			var failure *agents.ToolFailure
			if !errors.As(err, &failure) || !failure.BeforeEffects || failure.Code != "computer_browser_observation_failed" {
				t.Fatal("lost typed read failure", err)
			}
		}
	}
	for _, combination := range [][]bool{{}, {true}, {true, true}, {false, true, true}, {true, false, true}} {
		for _, tool := range browserToolsForVision(false, combination...) {
			if computer.BrowserStructuredTool(tool.Function.Name) {
				t.Fatal("partial negotiation expanded schema")
			}
		}
	}
	if got := len(browserToolsForVision(false, true, true, true)); got != 17 {
		t.Fatal("structured schema count", got)
	}
	if validateBrowserArguments("browser_search", json.RawMessage(`{"role":"button","name":null,"text":""}`)) == nil {
		t.Fatal("service skipped exact validation")
	}
}

func TestBrowserStructuredDescriptionsPreserveLegacyQualificationCatalog(t *testing.T) {
	legacy := map[string]string{}
	for _, tool := range browserToolsForVision(true, true, true) {
		legacy[tool.Function.Name] = tool.Function.Description
	}
	for _, tool := range browserToolsForVision(true, true, true, true) {
		switch tool.Function.Name {
		case "browser_fill":
			if !strings.Contains(tool.Function.Description, "contenteditable") || tool.Function.Description == legacy[tool.Function.Name] {
				t.Fatal("missing structured editable description")
			}
		case "browser_set_checked":
			if !strings.Contains(tool.Function.Description, "consequential") || !strings.Contains(tool.Function.Description, "ARIA") {
				t.Fatal("missing custom-control policy")
			}
		case "browser_capture":
			if !strings.Contains(tool.Function.Description, "Closed shadow") || !strings.Contains(tool.Function.Description, "nested frames") {
				t.Fatal("missing capture limitations")
			}
		default:
			if !computer.BrowserStructuredTool(tool.Function.Name) && tool.Function.Description != legacy[tool.Function.Name] {
				t.Fatal("unrelated description changed")
			}
		}
	}
	for _, tool := range browserToolsForVision(true, true, true) {
		if tool.Function.Description != legacy[tool.Function.Name] {
			t.Fatal("structured presentation mutated legacy schema")
		}
	}
}
