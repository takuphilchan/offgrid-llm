package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/takuphilchan/offgrid-llm/internal/agents"
	"github.com/takuphilchan/offgrid-llm/internal/computer"
)

func latestApplicationQuery(task *agents.Task, name string) string {
	step := 0
	for _, s := range task.Steps {
		if s.Type == "action" && s.ToolName == "computer_applications" {
			step = s.ID
		}
	}
	data, _ := json.Marshal(applicationQuery{Query: name, CatalogStep: step})
	return string(data)
}

func appendApplicationLookup(t *testing.T, task *agents.Task, name string, catalog int) {
	t.Helper()
	args, _ := json.Marshal(applicationQuery{Query: name, CatalogStep: catalog})
	result, err := applicationSearchResult(task, args)
	if err != nil {
		t.Fatal(err)
	}
	task.Steps = append(task.Steps, agents.Step{ID: len(task.Steps) + 1, Type: "action", ToolName: applicationSearchTool, ToolArgs: string(args), ToolResult: result})
}

func TestApplicationLookupBindsLaunchAndRejectsSubstitution(t *testing.T) {
	task := &agents.Task{Config: agents.AgentConfig{ComputerScope: "desktop", ComputerSession: "session"}, Steps: []agents.Step{{ID: 1, Type: "action", ToolName: "computer_applications", ToolResult: `{"applications":[{"id":"access","name":"Access"},{"id":"settings","name":"Settings"},{"id":"advanced","name":"Advanced Settings"},{"id":"localized","name":"設定"}]}`}}}
	if err := validateApplicationLookup(task, "settings"); err == nil {
		t.Fatal("launch without lookup")
	}
	appendApplicationLookup(t, task, "Settings", 1)
	if err := validateApplicationLookup(task, "settings"); err != nil {
		t.Fatal(err)
	}
	adapter := &browserRunTools{}
	if err := adapter.ValidateDeclaredExpectations(task, []agents.OutcomeExpectation{{ID: "wrong", Kind: "application_open", ApplicationID: "access"}}); err == nil {
		t.Fatal("unrelated immutable application expectation accepted")
	}
	if err := adapter.ValidateDeclaredExpectations(task, []agents.OutcomeExpectation{{ID: "correct", Kind: "application_open", ApplicationID: "settings"}}); err != nil {
		t.Fatal(err)
	}
	if err := validateApplicationLookup(task, "access"); err == nil {
		t.Fatal("Access substituted for Settings")
	}
	appendApplicationLookup(t, task, "sett", 1)
	if err := validateApplicationLookup(task, "settings"); err == nil {
		t.Fatal("ambiguous fragment selected first app")
	}
	appendApplicationLookup(t, task, "設定", 1)
	if err := validateApplicationLookup(task, "localized"); err != nil {
		t.Fatal(err)
	}
	task.Steps = append(task.Steps, agents.Step{ID: len(task.Steps) + 1, Type: "action", ToolName: "computer_applications", ToolResult: task.Steps[0].ToolResult})
	if err := validateApplicationLookup(task, "localized"); err == nil {
		t.Fatal("catalog refresh retained old lookup")
	}
	appendApplicationLookup(t, task, "設定", len(task.Steps))
	task.Config.ComputerStepOffset = len(task.Steps)
	if err := validateApplicationLookup(task, "localized"); err == nil {
		t.Fatal("session replacement retained old lookup")
	}
}

func TestApplicationMismatchRejectedByDispatchBeforeHostContact(t *testing.T) {
	s, tools, _, _, _ := compositionFixture(t)
	runner := agents.NewRunner(s.agentManager, tools, nil)
	config := compositionConfig("windows-uia", true)
	config.ComputerScope = "desktop"
	task, err := runner.Create("Open Settings", "fixture", "alice", config)
	if err != nil {
		t.Fatal(err)
	}
	task.Config.ComputerScope = "desktop"
	task.Steps = []agents.Step{{ID: 1, Type: "action", ToolName: "computer_applications", ToolResult: `{"applications":[{"id":"access","name":"Access"},{"id":"settings","name":"Settings"}]}`}}
	appendApplicationLookup(t, task, "Settings", 1)
	for _, step := range task.Steps {
		if err := s.agentManager.AddTaskStep(task.ID, step); err != nil {
			t.Fatal(err)
		}
	}
	err = tools.Authorize(context.Background(), "computer_open_application", json.RawMessage(`{"application_id":"access"}`), agents.ToolExecution{Actor: "alice", RunID: task.ID})
	var rejection *agents.ToolAuthorizationError
	if !errors.As(err, &rejection) || rejection.Code != "computer_application_lookup_required" {
		t.Fatalf("not an actionable pre-dispatch rejection: %v", err)
	}
}

