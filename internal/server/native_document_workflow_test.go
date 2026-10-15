package server

// Full durable runner -> service authorization -> authenticated hub -> owned
// executor/supervisor/journal -> independent read-back. The provider below is
// synthetic and has NO OS input or filesystem capability. Native qualification
// is a separate opt-in suite; these tests must be safe on a personal desktop.
import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync/atomic"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/takuphilchan/offgrid-llm/internal/agents"
	"github.com/takuphilchan/offgrid-llm/internal/capabilities"
	"github.com/takuphilchan/offgrid-llm/internal/computer"
	"github.com/takuphilchan/offgrid-llm/pkg/api"
)

// Keep a realistic composed catalog for token-budget testing, but an installed
// model must never execute builtin filesystem/network/shell tools in this test.
type documentFixtureRegistry struct{ agents.RunTools }

func (r documentFixtureRegistry) GetTools() []api.Tool {
	var tools []api.Tool
	for _, tool := range r.RunTools.GetTools() {
		if tool.Function.Name == "mcp__docs__docs" {
			tools = append(tools, tool)
		}
	}
	return tools
}

func (r documentFixtureRegistry) Authorize(ctx context.Context, name string, args json.RawMessage, e agents.ToolExecution) error {
	if name != "mcp__docs__docs" {
		return capabilities.ErrDenied
	}
	return r.RunTools.Authorize(ctx, name, args, e)
}
func (r documentFixtureRegistry) ExecuteWithPolicy(ctx context.Context, name string, args json.RawMessage, e agents.ToolExecution) (string, error) {
	if name != "mcp__docs__docs" {
		return "", capabilities.ErrDenied
	}
	return r.RunTools.ExecuteWithPolicy(ctx, name, args, e)
}

type workflowDocumentBackend struct {
	target   computer.TargetIdentity
	text     string
	created  bool
	sequence uint64
	inputs   atomic.Int32
	creates  atomic.Int32
	fault    string
}

func (f *workflowDocumentBackend) ReadDocument(ctx context.Context, binding computer.ControlBinding) (computer.DocumentInventory, computer.DocumentSnapshot, error) {
	if ctx.Err() != nil {
		return computer.DocumentInventory{}, computer.DocumentSnapshot{}, ctx.Err()
	}
	if binding.Target != f.target {
		return computer.DocumentInventory{}, computer.DocumentSnapshot{}, computer.ErrControlScope
	}
	f.sequence++
	doc, gen, container, revision, text := "old", "old-generation", "old-tab", "old-revision", "Untouched — café 日本語"
	if f.created {
		doc, gen, container, revision, text = "new", "new-generation", "new-tab", "content-"+computer.DocumentTextDigest(f.text), f.text
	}
	b := computer.DocumentBinding{Target: f.target, Document: doc, Generation: gen, Container: container, Editor: "reused-editor", Observation: fmt.Sprintf("observation-%d", f.sequence), Revision: revision}
	inv := computer.DocumentInventory{Version: 1, Complete: true, Active: &b, Observation: computer.ControlObservation{ID: b.Observation, Target: f.target, Sequence: f.sequence, CapturedAt: time.Now().UTC(), StateDigest: computer.DocumentTextDigest(text)}, Documents: []computer.NativeDocument{{ID: "old", Generation: "old-generation", Container: "old-tab", Selected: !f.created, Capabilities: computer.DocumentCapabilities{Read: true}}}, Commands: []computer.DocumentCreationCommand{{ID: "new-command", Element: "new-button", Kind: "new_document", Profile: "owned-fixture", Revision: "v1"}}}
	if f.created {
		// Advertise only operations implemented by this owned provider. Selection
		// and replacement have separate executor fixtures; returning unsupported
		// capabilities here falsely tests a host that promises actions it rejects.
		inv.Documents = append(inv.Documents, computer.NativeDocument{ID: doc, Generation: gen, Container: container, Selected: true, Capabilities: computer.DocumentCapabilities{Read: true, InsertText: true}})
	}
	caret := utf8.RuneCountInString(text)
	snap := computer.DocumentSnapshot{ID: fmt.Sprintf("snapshot-%d", f.sequence), Binding: b, Text: text, Complete: true, Composition: "stable", Selection: &computer.DocumentTextRange{Start: caret, End: caret}}
	return inv, snap, nil
}

