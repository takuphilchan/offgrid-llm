package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/takuphilchan/offgrid-llm/internal/agents"
	"github.com/takuphilchan/offgrid-llm/internal/capabilities"
	"github.com/takuphilchan/offgrid-llm/internal/computer"
	"github.com/takuphilchan/offgrid-llm/internal/inference"
	"github.com/takuphilchan/offgrid-llm/pkg/api"
)

func TestNativePairingHTTPDoesNotUpgradeBrowserGrantOrAcceptRenderer(t *testing.T) {
	for _, mode := range []string{"native", "browser-version", "renderer", "mixed"} {
		t.Run(mode, func(t *testing.T) {
			hub := computer.NewBrowserHub()
			code, _ := hub.PairCode("local-admin")
			s := &Server{browserHub: hub}
			body := map[string]any{"code": code, "protocol_version": 2, "title": "Editor", "target": computer.TargetIdentity{ID: "window", OSSession: "os", ProcessGeneration: "generation", Surface: "surface", Driver: "windows-uia"}}
			if mode == "browser-version" {
				body["protocol_version"] = 1
			}
			if mode == "mixed" {
				body["origin"] = "https://example.com"
			}
			encoded, _ := json.Marshal(body)
			r := httptest.NewRequest(http.MethodPost, "/api/v2/computer/companion/pair", strings.NewReader(string(encoded)))
			r.Header.Set("Content-Type", "application/json")
			if mode == "renderer" {
				r.Header.Set("Origin", "http://127.0.0.1:11611")
			}
			w := httptest.NewRecorder()
			s.browserTransport(http.NotFoundHandler()).ServeHTTP(w, r)
			if (w.Code == 200) != (mode == "native") {
				t.Fatalf("unexpected pairing status %d", w.Code)
			}
			if mode == "native" {
				w = httptest.NewRecorder()
				s.handleComputerCapabilities(w, httptest.NewRequest(http.MethodGet, "/api/v2/computer/capabilities", nil))
				var capability computer.Capabilities
				if err := json.Unmarshal(w.Body.Bytes(), &capability); err != nil {
					t.Fatal(err)
				}
				if capability.ProtocolVersion != 2 || capability.ReasonCode != "native_development" {
					t.Fatalf("incorrect native status: %+v", capability)
				}
				for _, driver := range capability.Drivers {
					if driver.Qualified || driver.Available != (driver.ID == "windows-uia") {
						t.Fatalf("invented driver: %+v", driver)
					}
				}
			}
		})
	}
}

type nativeProbeEngine struct {
	inference.Engine
	forged bool
	calls  int
}

func (e *nativeProbeEngine) ChatCompletion(_ context.Context, request *api.ChatCompletionRequest) (*api.ChatCompletionResponse, error) {
	e.calls++
	if len(request.Tools) != 7 || request.Tools[0].Function.Name != "computer_observe" {
		return nil, errors.New("browser tools leaked into native check")
	}
	name, args := "computer_observe", "{}"
	if e.calls == 2 {
		var view struct {
			Requested   string `json:"requested_text"`
			Observation struct {
				ID string `json:"id"`
			} `json:"observation"`
			Elements []struct {
				ID string `json:"id"`
			} `json:"elements"`
		}
		if err := json.Unmarshal([]byte(request.Messages[len(request.Messages)-1].StringContent()), &view); err != nil {
			return nil, err
		}
		if e.forged {
			view.Elements[0].ID = "invented"
		}
		encoded, _ := json.Marshal(map[string]string{"text": view.Requested, "observation_id": view.Observation.ID, "element": view.Elements[0].ID})
		args = string(encoded)
		name = "computer_replace_text"
	}
	return &api.ChatCompletionResponse{Choices: []api.ChatCompletionChoice{{FinishReason: "tool_calls", Message: api.ChatMessage{Role: "assistant", ToolCalls: []api.ToolCall{{ID: "probe", Type: "function", Function: api.FunctionCall{Name: name, Arguments: args}}}}}}}, nil
}
func TestNativeModelProbeUsesNativeObservedIDs(t *testing.T) {
	for _, forged := range []bool{false, true} {
		engine := &nativeProbeEngine{forged: forged}
		err := probeComputerToolsDriver(context.Background(), engine, "fixture", "windows-uia")
		if (err != nil) != forged || engine.calls != 2 {
			t.Fatalf("forged=%v calls=%d err=%v", forged, engine.calls, err)
		}
	}
}