func TestApplicationLookupRejectsAmbiguityRevocationAndChangedIntent(t *testing.T) {
	for _, mode := range []string{"duplicate names", "missing", "revoked", "changed intent", "failed newer lookup", "forged result"} {
		t.Run(mode, func(t *testing.T) {
			task := &agents.Task{Config: agents.AgentConfig{ComputerScope: "desktop", ComputerSession: "session"}, Steps: []agents.Step{{ID: 1, Type: "action", ToolName: "computer_applications", ToolResult: `{"applications":[{"id":"one","name":"Editor"}]}`}}}
			query := "Editor"
			if mode == "duplicate names" {
				task.Steps[0].ToolResult = `{"applications":[{"id":"one","name":"Editor"},{"id":"two","name":"Editor"}]}`
			}
			if mode == "missing" || mode == "forged result" {
				query = "Missing"
			}
			appendApplicationLookup(t, task, query, 1)
			switch mode {
			case "revoked":
				task.ComputerSessionExpired = true
			case "changed intent":
				task.Instructions = []agents.Instruction{{At: time.Now(), Text: "Open a different app instead"}}
			case "failed newer lookup":
				task.Steps = append(task.Steps, agents.Step{ID: 3, Type: "tool_error", ToolName: applicationSearchTool})
			case "forged result":
				task.Steps[1].ToolResult = `{"applications":[{"id":"one","name":"Editor"}],"total_matches":1}`
			}
			if err := validateApplicationLookup(task, "one"); err == nil {
				t.Fatal("invalid selection allowed")
			}
		})
	}
}

func TestApplicationSearchFindsOmittedNamesAndInvalidatesOldCatalog(t *testing.T) {
	apps := make([]computer.NativeApplication, 95)
	for i := range apps {
		apps[i] = computer.NativeApplication{ID: fmt.Sprintf("app-%d", i), Name: fmt.Sprintf("Fixture %d", i)}
	}
	apps[0].Name = "Access"
	apps[61].Name = "Settings"
	apps[61].Aliases = []string{"System preferences", "設定アプリ"}
	apps[94].Name = "設定"
	data, _ := json.Marshal(map[string]any{"applications": apps})
	task := &agents.Task{Config: agents.AgentConfig{ComputerSession: "session", ComputerScope: "desktop"}, Steps: []agents.Step{{ID: 1, Type: "action", ToolName: "computer_applications", ToolResult: string(data)}}}
	for _, test := range []struct{ query, id string }{{"sett", "app-61"}, {"設定", "app-94"}, {"System preferences", "app-61"}, {"設定アプリ", "app-61"}} {
		args, _ := json.Marshal(applicationQuery{Query: test.query, CatalogStep: 1})
		result, err := applicationSearchResult(task, args)
		if err != nil || !strings.Contains(result, test.id) || strings.Contains(result, "Access") {
			t.Fatalf("lookup failed: %s %v", result, err)
		}
	}
	result, err := applicationSearchResult(task, json.RawMessage(`{"query":"Fixture","catalog_step":1}`))
	if err != nil || !strings.Contains(result, `"next_offset":16`) {
		t.Fatalf("unbounded search: %s %v", result, err)
	}
	task.Steps = append(task.Steps, agents.Step{ID: 2, Type: "action", ToolName: "computer_applications", ToolResult: string(data)})
	if _, err = applicationSearchResult(task, json.RawMessage(`{"query":"Settings","catalog_step":1}`)); err == nil {
		t.Fatal("accepted old catalog cursor")
	}
	task.ComputerSessionExpired = true
	if _, err = applicationSearchResult(task, json.RawMessage(`{"query":"Settings","catalog_step":2}`)); err == nil {
		t.Fatal("searched revoked session")
	}
	for _, args := range []string{`null`, `{"query":null,"catalog_step":1}`, `{"query":" ","catalog_step":1}`, `{"catalog_step":1}`, `{"query":"x","catalog_step":1,"offset":-1}`, `{"query":"x","catalog_step":1,"command":"run"}`} {
		if _, err := parseApplicationQuery(json.RawMessage(args)); err == nil {
			t.Fatalf("accepted invalid query: %s", args)
		}
	}
}

func TestDesktopInstructionsComposeConversationAndProfileBehavior(t *testing.T) {
	for _, version := range []int{0, agents.WorkflowEvidenceVersion, agents.DocumentWorkflowEvidenceVersion} {
		cfg := agents.DefaultAgentConfig()
		cfg.NativeWorkflowVersion = version
		cfg.Instructions = "Respond briefly in the user's language."
		configureDesktopTask(&cfg)
		first := cfg.SystemPrompt
		configureDesktopTask(&cfg)
		if first != cfg.SystemPrompt || !strings.Contains(first, cfg.Instructions) || !strings.Contains(first, "Answer conversational requests directly") || !strings.Contains(first, "Never repeat uncertain input") {
			t.Fatal("lost or duplicated protocol sections")
		}
		if (version > 0) != strings.Contains(first, "task_expectations(application_open)") {
			t.Fatal("wrong expectation protocol")
		}
		if (version == agents.DocumentWorkflowEvidenceVersion) != strings.Contains(first, documentTaskProtocol) {
			t.Fatal("wrong document protocol")
		}
	}
}

func TestConversationProtocolPreservedAcrossComputerProfiles(t *testing.T) {
	for _, driver := range []string{"windows-uia", "macos-accessibility", "linux-atspi", "managed-browser"} {
		for _, taskFirst := range []bool{false, true} {
			cfg := agents.DefaultAgentConfig()
			cfg.ComputerDriver, cfg.TaskFirst = driver, taskFirst
			configureComputerTask(&cfg)
			if !strings.Contains(cfg.SystemPrompt, conversationTaskProtocol) || !strings.Contains(cfg.SystemPrompt, "when a tool is needed") || !strings.Contains(cfg.SystemPrompt, "A final answer or clarification has no tool call") {
				t.Fatalf("%s forced action for conversation (task-first=%v)", driver, taskFirst)
			}
		}
	}
}
