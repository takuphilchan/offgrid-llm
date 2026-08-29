package agents

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestToolEnabledStatePersists(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "tools.json")
	registry := NewToolRegistry()
	if err := registry.LoadUserTools(configPath); err != nil {
		t.Fatal(err)
	}
	if err := registry.DisableTool("calculator"); err != nil {
		t.Fatal(err)
	}

	reloaded := NewToolRegistry()
	if err := reloaded.LoadUserTools(configPath); err != nil {
		t.Fatal(err)
	}
	found := false
	for _, tool := range reloaded.GetAllToolsWithStatus() {
		if tool["name"] == "calculator" {
			found = true
			if tool["enabled"] != false {
				t.Fatalf("calculator enabled after reload: %#v", tool)
			}
		}
	}
	if !found {
		t.Fatal("calculator tool missing after reload")
	}
}

func TestMCPServerConfigurationPersists(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "tools.json")
	registry := NewToolRegistry()
	if err := registry.LoadUserTools(configPath); err != nil {
		t.Fatal(err)
	}
	if err := registry.PersistMCPServer(MCPServerConfig{Name: "documents", URL: "http://127.0.0.1:3000/mcp", Transport: "http"}); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatal(err)
	}
	var config ToolsConfig
	if err := json.Unmarshal(data, &config); err != nil {
		t.Fatal(err)
	}
	if len(config.MCPServers) != 1 || !config.MCPServers[0].Enabled || config.MCPServers[0].Name != "documents" {
		t.Fatalf("unexpected persisted MCP config: %#v", config.MCPServers)
	}
}

func TestApprovalWaitingTaskPersists(t *testing.T) {
	dataDir := t.TempDir()
	manager := NewManagerWithPersistence(nil, nil, nil, dataDir)
	task := manager.CreateTask("run-1", "read a protected file", nil)
	if err := manager.StartTask(task.ID); err != nil {
		t.Fatal(err)
	}
	if err := manager.WaitForApproval(task.ID, errors.New("approval required")); err != nil {
		t.Fatal(err)
	}

	reloaded := NewManagerWithPersistence(nil, nil, nil, dataDir)
	loaded, ok := reloaded.GetTask(task.ID)
	if !ok || loaded.Status != TaskWaiting {
		t.Fatalf("waiting task was not restored: %#v", loaded)
	}
}
