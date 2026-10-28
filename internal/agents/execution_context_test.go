package agents

import (
	"encoding/json"
	"github.com/takuphilchan/offgrid-llm/internal/execution"
	"strings"
	"testing"
)

func TestExecutionBackendAndPolicyValidation(t *testing.T) {
	ref := execution.Reference{Host: "host", Session: "session", Grant: "grant", Revision: 1}
	config := DefaultAgentConfig()
	config.Execution = &ref
	config.ExecutionWorkspace = &execution.Resource{Reference: ref, Backend: "isolated_linux", Root: "/approved", Directory: ".", Permissions: execution.Permissions{Read: true, Write: true, Processes: true}}
	if err := config.validateExecution(); err != nil {
		t.Fatal("isolated admission rejected", err)
	}
	limit := execution.Permissions{Read: true}
	config.Profile = &AgentProfile{Settings: ProfileSettings{Execution: &limit}}
	if err := config.validateExecution(); err == nil {
		t.Fatal("profile ceiling omitted")
	}
	selected := execution.Permissions{Read: true, Write: true}
	config.ExecutionPolicy = &selected
	if err := config.validateExecution(); err == nil {
		t.Fatal("profile policy widened")
	}
	selected.Write = false
	if err := config.validateExecution(); err != nil {
		t.Fatal(err)
	}
	config.ExecutionWorkspace.Backend = "unknown"
	if err := config.validateExecution(); err == nil {
		t.Fatal("unknown backend accepted")
	}
}

func TestExecutionContextPersistsWithoutRestoringAuthority(t *testing.T) {
	dir := t.TempDir()
	manager := NewManagerWithPersistence(nil, nil, nil, dir)
	runner := NewRunner(manager, &runTestTools{}, nil)
	config := taskConfig()
	config.ContextWindow = 8192
	config.MaxTokens = 512
	ref := execution.Reference{Host: "host", Session: "session", Grant: "grant", Revision: 2}
	resource := execution.Resource{Reference: ref, Backend: "local", Root: "/fixture/approved", Directory: "src", Permissions: execution.Permissions{Read: true}}
	config.Execution = &ref
	config.ExecutionWorkspace = &resource
	task, err := runner.Create("read the workspace", "model", "alice", config)
	if err != nil {
		t.Fatal(err)
	}
	stored := NewManagerWithPersistence(nil, nil, nil, dir)
	loaded, ok := stored.GetTask(task.ID)
	if !ok || loaded.Config.ExecutionWorkspace == nil || *loaded.Config.ExecutionWorkspace != resource {
		t.Fatal("context lost", loaded)
	}
	messages, err := runner.prepareContext(loaded, nil)
	if err != nil {
		t.Fatal(err)
	}
	encoded, _ := json.Marshal(messages)
	for _, expected := range []string{"/fixture/approved", "initial_directory_display", "untrusted data", "not the service filesystem", "workspace_info"} {
		if !strings.Contains(string(encoded), expected) {
			t.Fatal("lost admission context", expected, string(encoded))
		}
	}
	if strings.Contains(executionResourceContext(config), `"grant"`) {
		t.Fatal("unnecessary grant identifier reached model")
	}
	if !strings.Contains(executionResourceContext(config), `"granted_permissions":{"read":true,"write":false,"processes":false}`) {
		t.Fatal("current permission subset was not exposed accurately", executionResourceContext(config))
	}
	// Snapshot metadata cannot exist independently or redirect its reference.
	config.Execution = nil
	if _, err = runner.Create("invalid", "model", "alice", config); err == nil {
		t.Fatal("unbound context accepted")
	}
}