func (f *workflowDocumentBackend) ApplyDocumentLifecycle(_ context.Context, _ computer.ControlBinding, _ computer.DocumentInventory, op computer.DocumentOperation) error {
	if op.Kind != "create_document" || op.Command != "new-command" || f.created {
		return computer.ErrInvalidControl
	}
	f.creates.Add(1)
	f.created = true
	f.text = ""
	return nil
}

func (f *workflowDocumentBackend) ApplyDocumentText(_ context.Context, _ computer.ControlBinding, plan computer.PreparedDocumentText) error {
	if !f.created || plan.Preimage().Text != f.text {
		return computer.ErrDocumentChanged
	}
	f.inputs.Add(1)
	f.text = plan.ExpectedText()
	if f.fault == "partial-input" {
		f.text = "partial"
		return computer.ErrUncertain
	}
	return nil
}

// Mirrors the owned companion's existing transport but uses no native app.
type workflowDocumentHost struct {
	backend    *workflowDocumentBackend
	executor   *computer.NativeDocumentExecutor
	supervisor *computer.NativeSupervisor
	binding    computer.ControlBinding
	inventory  computer.DocumentInventory
	snapshot   computer.DocumentSnapshot
	prepared   computer.PreparedControlAction
	desktop    bool
	launches   atomic.Int32
	appName    string
}

// A document fixture must describe the same editor as its document provider.
// Reusing a browser pressure fixture (URLs/navigation) made model qualification
// test a different application than the executor actually implemented.
func (h *workflowDocumentHost) observe() computer.NativeView {
	h.backend.sequence++
	text := "Untouched — café 日本語"
	if h.backend.created {
		text = h.backend.text
	}
	view := computer.NativeView{
		Observation: computer.ControlObservation{ID: fmt.Sprintf("view-%d", h.backend.sequence), Target: h.binding.Target, Sequence: h.backend.sequence, CapturedAt: time.Now().UTC(), StateDigest: computer.DocumentTextDigest(text)},
		Elements: []computer.NativeElement{
			{ID: "editor-region", Name: "Notepad", Role: "region"},
			{ID: "old-tab", Name: "Existing unsaved document", Role: "tab", Invokable: true},
			{ID: "new-button", Name: "New document", Role: "button", Invokable: true},
			{ID: "reused-editor", Name: "Document editor", Role: "document", Text: &text, Writable: h.backend.created},
		},
	}
	if h.appName != "" {
		view.Elements = []computer.NativeElement{{ID: "application-region", Name: h.appName, Role: "region"}}
	}
	return view
}