func TestNativeShortcutArgumentsAreClosedAndBounded(t *testing.T) {
	for _, shortcut := range []string{"save", "enter", "escape", "tab", "reverse_tab", "left", "right", "up", "down", "page_up", "page_down", "home", "end"} {
		if err := validateNativeArguments("computer_shortcut", json.RawMessage(fmt.Sprintf(`{"observation_id":"view","element":"field","shortcut":%q}`, shortcut))); err != nil {
			t.Fatalf("%s: %v", shortcut, err)
		}
	}
	for _, args := range []string{`{"observation_id":"view","element":"field","shortcut":"alt_f4"}`, `{"observation_id":"view","element":"field","shortcut":"save","keys":"arbitrary"}`, `{"observation_id":"view","shortcut":"save"}`} {
		if validateNativeArguments("computer_shortcut", json.RawMessage(args)) == nil {
			t.Fatalf("accepted %s", args)
		}
	}
}

func TestNativeCheckedArgumentsRequireRealBoolean(t *testing.T) {
	for _, name := range []string{"computer_set_checked", "computer_verify_checked"} {
		if err := validateNativeArguments(name, json.RawMessage(`{"observation_id":"view","element":"check","checked":true}`)); err != nil {
			t.Fatal(err)
		}
		for _, args := range []string{`{"observation_id":"view","element":"check","checked":"true"}`, `{"observation_id":"view","element":"check"}`, `{"observation_id":"view","element":"check","checked":true,"toggle":true}`} {
			if validateNativeArguments(name, json.RawMessage(args)) == nil {
				t.Fatalf("accepted %s", args)
			}
		}
	}
}

func TestNativeCompletionRequiresMatchingIndependentFreshRead(t *testing.T) {
	target := computer.TargetIdentity{ID: "window", OSSession: "os", ProcessGeneration: "generation", Surface: "surface", Driver: "windows-uia"}
	text := "Zimbabwe research — final (2026)"
	view := computer.NativeView{Observation: computer.ControlObservation{ID: "view", Target: target}, Elements: []computer.NativeElement{{ID: "field", Text: &text}}}
	for _, mode := range []string{"valid", "false", "missing-observe", "changed-target", "changed-text", "stale-observation", "unknown-element", "click-only"} {
		t.Run(mode, func(t *testing.T) {
			result := map[string]any{"verified": true, "check": "control_text_equals", "element": "field", "text": text, "observation_id": "view", "target": target}
			switch mode {
			case "false":
				result["verified"] = false
			case "changed-target":
				changed := target
				changed.ProcessGeneration = "replacement"
				result["target"] = changed
			case "changed-text":
				result["text"] = "unrelated"
			case "stale-observation":
				result["observation_id"] = "old"
			case "unknown-element":
				result["element"] = "invented"
			}
			observed, _ := json.Marshal(view)
			encoded, _ := json.Marshal(result)
			args, _ := json.Marshal(map[string]string{"element": "field", "text": text, "observation_id": "view"})
			task := &agents.Task{Config: agents.AgentConfig{ComputerDriver: target.Driver}, Steps: []agents.Step{{ToolName: "computer_observe", ToolResult: string(observed)}, {ToolName: "computer_verify", ToolArgs: string(args), ToolResult: string(encoded)}}}
			if mode == "missing-observe" {
				task.Steps = task.Steps[1:]
			} else if mode == "click-only" {
				task.Steps[1].ToolName = "computer_activate"
			}
			if (validateNativeCompletion(task) == nil) != (mode == "valid") {
				t.Fatal("incorrect completion acceptance")
			}
		})
	}
}

func TestNativeCheckedCompletionRequiresFreshMatchingState(t *testing.T) {
	target := computer.TargetIdentity{ID: "window", OSSession: "os", ProcessGeneration: "generation", Surface: "surface", Driver: "windows-uia"}
	checked := true
	view := computer.NativeView{Observation: computer.ControlObservation{ID: "view", Target: target}, Elements: []computer.NativeElement{{ID: "check", Checkable: true, Checked: &checked}}}
	observed, _ := json.Marshal(view)
	args, _ := json.Marshal(map[string]any{"element": "check", "checked": true, "observation_id": "view"})
	result, _ := json.Marshal(map[string]any{"verified": true, "check": "control_checked_equals", "element": "check", "checked": true, "observation_id": "view", "target": target})
	task := &agents.Task{Config: agents.AgentConfig{ComputerDriver: target.Driver}, Steps: []agents.Step{{ToolName: "computer_observe", ToolResult: string(observed)}, {ToolName: "computer_verify_checked", ToolArgs: string(args), ToolResult: string(result)}}}
	if err := validateNativeCompletion(task); err != nil {
		t.Fatal(err)
	}
	task.Steps[1].ToolArgs = `{"element":"check","checked":false,"observation_id":"view"}`
	if validateNativeCompletion(task) == nil {
		t.Fatal("mismatched checkbox verification accepted")
	}
}

