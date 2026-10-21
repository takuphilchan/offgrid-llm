package server

// These tests exercise the real durable runner, service checks and authenticated
// companion transport against in-memory application controls. No browser, OS
// input, public network request or personal profile is available to this host.
import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/url"
	"os"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/takuphilchan/offgrid-llm/internal/agents"
	"github.com/takuphilchan/offgrid-llm/internal/computer"
	"github.com/takuphilchan/offgrid-llm/internal/inference"
	"github.com/takuphilchan/offgrid-llm/pkg/api"
)

const syntheticSearchQuery = "tourism in Zimbabwe"
const syntheticSearchURL = "https://example.test/search?q=tourism+in+Zimbabwe"

type searchActionHost struct {
	base                                         *workflowDocumentHost
	view                                         computer.NativeView
	text                                         string
	inputs, submits, observations, documentReads int
	preparedName, preparedArgs                   string
}

func (h *searchActionHost) observe() computer.NativeView {
	h.observations++
	h.view = computer.NativeView{Observation: computer.ControlObservation{ID: fmt.Sprintf("search-view-%d", h.observations), Target: h.base.binding.Target, Sequence: uint64(h.observations), CapturedAt: time.Now().UTC(), StateDigest: computer.DocumentTextDigest(h.text)}, Elements: []computer.NativeElement{{ID: "search-field", Identity: "stable-address-field", Name: "Address and search bar", Role: "edit", Text: &h.text, Writable: true}}}
	if h.submits > 0 {
		heading := "Search results for tourism in Zimbabwe"
		h.view.Elements = append(h.view.Elements, computer.NativeElement{ID: "results-heading", Identity: "results-heading", Name: heading, Role: "heading", Text: &heading})
	}
	return h.view
}

func (h *searchActionHost) validateControl(name string, raw json.RawMessage) error {
	if h.base.launches.Load() != 1 {
		return computer.ErrTargetNotSelected
	}
	if err := validateNativeArguments(name, raw); err != nil {
		return err
	}
	var args struct {
		Observation string `json:"observation_id"`
		Element     string `json:"element"`
		Text        string `json:"text,omitempty"`
		Shortcut    string `json:"shortcut,omitempty"`
	}
	if json.Unmarshal(raw, &args) != nil || args.Observation != h.view.Observation.ID || args.Element != "search-field" {
		return computer.ErrStaleObservation
	}
	if name == "computer_replace_text" && (args.Text != syntheticSearchQuery || h.inputs != 0 || h.submits != 0) {
		return computer.ErrInvalidControl
	}
	if name == "computer_shortcut" && (args.Shortcut != "enter" || h.text != syntheticSearchQuery || h.inputs != 1 || h.submits != 0) {
		return computer.ErrInvalidControl
	}
	return nil
}

func (h *searchActionHost) execute(ctx context.Context, action *computer.BrowserAction) (string, error) {
	switch action.Kind {
	case "computer_applications", "computer_open_application":
		return h.base.execute(ctx, action)
	case "computer_verify_application":
		if h.base.launches.Load() != 1 {
			return "", computer.ErrApplicationNotReady
		}
		view := h.observe()
		return documentJSON(computer.ApplicationEvidence{Verified: true, Check: "application_open", Application: "fixture-editor", Target: &h.base.binding.Target, View: &view}), nil
	case "computer_documents":
		h.documentReads++
		return "", computer.ErrDocumentUnavailable
	case "computer_observe":
		if h.base.launches.Load() != 1 {
			return "", computer.ErrTargetNotSelected
		}
		return documentJSON(h.observe()), nil
	case "computer_prepare":
		var request struct {
			Tool      string          `json:"tool"`
			Arguments json.RawMessage `json:"arguments"`
		}
		if computer.DecodeControlMessage(action.Arguments, &request) != nil {
			return "", computer.ErrInvalidControl
		}
		if request.Tool == "computer_open_application" {
			return h.base.execute(ctx, action)
		}
		if request.Tool != "computer_replace_text" && request.Tool != "computer_shortcut" {
			return "", computer.ErrInvalidControl
		}
		if err := h.validateControl(request.Tool, request.Arguments); err != nil {
			return "", err
		}
		h.preparedName, h.preparedArgs = request.Tool, string(request.Arguments)
		return `{"prepared":true,"approval_class":"reversible"}`, nil
	case "computer_replace_text", "computer_shortcut":
		if h.preparedName != action.Kind || h.preparedArgs != string(action.Arguments) {
			return "", computer.ErrControlApproval
		}
		if err := h.validateControl(action.Kind, action.Arguments); err != nil {
			return "", err
		}
		h.preparedName, h.preparedArgs = "", ""
		if action.Kind == "computer_replace_text" {
			h.inputs++
			h.text = syntheticSearchQuery
		} else {
			h.submits++
			h.text = syntheticSearchURL
		}
		return `{"actions":[{"action_id":"synthetic-search-input","outcome":"dispatched","uncertain":false}]}`, nil
	case "computer_verify":
		if err := h.validateControl(action.Kind, action.Arguments); err != nil {
			return "", err
		}
		var request struct {
			Text string `json:"text"`
		}
		_ = json.Unmarshal(action.Arguments, &request)
		return documentJSON(map[string]any{"verified": h.text == request.Text, "check": "control_text_equals", "observation_id": h.view.Observation.ID, "element": "search-field", "text": h.text, "target": h.base.binding.Target}), nil
	}
	return "", computer.ErrInvalidControl
}