func (h *workflowDocumentHost) execute(ctx context.Context, a *computer.BrowserAction) (string, error) {
	switch a.Kind {
	case "computer_applications":
		apps := []computer.NativeApplication{}
		for i := 0; i < 96; i++ {
			name, id := fmt.Sprintf("Installed fixture app %02d", i), fmt.Sprintf("fixture-app-%02d", i)
			if i == 37 {
				name, id = "Notepad", "fixture-editor"
			}
			if h.appName != "" && i == 37 {
				name, id = "Microsoft Access", "fixture-access"
			}
			if h.appName != "" && i == 61 {
				name, id = h.appName, "fixture-editor"
			}
			apps = append(apps, computer.NativeApplication{ID: id, Name: name})
		}
		return documentJSON(map[string]any{"applications": apps}), nil
	case "computer_open_application":
		var selected struct {
			ID string `json:"application_id"`
		}
		if json.Unmarshal(a.Arguments, &selected) != nil || selected.ID != "fixture-editor" {
			return "", computer.ErrControlScope
		}
		if h.launches.Add(1) != 1 {
			return "", computer.ErrUncertain
		}
		return `{"actions":[{"action_id":"fixture-launch","outcome":"dispatched","uncertain":false}]}`, nil
	case "computer_verify_application":
		if h.launches.Load() != 1 {
			return "", computer.ErrApplicationNotReady
		}
		view := h.observe()
		return documentJSON(computer.ApplicationEvidence{Verified: true, Check: "application_open", Application: "fixture-editor", Target: &h.binding.Target, View: &view}), nil
	case "computer_observe":
		if h.desktop && h.launches.Load() != 1 {
			return "", computer.ErrTargetNotSelected
		}
		return documentJSON(h.observe()), nil
	case "computer_documents":
		if h.desktop && h.launches.Load() != 1 {
			return "", computer.ErrTargetNotSelected
		}
		var err error
		h.inventory, h.snapshot, err = h.executor.Read(ctx, h.binding)
		return documentJSON(h.inventory), err
	case "computer_read_text":
		var args struct {
			Observation string                        `json:"observation_id"`
			Document    string                        `json:"document"`
			Page        *computer.DocumentPageRequest `json:"page,omitempty"`
		}
		if json.Unmarshal(a.Arguments, &args) != nil || h.inventory.Active == nil || args.Observation != h.inventory.Observation.ID || args.Document != h.inventory.Active.Document {
			return "", computer.ErrDocumentChanged
		}
		if args.Page != nil && (args.Page.Validate() != nil || args.Page.Snapshot != h.snapshot.ID || args.Page.Revision != h.snapshot.Binding.Revision) {
			return "", computer.ErrStaleObservation
		}
		current, snapshot, err := h.executor.ReadVerification(ctx, h.binding)
		if err != nil {
			return "", err
		}
		if current.Active == nil || computer.ValidateDocumentPrecondition(h.snapshot, snapshot) != nil {
			return "", computer.ErrDocumentChanged
		}
		snapshot.Binding.Observation = h.inventory.Observation.ID
		start := 0
		if args.Page == nil {
			h.snapshot = snapshot
		} else {
			start = args.Page.Start
		}
		read, err := computer.NewDocumentReadback(h.snapshot)
		if err != nil {
			return "", err
		}
		pager, err := computer.NewDocumentPager(h.snapshot)
		if err != nil {
			return "", err
		}
		page, err := pager.Page(h.snapshot.ID, h.snapshot.Binding.Revision, start, snapshot)
		read.Page, read.Selection = &page, h.snapshot.Selection
		return documentJSON(read), err
	case "computer_prepare":
		var args struct {
			Tool      string          `json:"tool"`
			Arguments json.RawMessage `json:"arguments"`
		}
		if computer.DecodeControlMessage(a.Arguments, &args) != nil {
			return "", computer.ErrInvalidControl
		}
		if args.Tool == "computer_open_application" && h.desktop {
			return `{"prepared":true,"approval_class":"reversible"}`, nil
		}
		edit, ok := decodeDocumentEditArguments(args.Tool, args.Arguments)
		if !ok {
			return "", computer.ErrInvalidControl
		}
		op := computer.DocumentOperation{Kind: documentEditModeForTool(args.Tool), Document: edit.Document, Snapshot: edit.Snapshot, Text: edit.Text}
		if args.Tool == "computer_create_document" {
			op = computer.DocumentOperation{Kind: "create_document", Command: edit.Command}
		}
		var err error
		h.prepared, err = h.executor.PrepareCurrent(ctx, h.binding, op, h.inventory, h.snapshot)
		if err != nil {
			return "", err
		}
		return `{"prepared":true,"approval_class":"reversible"}`, nil
	case "computer_create_document", "computer_insert_text":
		if h.prepared.ID == "" {
			return "", computer.ErrControlApproval
		}
		step := computer.BoundedStep{ID: "step-" + h.prepared.ID, Binding: h.binding, Actions: []computer.PreparedControlAction{h.prepared}}
		grant, err := h.supervisor.Approve(step, func(computer.BoundedStep) bool { return true })
		if err != nil {
			return "", err
		}
		results, err := h.supervisor.Execute(step, grant.ID)
		h.prepared = computer.PreparedControlAction{}
		if err != nil {
			return "", err
		}
		if len(results) != 1 || results[0].Document == nil {
			return "", computer.ErrUncertain
		}
		if a.Kind == "computer_insert_text" && h.backend.fault == "lost-ack" {
			return "", computer.ErrUncertain
		}
		return documentJSON(results[0].Document), nil
	}
	return "", computer.ErrInvalidControl
}