func TestNativeRunnerRequiresDurableApprovalAndDoesNotClaimClickCompletion(t *testing.T) {
	hub := computer.NewBrowserHub()
	code, _ := hub.PairCode("alice")
	target := computer.TargetIdentity{ID: "window", OSSession: "os", ProcessGeneration: "generation", Surface: "surface", Driver: "windows-uia"}
	session, token, err := hub.PairNative(code, "Owned editor", target, 2)
	if err != nil {
		t.Fatal(err)
	}
	manager := agents.NewManagerWithPersistence(nil, nil, nil, t.TempDir())
	registry := agents.NewToolRegistry()
	registry.SetCapabilityBroker(capabilities.NewBroker(nil))
	s := &Server{browserHub: hub, agentManager: manager}
	tools := &browserRunTools{RunTools: registry, server: s}
	caller := func(_ context.Context, _ *agents.Task, messages []api.ChatMessage, available []api.Tool) (*api.ChatCompletionResponse, error) {
		if len(available) != 7 || available[0].Function.Name != "computer_observe" {
			t.Fatal("native tools missing")
		}
		name, args := "computer_observe", "{}"
		if len(messages) > 2 {
			name = "computer_replace_text"
			args = `{"observation_id":"view","element":"field","text":"Hello"}`
		}
		return &api.ChatCompletionResponse{Choices: []api.ChatCompletionChoice{{FinishReason: "tool_calls", Message: api.ChatMessage{Role: "assistant", ToolCalls: []api.ToolCall{{ID: "model-call", Type: "function", Function: api.FunctionCall{Name: name, Arguments: args}}}}}}}, nil
	}
	runner := agents.NewRunner(manager, tools, caller)
	config := agents.DefaultAgentConfig()
	config.ComputerSession = session.ID
	config.ComputerDriver = target.Driver
	configureComputerTask(&config)
	task, err := runner.Create("Edit the selected field", "model", "alice", config)
	if err != nil {
		t.Fatal(err)
	}
	if err := hub.Reserve("alice", session.ID, task.ID); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	dispatched := make(chan error, 1)
	go func() {
		for ctx.Err() == nil {
			a, err := hub.Poll(token)
			if err != nil {
				dispatched <- err
				return
			}
			if a != nil {
				if a.Kind != "computer_observe" || a.Binding == nil || a.Binding.Task != task.ID || a.Binding.Target != target {
					dispatched <- errors.New("invalid native task binding")
					return
				}
				if err := hub.Reply(token, computer.BrowserReply{ID: a.ID, Result: `{"observation":{"id":"view"},"elements":[{"id":"field","writable":true}]}`}); err != nil {
					dispatched <- err
					return
				}
				for ctx.Err() == nil {
					prepared, err := hub.Poll(token)
					if err != nil {
						dispatched <- err
						return
					}
					if prepared != nil && prepared.Kind == "computer_prepare" {
						dispatched <- hub.Reply(token, computer.BrowserReply{ID: prepared.ID, Result: `{"prepared":true}`})
						return
					}
					time.Sleep(time.Millisecond)
				}
				dispatched <- ctx.Err()
				return
			}
			time.Sleep(time.Millisecond)
		}
		dispatched <- ctx.Err()
	}()
	result, err := runner.Continue(ctx, task.ID, "alice", "start", "", false)
	if err != nil {
		t.Fatal(err)
	}
	if err := <-dispatched; err != nil {
		t.Fatal(err)
	}
	if result.Status != agents.TaskWaiting || result.PendingApproval == nil || result.PendingApproval.Tool != "computer_replace_text" {
		t.Fatalf("missing durable approval: %s", result.Status)
	}
	if action, err := hub.Poll(token); err != nil || action != nil {
		t.Fatal("mutation dispatched before approval")
	}
	if tools.ValidateCompletion(result) == nil {
		t.Fatal("unverified task reported complete")
	}
	descriptor, _ := tools.Capability("computer_replace_text")
	for _, name := range []string{"browser_fill", "shell", "computer_activate"} {
		if err := tools.Authorize(ctx, name, json.RawMessage(`{}`), agents.ToolExecution{Actor: "alice", RunID: task.ID, Approved: true, ExpectedCapability: &descriptor}); err == nil {
			t.Fatalf("accepted alternate tool %s", name)
		}
	}
}
