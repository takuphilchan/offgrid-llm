package server

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/takuphilchan/offgrid-llm/internal/agents"
	"github.com/takuphilchan/offgrid-llm/internal/computer"
	"github.com/takuphilchan/offgrid-llm/internal/inference"
	"github.com/takuphilchan/offgrid-llm/pkg/api"
)

func TestDesktopDurableChromeWorkflowKeepsMCPAndNeverAsksForWindow(t *testing.T) {
	for _, repeatLaunch := range []bool{false, true} {
		name := "normal"
		if repeatLaunch {
			name = "duplicate_launch_repaired"
		}
		t.Run(name, func(t *testing.T) { testDesktopDurableChromeWorkflow(t, repeatLaunch, "Google Chrome", false) })
	}
}

func TestDesktopObservationContextContinuesIntoControlWorkflow(t *testing.T) {
	for _, app := range []string{"Microsoft Edge", "LibreOffice Writer", "محرر 旅行"} {
		t.Run(app, func(t *testing.T) { testDesktopDurableChromeWorkflow(t, false, app, true) })
	}
}

func TestDesktopVerifiedApplicationRejectsRelaunchBeforeFurtherControls(t *testing.T) {
	testDesktopDurableChromeWorkflow(t, true, "Registered application", true)
}

func testDesktopDurableChromeWorkflow(t *testing.T, repeatLaunch bool, appName string, controlWorkflow bool) {
	s, baseTools, _, _, calls := compositionFixture(t)
	// Provider metadata is not bounded by a short hand-authored fixture. Model
	// the additional schema bytes of three documentation tools in the 8K report.
	tools := &desktopSchemaPressure{browserRunTools: baseTools}
	sequence := []api.FunctionCall{{Name: "mcp__docs__docs", Arguments: `{}`}, {Name: "computer_applications", Arguments: `{}`}, {Name: "computer_open_application", Arguments: `{"application_id":"chrome"}`}, {Name: "computer_verify_application", Arguments: `{"application_id":"chrome"}`}}
	if repeatLaunch && !controlWorkflow {
		sequence = append(sequence[:3:3], sequence[2], sequence[1], sequence[3])
	}
	if repeatLaunch && controlWorkflow {
		sequence = append(sequence, sequence[2])
	}
	if controlWorkflow {
		sequence = append(sequence,
			api.FunctionCall{Name: "computer_observe", Arguments: `{}`},
			api.FunctionCall{Name: "computer_find_controls", Arguments: `{"observation_id":"before-edit","name":"Fixture document"}`},
			api.FunctionCall{Name: "computer_inspect_control", Arguments: `{"observation_id":"before-edit","element":"document"}`},
			api.FunctionCall{Name: "task_expectations", Arguments: `{"expectations":[{"id":"text","title":"Search field text","kind":"control_text_equals","target_id":"fixture-window","element":"search-field","text":"tourism"}]}`},
			api.FunctionCall{Name: "computer_replace_text", Arguments: `{"observation_id":"before-edit","element":"search-field","text":"tourism"}`},
			api.FunctionCall{Name: "computer_observe", Arguments: `{}`},
			api.FunctionCall{Name: "computer_shortcut", Arguments: `{"observation_id":"after-edit","element":"search-field","shortcut":"enter"}`},
			api.FunctionCall{Name: "computer_observe", Arguments: `{}`},
			api.FunctionCall{Name: "computer_verify", Arguments: `{"observation_id":"after-submit","element":"search-field","text":"tourism"}`},
			api.FunctionCall{Name: "computer_verify_application", Arguments: `{"application_id":"chrome"}`})
	}
	sequence = append([]api.FunctionCall{{Name: "task_expectations", Arguments: `{"expectations":[{"id":"open","title":"Application ready","kind":"application_open","application_id":"chrome"},{"id":"research","title":"Research","kind":"research"}]}`}}, sequence...)
	modelCalls := 0
	r := agents.NewRunner(s.agentManager, tools, func(_ context.Context, task *agents.Task, messages []api.ChatMessage, available []api.Tool) (*api.ChatCompletionResponse, error) {
		names := availableNames(available)
		if !names["mcp__docs__docs"] || !names["computer_open_application"] || names[computerAccessToolName] {
			t.Error("incorrect composition", names)
		}
		if task.Context.EstimatedTokens+task.Config.MaxTokens+512 > task.Context.Window {
			t.Fatal("model received an over-budget prompt")
		}
		last := messages[len(messages)-1]
		if last.Name == "computer_observe" || last.Name == "computer_verify_application" {
			if !strings.Contains(last.StringContent(), `"element_columns"`) || !strings.Contains(last.StringContent(), modelReference(nativeModelReferences(task), "search-field")) || !strings.Contains(last.StringContent(), "omitted_controls") {
				t.Fatal("missing compact current controls")
			}
			canonical := task.Checkpoint.Messages[len(task.Checkpoint.Messages)-1].StringContent()
			if strings.Contains(canonical, "element_columns") || !strings.Contains(canonical, `"writable":true`) {
				t.Fatal("canonical observation was rewritten")
			}
		}
		m := api.ChatMessage{Role: "assistant", Content: appName + " independently observed; fixture control state checked, not proof of public search."}
		reason := "stop"
		if modelCalls < len(sequence) {
			m.Content = ""
			m.ToolCalls = []api.ToolCall{{Type: "function", Function: sequence[modelCalls]}}
			m.ToolCalls[0].Function.Arguments = translateNativeReferences(m.ToolCalls[0].Function.Name, m.ToolCalls[0].Function.Arguments, nativeModelReferences(task))
			reason = "tool_calls"
		}
		modelCalls++
		return &api.ChatCompletionResponse{Choices: []api.ChatCompletionChoice{{Message: m, FinishReason: reason}}}, nil
	})
	countURL := os.Getenv("OFFGRID_TEST_TOKEN_COUNT_URL")
	if countURL == "" {
		// Deterministic tokenizer double. The opt-in check below uses the installed
		// runtime's real tokenizer, without running any of the proposed host tools.
		endpoint := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			if req.URL.Path != "/v1/chat/completions/input_tokens" {
				t.Error("unexpected generation")
			}
			w.Write([]byte(`{"input_tokens":5200}`))
		}))
		t.Cleanup(endpoint.Close)
		countURL = endpoint.URL
	}
	counter := inference.NewLlamaHTTPEngine(countURL)
	r.CountContext = func(ctx context.Context, task *agents.Task, messages []api.ChatMessage, tools []api.Tool) (int, error) {
		max := task.Config.MaxTokens
		req := &api.ChatCompletionRequest{Messages: messages, Tools: tools, ToolChoice: "auto", MaxTokens: &max}
		configureComputerRequest(req, task)
		return counter.CountPromptTokens(ctx, req)
	}
	defer r.Shutdown(context.Background())
	cfg := agents.DefaultAgentConfig()
	cfg.TaskFirst = true
	cfg.NativeWorkflowVersion = agents.WorkflowEvidenceVersion
	cfg.ContextWindow = 8192
	cfg.MaxIterations = 20
	cfg.RequestedComputerScope = "desktop"
	cfg.RequestedComputerApprovalMode = "full_task"
	task, err := r.Create("Read documentation, open "+appName+" and enter tourism in its search field", "model", "alice", cfg)
	if err != nil {
		t.Fatal(err)
	}
	if modelCalls != 0 || task.Status != agents.TaskInput {
		t.Fatal("model ran before consent")
	}
	code, _ := s.browserHub.PairTaskCode("alice", task.ID, task.PendingInput.ID, "workspace", computer.ApprovalFullTask)
	desktop := computer.TargetIdentity{ID: "desktop", OSSession: "os", ProcessGeneration: "desktop", Surface: "desktop", Driver: "windows-uia"}
	host, token, err := s.browserHub.PairDesktop(code, desktop, computer.ApprovalFullTask)
	if err != nil {
		t.Fatal(err)
	}
	if err = s.browserHub.ActivateDesktop(token); err != nil {
		t.Fatal(err)
	}
	if err = s.browserHub.ReserveTask("alice", host.ID, task.ID); err != nil {
		t.Fatal(err)
	}
	cfg.ComputerSession = host.ID
	cfg.ComputerDriver = "windows-uia"
	cfg.ComputerScope = "desktop"
	cfg.ComputerApprovalMode = "full_task"
	configureDesktopTask(&cfg)
	task, err = r.ResolveComputerInput(task.ID, "alice", task.PendingInput.ID, cfg)
	if err != nil {
		t.Fatal(err)
	}
	task, err = r.Continue(context.Background(), task.ID, "alice", "start", "", false)
	if err != nil || task.Status != agents.TaskWaiting || task.PendingApproval.Tool != "mcp__docs__docs" || calls.Load() != 0 {
		t.Fatalf("independent MCP approval lost: %+v %v", task, err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	done := make(chan error, 1)
	go func() {
		launches := 0
		target := desktop
		target.ID, target.ProcessGeneration, target.Surface = "fixture-window", "fixture-process", "fixture-surface"
		controlText, observation := "", ""
		observations, submitted := 0, false
		for n := 0; n < 30; {
			select {
			case <-ctx.Done():
				done <- ctx.Err()
				return
			case <-time.After(time.Millisecond):
			}
			a, err := s.browserHub.Poll(token)
			if err != nil {
				done <- err
				return
			}
			if a == nil {
				continue
			}
			result := `{}`
			switch a.Kind {
			case "computer_prepare":
				result = `{"prepared":true,"approval_class":"reversible"}`
			case "computer_applications":
				encoded, _ := json.Marshal(map[string]any{"applications": []map[string]string{{"id": "chrome", "name": appName}}})
				result = string(encoded)
			case "computer_open_application":
				launches++
				if launches > 1 {
					done <- context.Canceled
					return
				}
				result = `{"actions":[{"outcome":"dispatched"}]}`
			case "computer_verify_application":
				view := desktopContextFixture(target, appName)
				view.Elements[7].Text = &controlText
				b, _ := json.Marshal(computer.ApplicationEvidence{Verified: true, Check: "application_open", Application: "chrome", Target: &target, View: &view})
				if len(b) < 8000 {
					t.Errorf("observation fixture too small: %d bytes", len(b))
				}
				result = string(b)
			case "computer_observe":
				observations++
				if observations > 3 {
					done <- fmt.Errorf("unexpected repeated observation")
					return
				}
				observation = []string{"before-edit", "after-edit", "after-submit"}[observations-1]
				view := desktopContextFixture(target, appName)
				view.Observation.ID = observation
				view.Elements[7].Text = &controlText
				b, _ := json.Marshal(view)
				result = string(b)
			case "computer_replace_text":
				controlText = "tourism"
				result = `{"actions":[{"outcome":"dispatched","uncertain":false}]}`
			case "computer_shortcut":
				if submitted {
					done <- fmt.Errorf("duplicate submission")
					return
				}
				submitted = true
				result = `{"actions":[{"outcome":"dispatched","uncertain":false}]}`
			case "computer_verify":
				b, _ := json.Marshal(map[string]any{"verified": submitted && controlText == "tourism", "check": "control_text_equals", "observation_id": observation, "element": "search-field", "text": controlText, "target": target})
				result = string(b)
			default:
				done <- context.Canceled
				return
			}
			if err = s.browserHub.Reply(token, computer.BrowserReply{ID: a.ID, Result: result}); err != nil {
				done <- err
				return
			}
			n++
			if a.Kind == "computer_verify_application" && (!controlWorkflow || submitted) {
				done <- nil
				return
			}
		}
		done <- nil
	}()
	task, err = r.Continue(ctx, task.ID, "alice", "approve", task.PendingApproval.ID, false)
	if err != nil || task.Status != agents.TaskCompleted {
		t.Fatalf("continuation status=%s error=%s context=%+v calls=%d: %v", task.Status, task.Error, task.Context, modelCalls, err)
	}
	if e := <-done; e != nil {
		t.Fatal(e)
	}
	t.Logf("allocated=%d prompt budget=%d compactions=%d calls=%d", task.Context.Window, task.Context.EstimatedTokens, task.Context.Compactions, modelCalls)
	if task.Workflow == nil || task.Workflow.State != "partial" {
		t.Fatalf("expected narrow partial proof: %+v", task.Workflow)
	}
	if err != nil || task.Status != agents.TaskCompleted || calls.Load() != 1 || modelCalls != len(sequence)+1 || task.Checkpoint.ProposalUnresolved {
		t.Fatalf("desktop workflow failed: %+v %v", task, err)
	}
	if repeatLaunch {
		rejected := 0
		code := "computer_application_verification_required"
		if controlWorkflow {
			code = "computer_application_already_ready"
		}
		for _, step := range task.Steps {
			if step.Type == "rejected" && strings.Contains(step.ToolResult, code) {
				rejected++
			}
		}
		if rejected != 1 {
			t.Fatal("duplicate launch did not enter pre-dispatch repair")
		}
	}
}

// Synthetic data only. The reported failure had 46 ordinary controls, not the
// single-element fixture that previously passed context-continuation checks.
func desktopContextFixture(target computer.TargetIdentity, name string) computer.NativeView {
	view := computer.NativeView{Observation: computer.ControlObservation{ID: "view", Target: target, Sequence: 2, CapturedAt: time.Unix(1700000000, 0).UTC(), StateDigest: strings.Repeat("a", 64)}}
	for i := 0; i < 46; i++ {
		view.Elements = append(view.Elements, computer.NativeElement{ID: fmt.Sprintf("%032x", i+1), Name: fmt.Sprintf("Fixture navigation control number %02d", i), Role: "button", Invokable: true})
	}
	view.Elements[0].Name, view.Elements[0].Role = name, "region"
	view.Elements[0].Invokable = false
	empty, document := "", strings.Repeat("Synthetic page text. ", 20)
	view.Elements[7] = computer.NativeElement{ID: "search-field", Identity: "fixture-os-control", Name: "Address and search bar", Role: "edit", Writable: true, Text: &empty}
	view.Elements[40] = computer.NativeElement{ID: "document", Name: "Fixture document", Role: "document", Text: &document}
	for _, i := range []int{1, 2} {
		url := "https://example.test/redirect?tracking="
		for j := 0; j < 31; j++ {
			url += fmt.Sprintf("%x", sha256.Sum256([]byte(fmt.Sprintf("url:%d:%d", i, j))))
		}
		view.Elements[i].Text = &url
	}
	return view
}

type desktopSchemaPressure struct{ *browserRunTools }

func (b *desktopSchemaPressure) ToolsForTaskContext(ctx context.Context, task *agents.Task) []api.Tool {
	tools := b.browserRunTools.ToolsForTaskContext(ctx, task)
	for i := range tools {
		if tools[i].Function.Name == "mcp__docs__docs" {
			tools[i].Function.Description += strings.Repeat(" Reference documentation. ", 140)
		}
	}
	return tools
}

func TestDesktopSubmitIsHeldAndRejectsV2Attachment(t *testing.T) {
	s := taskFirstServer(t)
	w := httptest.NewRecorder()
	body := `{"prompt":"open chrome","model":"model","request_id":"desktop-request","computer_scope":"desktop","computer_approval_mode":"full_task"}`
	s.handleJobSubmit(w, httptest.NewRequest("POST", "/api/v2/jobs", strings.NewReader(body)))
	if w.Code != 202 {
		t.Fatal(w.Code, w.Body.String())
	}
	var response struct {
		ID string `json:"run_id"`
	}
	json.Unmarshal(w.Body.Bytes(), &response)
	task, _ := s.agentManager.GetTask(response.ID)
	if task.Status != agents.TaskInput || task.Checkpoint.Iteration != 0 || task.PendingInput.Mode != "desktop" {
		t.Fatalf("inference began before access: %+v", task)
	}
	code, _ := s.browserHub.PairCode(task.Actor)
	target := computer.TargetIdentity{ID: "window", OSSession: "os", ProcessGeneration: "process", Surface: "surface", Driver: "windows-uia"}
	host, _, err := s.browserHub.PairNative(code, "Chrome", target, 2, computer.ApprovalFullTask)
	if err != nil {
		t.Fatal(err)
	}
	w = httptest.NewRecorder()
	s.handleJobInput(w, httptest.NewRequest("POST", "/api/v2/jobs/"+task.ID+"/input", strings.NewReader(`{"input_id":"`+task.PendingInput.ID+`","computer_session":"`+host.ID+`"}`)), task.ID)
	if w.Code != 409 || !strings.Contains(w.Body.String(), "scope") {
		t.Fatal(w.Code, w.Body.String())
	}
}

func TestDesktopOpenChromeNeedsCatalogIDAndRealReadinessNotURL(t *testing.T) {
	if err := validateNativeArguments("computer_open_application", json.RawMessage(`{"application_id":"host-chrome"}`)); err != nil {
		t.Fatal(err)
	}
	for _, args := range []string{`{"url":"https://example.com"}`, `{"application_id":"C:\\chrome.exe"}`, `{"application_id":"host-chrome","command":"cmd"}`} {
		if validateNativeArguments("computer_open_application", json.RawMessage(args)) == nil {
			t.Fatal("accepted model path/command", args)
		}
	}
	var cfg agents.AgentConfig
	configureDesktopTask(&cfg)
	if !strings.Contains(cfg.SystemPrompt, "A website URL is not needed") {
		t.Fatal("missing launch route")
	}
	target := computer.TargetIdentity{ID: "window", OSSession: "os", ProcessGeneration: "process", Surface: "surface", Driver: "windows-uia"}
	evidence := computer.ApplicationEvidence{Verified: true, Check: "application_open", Application: "host-chrome", Target: &target, View: &computer.NativeView{Observation: computer.ControlObservation{ID: "view", Target: target, CapturedAt: time.Now()}}}
	encoded, _ := json.Marshal(evidence)
	task := &agents.Task{Config: agents.AgentConfig{ComputerDriver: "windows-uia", ComputerScope: "desktop"}, Steps: []agents.Step{{Type: "action", ToolName: "computer_open_application", ToolArgs: `{"application_id":"host-chrome"}`}, {Type: "action", ToolName: "computer_verify_application", ToolArgs: `{"application_id":"host-chrome"}`, ToolResult: string(encoded)}}}
	tools := &browserRunTools{}
	if err := tools.ValidateCompletion(task); err != nil {
		t.Fatal(err)
	}
	task.Steps = append(task.Steps, agents.Step{Type: "action", ToolName: "mcp_docs", ToolResult: "source"})
	if err := tools.ValidateCompletion(task); err != nil {
		t.Fatal("MCP erased readiness", err)
	}
	task.Steps[0].ToolName = "computer_replace_text"
	if tools.ValidateCompletion(task) == nil {
		t.Fatal("launch check verified document edit")
	}
	task.Steps = []agents.Step{{Type: "action", ToolName: "mcp_docs"}}
	if err := tools.ValidateCompletion(task); err != nil {
		t.Fatal("desktop grant forced unrelated host work", err)
	}
}