func TestDocumentWorkflowRunnerHostCreateEditReadback(t *testing.T) {
	for _, fault := range []string{"none", "partial-input", "lost-ack", "lost-service-ack"} {
		t.Run(fault, func(t *testing.T) { runDocumentWorkflowFixture(t, fault, nil) })
	}
}

func TestDocumentWorkflowDesktopLaunchCreateEditReadback(t *testing.T) {
	runDocumentWorkflowFixtureFrom(t, "none", nil, true)
}

// A callback may use an explicitly configured installed local model. Its only
// executable computer tools still target the synthetic in-memory provider.
func runDocumentWorkflowFixture(t *testing.T, fault string, modelCall agents.RunCaller) {
	runDocumentWorkflowFixtureFrom(t, fault, modelCall, false)
}

func runDocumentWorkflowFixtureFrom(t *testing.T, fault string, modelCall agents.RunCaller, desktop bool, counters ...func(context.Context, *agents.Task, []api.ChatMessage, []api.Tool) (int, error)) {
	runConversationQualificationFixture(t, fault, modelCall, desktop, "document", counters...)
}

func runConversationQualificationFixture(t *testing.T, fault string, modelCall agents.RunCaller, desktop bool, scenario string, counters ...func(context.Context, *agents.Task, []api.ChatMessage, []api.Tool) (int, error)) {
	t.Helper()
	appOnly := scenario == "obs" || scenario == "settings" || scenario == "mixed" || scenario == "mcp-native"
	features := computer.NativeFeatures{Documents: 1}
	if appOnly {
		features.Documents = 0
	}
	measurements := &computer.NativeMeasurements{}
	totalStarted := time.Now()
	calls := 0
	measuredTokens, tokensKnown := 0, modelCall != nil
	defer func() {
		var tokens *int
		if tokensKnown {
			tokens = &measuredTokens
		}
		measurements.RecordModel(calls, tokens)
		measurements.Add("total", time.Since(totalStarted))
		t.Log("OFFGRID_NATIVE_METRICS " + documentJSON(measurements.Report()))
	}()
	s, adapter, _, _, mcpCalls := compositionFixture(t)
	s.browserHub.RecordReply = s.recordComputerReply
	adapter.RunTools = documentFixtureRegistry{adapter.RunTools}
	code, _ := s.browserHub.PairCode("alice")
	target := computer.TargetIdentity{ID: "fixture-window", OSSession: "fixture-os", ProcessGeneration: "fixture-process", Surface: "fixture-surface", Driver: "windows-uia"}
	session, token, err := s.browserHub.PairNativeFeatures(code, "Owned document fixture", target, 2, computer.ApprovalFullTask, features)
	if desktop {
		s.browserHub.StopSession("alice", session.ID)
	}
	if err != nil {
		t.Fatal(err)
	}
	cfg := compositionConfig(target.Driver, true)
	cfg.ComputerSession, cfg.NativeWorkflowVersion = session.ID, 2
	if desktop {
		cfg.ComputerSession = ""
		cfg.RequestedComputerScope, cfg.RequestedComputerApprovalMode = "desktop", "full_task"
	}
	cfg.ContextWindow, cfg.MaxIterations = 8192, 24
	if modelCall != nil {
		cfg.MaxTokens = 512
	} // match the actual installed-model request/reserve
	configureDesktopTask(&cfg)
	if scenario == "greeting" {
		cfg.ComputerSession, cfg.RequestedComputerScope, cfg.RequestedComputerApprovalMode = "", "", ""
	}
	const input = "OffGrid test — café 日本語 12345"
	runner := agents.NewRunner(s.agentManager, adapter, func(ctx context.Context, task *agents.Task, messages []api.ChatMessage, tools []api.Tool) (*api.ChatCompletionResponse, error) {
		calls++
		if modelCall != nil {
			started := time.Now()
			result, err := modelCall(ctx, task, messages, tools)
			measurements.Add("model", time.Since(started))
			if result != nil && result.Usage.PromptTokens > 0 {
				measuredTokens += result.Usage.PromptTokens
			} else {
				tokensKnown = false
			}
			return result, err
		}
		name, args := "", "{}"
		state, _ := documentState(task)
		checks := adapter.AssessWorkflow(task)
		baseChecks := 0
		if desktop {
			baseChecks = 1
		}
		verified := len(checks) == 2+baseChecks
		for _, check := range checks {
			verified = verified && check.State == "verified"
		}
		if verified {
			return &api.ChatCompletionResponse{Choices: []api.ChatCompletionChoice{{FinishReason: "stop", Message: api.ChatMessage{Content: "Created a new unsaved document, entered and independently read back the requested text."}}}}, nil
		}
		catalog, _, selected := desktopPlanningState(task)
		_, lookupErr := resolvedApplicationLookup(task)
		if desktop && !catalog {
			name = "computer_applications"
		} else if desktop && lookupErr != nil {
			name, args = applicationSearchTool, latestApplicationQuery(task, "Notepad")
		} else if desktop && len(checks) == 0 {
			name, args = "task_expectations", `{"expectations":[{"id":"open","title":"Notepad ready","kind":"application_open","application_id":"fixture-editor"}]}`
		} else if desktop && len(agents.PendingApplicationLaunches(task)) > 0 {
			name, args = "computer_verify_application", `{"application_id":"fixture-editor"}`
		} else if desktop && !selected {
			name, args = "computer_open_application", `{"application_id":"fixture-editor"}`
		} else if state.step == 0 {
			name = "computer_documents"
		} else if state.inventory.Active.Document != "new" {
			if len(checks) == baseChecks {
				name = "task_expectations"
				args = documentJSON(map[string]any{"expectations": []agents.OutcomeExpectation{{ID: "blank", Title: "New blank document", Kind: "document_blank", TargetID: target.ID, Command: "new-command"}}})
			} else {
				name = "computer_create_document"
				args = documentJSON(documentEditArguments{Observation: state.inventory.Observation.ID, Command: "new-command"})
			}
		} else if state.readStep == 0 {
			name = "computer_read_text"
			args = documentJSON(map[string]string{"observation_id": state.inventory.Observation.ID, "document": state.inventory.Active.Document})
		} else if len(checks) == 1+baseChecks {
			name = "task_expectations"
			text := input
			args = documentJSON(map[string]any{"expectations": []agents.OutcomeExpectation{{ID: "edit", Title: "Exact text", Kind: "document_edit_matches", TargetID: target.ID, Document: state.read.Binding.Document, Snapshot: state.read.Snapshot, EditMode: "insert_text", Text: &text}}})
		} else {
			name = "computer_insert_text"
			text := input
			args = documentJSON(documentEditArguments{Observation: state.read.Binding.Observation, Document: state.read.Binding.Document, Snapshot: state.read.Snapshot, Text: &text})
		}
		if !availableNames(tools)[name] {
			t.Fatalf("stage omitted needed tool %s", name)
		}
		return &api.ChatCompletionResponse{Choices: []api.ChatCompletionChoice{{FinishReason: "tool_calls", Message: api.ChatMessage{ToolCalls: []api.ToolCall{{Function: api.FunctionCall{Name: name, Arguments: args}}}}}}}, nil
	})
	if len(counters) > 0 {
		runner.CountContext = counters[0]
	} else if desktop {
		// Unit tokenizer double, not hardware/model evidence. The installed-model
		// test supplies the real runtime counter for this same complete workflow.
		runner.CountContext = func(context.Context, *agents.Task, []api.ChatMessage, []api.Tool) (int, error) { return 5200, nil }
	}
	defer runner.Shutdown(context.Background())
	prompt := "In the already selected editor create a new blank document, insert exactly: " + input + ". Read it back and confirm. Do not save or change the existing document."
	if desktop {
		prompt = "Open Notepad. In a new blank document, insert exactly: " + input + ". Read it back and confirm. Do not save or change the existing document."
	}
	appName := ""
	switch scenario {
	case "greeting":
		prompt = "Hello!"
	case "obs":
		appName = "OBS Studio"
		prompt = "Open OBS Studio. Confirm it is ready, then finish."
	case "settings":
		appName = "Settings"
		prompt = "Open Settings. Confirm the Settings application is ready, then finish. Do not open Access or change any settings."
	case "mixed":
		appName = "OBS Studio"
		prompt = "Briefly explain what OBS Studio is, and open it. Confirm it is ready. Do not start recording or change any settings."
	case "mcp-native":
		appName = "OBS Studio"
		prompt = "Read the connected synthetic documentation using its docs tool, summarize the notes, and open OBS Studio. Confirm it is ready."
	}
	task, err := runner.Create(prompt, "fixture-model", "alice", cfg)
	if err != nil {
		t.Fatal(err)
	}
	if desktop && scenario != "greeting" {
		if task.PendingInput == nil {
			t.Fatal("desktop fixture bypassed consent")
		}
		code, _ = s.browserHub.PairTaskCode("alice", task.ID, task.PendingInput.ID, "fixture-workspace", computer.ApprovalFullTask)
		root := computer.TargetIdentity{ID: "fixture-desktop", OSSession: target.OSSession, ProcessGeneration: "fixture-desktop", Surface: "desktop", Driver: target.Driver}
		session, token, err = s.browserHub.PairDesktopFeatures(code, root, computer.ApprovalFullTask, features)
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
	}
	backend := &workflowDocumentBackend{target: target, fault: fault}
	binding := computer.ControlBinding{Actor: "alice", Task: task.ID, Companion: session.ID, Session: session.ID, Target: target}
	executor := computer.NewNativeDocumentExecutor(backend)
	executor.SetMeasurements(measurements)
	journal, err := computer.OpenNativeJournal(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer journal.Close()
	now := time.Now()
	supervisor := computer.NewNativeSupervisor(journal, computer.LocalConsent{ID: "fixture-consent", Binding: binding, ApprovalMode: computer.ApprovalFullTask, IssuedAt: now, ExpiresAt: now.Add(35 * time.Minute), Active: true}, executor.Dispatch)
	defer supervisor.Stop()
	host := &workflowDocumentHost{backend: backend, executor: executor, supervisor: supervisor, binding: binding, desktop: desktop, appName: appName}
	timeout := 30 * time.Second
	if modelCall != nil {
		// Isolated CPU qualification may be slow; this is not a latency gate or a
		// production timeout. The real input scopes/action limits stay unchanged.
		timeout = 30 * time.Minute
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	if fault == "lost-service-ack" {
		s.browserHub.RecordReply = func(actor, session, run string, action computer.BrowserAction, reply computer.BrowserReply) error {
			if err := s.recordComputerReply(actor, session, run, action, reply); err != nil {
				return err
			}
			if action.Kind == "computer_insert_text" {
				cancel()
				return computer.ErrUncertain
			}
			return nil
		}
	}
	done := make(chan struct{})
	go func() {
		defer close(done)
		for ctx.Err() == nil {
			if action, e := s.browserHub.Poll(token); e == nil && action != nil {
				started := time.Now()
				result, e := host.execute(ctx, action)
				measurements.Add("tool", time.Since(started))
				reply := computer.BrowserReply{ID: action.ID, Result: result}
				if e != nil {
					reply.Error = e.Error()
				}
				_ = s.browserHub.Reply(token, reply)
			}
			select {
			case <-ctx.Done():
				return
			case <-time.After(time.Millisecond):
			}
		}
	}()
	got, err := runner.Continue(ctx, task.ID, "alice", "start", "", false)
	for err == nil && got.Status == agents.TaskWaiting && scenario == "mcp-native" && got.PendingApproval != nil && got.PendingApproval.Tool == "mcp__docs__docs" {
		got, err = runner.Continue(ctx, task.ID, "alice", "approve", got.PendingApproval.ID, false)
	}
	cancel()
	<-done
	if err != nil && fault == "none" {
		t.Fatal(err)
	}
	if err != nil {
		got, _ = s.agentManager.GetTask(task.ID)
	}
	if scenario == "greeting" || appOnly {
		if got.Status != agents.TaskCompleted || got.Result == "" || backend.creates.Load() != 0 || backend.inputs.Load() != 0 {
			t.Fatalf("conversation/application failed: status=%s error=%s", got.Status, got.Error)
		}
		if scenario == "greeting" {
			if host.launches.Load() != 0 || mcpCalls.Load() != 0 || got.PendingInput != nil || got.Workflow != nil && len(got.Workflow.Checks) != 0 {
				t.Fatal("greeting performed actions or requested checks/access")
			}
		} else if host.launches.Load() != 1 || got.Workflow == nil || got.Workflow.State != "checks_passed" {
			t.Fatal("application not independently verified exactly once")
		}
		if scenario == "mcp-native" && mcpCalls.Load() != 1 {
			for _, step := range got.Steps {
				if step.ToolName == "mcp__docs__docs" {
					t.Logf("MCP saved step: type=%s result=%s content=%s", step.Type, step.ToolResult, step.Content)
				}
			}
			t.Fatal("MCP tool missing or repeated", mcpCalls.Load())
		}
		t.Logf("synthetic %s workflow: calls=%d launches=%d mcp=%d status=%s", scenario, calls, host.launches.Load(), mcpCalls.Load(), got.Status)
		return
	}
	if backend.creates.Load() != 1 || backend.inputs.Load() != 1 {
		for _, step := range got.Steps {
			if step.Type == "rejected" || step.Type == "error" {
				t.Log(step.ToolName, step.ToolResult)
			}
		}
		t.Fatalf("dispatch counts create=%d edit=%d; status=%s error=%s", backend.creates.Load(), backend.inputs.Load(), got.Status, got.Error)
	}
	if desktop && host.launches.Load() != 1 {
		t.Fatal("application launch missing or repeated", host.launches.Load())
	}
	if fault == "none" {
		if got.Status != agents.TaskCompleted || backend.text != input || got.Workflow.State != "checks_passed" {
			t.Fatalf("workflow failed: %s %s %+v", got.Status, got.Error, got.Workflow)
		}
	} else if fault == "lost-service-ack" {
		if got.Status != agents.TaskInterrupted || got.ErrorCode != "computer_result_recovered" || got.Checkpoint.ExecutingCall != "" || got.Result != "" {
			t.Fatal("durable receipt did not recover without resuming", got.Status, got.Error)
		}
		last := got.Steps[len(got.Steps)-1]
		if last.ToolName != "computer_insert_text" || last.Provenance != "companion_receipt" {
			t.Fatal("lost action-bound provenance", last)
		}
	} else if got.Status != agents.TaskUncertain {
		t.Fatalf("partial/uncertain input did not pause: %s %s", got.Status, got.Error)
	}
	if calls > cfg.MaxIterations {
		t.Fatal("unbounded model loop", calls)
	}
	if got.Context == nil || got.Context.EstimatedTokens+cfg.MaxTokens+512 > 8192 {
		t.Fatal("context allocation exceeded", got.Context)
	}
	if got.Prompt != task.Prompt || !strings.Contains(got.Prompt, "Do not save or change the existing document") {
		t.Fatal("original instructions changed")
	}
	t.Logf("synthetic document workflow: model_calls=%d created=%d edits=%d status=%s", calls, backend.creates.Load(), backend.inputs.Load(), got.Status)
}