func TestNativeSearchActionWorkflowRunnerCompletes(t *testing.T) {
	for _, explicit := range []bool{false, true} {
		t.Run(fmt.Sprintf("caller-check-%t", explicit), func(t *testing.T) { runSyntheticSearchActionWorkflow(t, nil, nil, explicit) })
	}
}

func runSyntheticSearchActionWorkflow(t *testing.T, model agents.RunCaller, counter func(context.Context, *agents.Task, []api.ChatMessage, []api.Tool) (int, error), preserveExplicit bool) {
	t.Helper()
	s, adapter, _, _, _ := compositionFixture(t)
	s.browserHub.RecordReply = s.recordComputerReply
	adapter.RunTools = documentFixtureRegistry{adapter.RunTools}
	sequence := []api.FunctionCall{{Name: "computer_applications", Arguments: `{}`}, {Name: applicationSearchTool}, {Name: "computer_open_application", Arguments: `{"application_id":"fixture-editor"}`}, {Name: "computer_verify_application", Arguments: `{"application_id":"fixture-editor"}`}, {Name: "computer_observe", Arguments: `{}`}, {Name: "computer_replace_text"}, {Name: "computer_observe", Arguments: `{}`}, {Name: "computer_verify"}, {Name: "computer_shortcut"}, {Name: "computer_observe", Arguments: `{}`}}
	calls := 0
	runner := agents.NewRunner(s.agentManager, adapter, func(ctx context.Context, task *agents.Task, messages []api.ChatMessage, tools []api.Tool) (*api.ChatCompletionResponse, error) {
		calls++
		if model != nil {
			return model(ctx, task, messages, tools)
		}
		if !availableNames(tools)["mcp__docs__docs"] {
			t.Fatal("MCP composition lost")
		}
		message := api.ChatMessage{Role: "assistant", Content: "The synthetic Edge search results heading is visible. This is fixture evidence, not public research."}
		finish := "stop"
		if calls <= len(sequence) {
			call := sequence[calls-1]
			if call.Name == applicationSearchTool {
				call.Arguments = latestApplicationQuery(task, "Microsoft Edge")
			}
			if call.Name == "computer_replace_text" || call.Name == "computer_verify" || call.Name == "computer_shortcut" {
				view, ok := currentNativeControlView(task)
				if !ok {
					t.Fatal("missing current control view", calls)
				}
				args := map[string]any{"observation_id": view.Observation.ID, "element": "search-field"}
				if call.Name == "computer_shortcut" {
					args["shortcut"] = "enter"
				} else {
					args["text"] = syntheticSearchQuery
				}
				call.Arguments = documentJSON(args)
			}
			if !availableNames(tools)[call.Name] {
				t.Fatalf("stage %d omitted %s: %v", calls, call.Name, availableNames(tools))
			}
			call.Arguments = translateNativeReferences(call.Name, call.Arguments, nativeModelReferences(task))
			message.Content, message.ToolCalls, finish = "", []api.ToolCall{{Type: "function", Function: call}}, "tool_calls"
		}
		return &api.ChatCompletionResponse{Choices: []api.ChatCompletionChoice{{Message: message, FinishReason: finish}}}, nil
	})
	defer runner.Shutdown(context.Background())
	runner.CountContext = func(context.Context, *agents.Task, []api.ChatMessage, []api.Tool) (int, error) { return 5200, nil }
	if counter != nil {
		runner.CountContext = counter
	}
	cfg := compositionConfig("windows-uia", true)
	cfg.ComputerSession = ""
	cfg.NativeWorkflowVersion, cfg.NativeActionContractVersion, cfg.ContextWindow, cfg.MaxIterations, cfg.MaxTokens = agents.DocumentWorkflowEvidenceVersion, agents.NativeActionContractVersion, 8192, 24, 512
	cfg.RequestedComputerScope, cfg.RequestedComputerApprovalMode = "desktop", "full_task"
	explicit := agents.OutcomeExpectation{ID: "caller-research", Title: "Public research remains unqualified", Kind: "research"}
	if preserveExplicit {
		cfg.NativeExpectations = []agents.OutcomeExpectation{explicit}
	}
	prompt := "Open Microsoft Edge. Search for exactly tourism in Zimbabwe using the address/search field, submit with Enter, then read the search-results heading. Do not follow links, sign in, download, or use personal browser profiles."
	task, err := runner.Create(prompt, "fixture-model", "alice", cfg)
	if err != nil {
		t.Fatal(err)
	}
	if task.PendingInput == nil || calls != 0 {
		t.Fatal("desktop authority was bypassed")
	}
	code, err := s.browserHub.PairTaskCode("alice", task.ID, task.PendingInput.ID, "fixture-workspace", computer.ApprovalFullTask)
	if err != nil {
		t.Fatal(err)
	}
	desktop := computer.TargetIdentity{ID: "fixture-desktop", OSSession: "fixture-os", ProcessGeneration: "fixture-desktop", Surface: "desktop", Driver: "windows-uia"}
	session, token, err := s.browserHub.PairDesktopFeatures(code, desktop, computer.ApprovalFullTask, computer.NativeFeatures{Documents: 1})
	if err != nil {
		t.Fatal(err)
	}
	if err = s.browserHub.ActivateDesktop(token); err != nil {
		t.Fatal(err)
	}
	if err = s.browserHub.ReserveTask("alice", session.ID, task.ID); err != nil {
		t.Fatal(err)
	}
	cfg.ComputerSession, cfg.ComputerScope = session.ID, "desktop"
	configureDesktopTask(&cfg)
	task, err = runner.ResolveComputerInput(task.ID, "alice", task.PendingInput.ID, cfg)
	if err != nil {
		t.Fatal(err)
	}
	target := computer.TargetIdentity{ID: "fixture-window", OSSession: desktop.OSSession, ProcessGeneration: "fixture-process", Surface: "fixture-surface", Driver: desktop.Driver}
	base := &workflowDocumentHost{backend: &workflowDocumentBackend{target: target}, binding: computer.ControlBinding{Actor: "alice", Task: task.ID, Companion: session.ID, Session: session.ID, Target: target}, desktop: true, appName: "Microsoft Edge"}
	host := &searchActionHost{base: base}
	timeout := 30 * time.Second
	if model != nil {
		timeout = 30 * time.Minute
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	done := make(chan error, 1)
	go func() {
		for ctx.Err() == nil {
			action, err := s.browserHub.Poll(token)
			if err != nil {
				done <- err
				return
			}
			if action != nil {
				result, executeErr := host.execute(ctx, action)
				reply := computer.BrowserReply{ID: action.ID, Result: result}
				if executeErr != nil {
					reply.Error = executeErr.Error()
				}
				if err := s.browserHub.Reply(token, reply); err != nil {
					done <- err
					return
				}
			}
			select {
			case <-ctx.Done():
				done <- nil
				return
			case <-time.After(time.Millisecond):
			}
		}
		done <- nil
	}()
	result, err := runner.Continue(ctx, task.ID, "alice", "start", "", false)
	cancel()
	if hostErr := <-done; hostErr != nil {
		t.Fatal(hostErr)
	}
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != agents.TaskCompleted || base.launches.Load() != 1 || host.inputs != 1 || host.submits != 1 || host.text != syntheticSearchURL || host.documentReads != 0 {
		for _, step := range result.Steps {
			t.Logf("synthetic step: %s %s %s %s", step.Type, step.ToolName, step.ToolArgs, step.ToolResult)
		}
		t.Fatalf("search flow status=%s error=%s launches=%d inputs=%d enters=%d document_reads=%d calls=%d", result.Status, result.Error, base.launches.Load(), host.inputs, host.submits, host.documentReads, calls)
	}
	derived, foundExplicit := 0, false
	for _, check := range adapter.AssessWorkflow(result) {
		if check.ActionDerivedCallID != "" {
			derived++
			if check.State != "verified" {
				t.Fatalf("navigation erased action evidence: %+v", check)
			}
		}
		if check.Expectation.ID == explicit.ID {
			foundExplicit = true
			if !reflect.DeepEqual(check.Expectation, explicit) || check.ActionDerivedCallID != "" || check.Provenance != "caller_explicit" || check.State != "unavailable" {
				t.Fatalf("caller check was replaced or falsely verified: %+v", check)
			}
		}
	}
	if derived != 2 || foundExplicit != preserveExplicit {
		t.Fatalf("wrong check set: derived=%d explicit=%t", derived, foundExplicit)
	}
	resultObserved, entered := false, false
	for _, step := range result.Steps {
		if step.Type != "action" {
			continue
		}
		if step.ToolName == "computer_shortcut" {
			entered = true
		}
		if entered && step.ToolName == "computer_observe" && strings.Contains(step.ToolResult, syntheticSearchURL) && strings.Contains(step.ToolResult, "Search results for tourism in Zimbabwe") {
			resultObserved = true
		}
	}
	if !resultObserved {
		t.Fatal("missing independent post-navigation observation")
	}
	t.Logf("synthetic search full flow: calls=%d one launch/input/Enter; source checks retained after URL transition", calls)
}

func TestNativeSearchActionWorkflowInstalledModel(t *testing.T) {
	endpoint, model := os.Getenv("OFFGRID_TEST_COMPLETION_URL"), os.Getenv("OFFGRID_TEST_CONTEXT_MODEL")
	if endpoint == "" || model == "" {
		t.Skip("requires explicit installed local model endpoint; synthetic host only")
	}
	parsed, err := url.Parse(endpoint)
	if err != nil || parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" || (parsed.Scheme != "http" && parsed.Scheme != "https") || net.ParseIP(parsed.Hostname()) == nil || !net.ParseIP(parsed.Hostname()).IsLoopback() {
		t.Fatal("requires literal loopback model URL without credentials/query/fragment")
	}
	engine := inference.NewLlamaHTTPEngine(strings.TrimSuffix(endpoint, "/v1/chat/completions"))
	request := func(task *agents.Task, messages []api.ChatMessage, tools []api.Tool) *api.ChatCompletionRequest {
		zero, max := float32(0), 512
		r := &api.ChatCompletionRequest{Model: model, Messages: messages, Tools: tools, ToolChoice: "auto", Temperature: &zero, MaxTokens: &max, StreamOptions: &api.StreamOptions{IncludeUsage: true}}
		configureComputerRequest(r, task)
		return r
	}
	for repetition := 1; repetition <= 3; repetition++ {
		if !t.Run(fmt.Sprintf("repeat-%d", repetition), func(t *testing.T) {
			runSyntheticSearchActionWorkflow(t, func(ctx context.Context, task *agents.Task, messages []api.ChatMessage, tools []api.Tool) (*api.ChatCompletionResponse, error) {
				r := request(task, messages, tools)
				count, err := engine.CountPromptTokens(ctx, r)
				if err != nil || count+512+512 > 8192 {
					return nil, fmt.Errorf("synthetic search prompt exceeds context or cannot be measured: %v", err)
				}
				accumulator := agentStreamAccumulator{calls: make(map[int]*api.ToolCall)}
				if err := engine.ChatCompletionStreamRaw(ctx, r, func(data json.RawMessage) error { _, _, err := accumulator.add(data); return err }); err != nil {
					return nil, err
				}
				result := accumulator.response()
				if result.Usage.PromptTokens <= 0 || result.Usage.PromptTokens+512+512 > 8192 {
					return nil, fmt.Errorf("actual synthetic search prompt usage unavailable or exceeds 8192")
				}
				t.Logf("synthetic search inference: counted_prompt=%d actual_prompt=%d output=%d", count, result.Usage.PromptTokens, result.Usage.CompletionTokens)
				for _, choice := range result.Choices {
					for _, call := range choice.Message.ToolCalls {
						t.Logf("synthetic search proposal: %s %s", call.Function.Name, call.Function.Arguments)
					}
				}
				return result, nil
			}, func(ctx context.Context, task *agents.Task, messages []api.ChatMessage, tools []api.Tool) (int, error) {
				return engine.CountPromptTokens(ctx, request(task, messages, tools))
			}, false)
		}) {
			break
		}
	}
}
