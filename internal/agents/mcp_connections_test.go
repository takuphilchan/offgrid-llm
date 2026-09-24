package agents

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/takuphilchan/offgrid-llm/internal/capabilities"
)

func mcpRemovalFixture(t *testing.T) (*ToolRegistry, string, string) {
	t.Helper()
	remote := httptest.NewServer(NewToolRegistry().MCPHTTPHandler("removal-test"))
	t.Cleanup(remote.Close)
	r := NewToolRegistry()
	path := filepath.Join(t.TempDir(), "tools.json")
	if err := r.LoadUserTools(path); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		for _, c := range r.mcpClients {
			_ = c.official.Close()
		}
	})
	return r, path, remote.URL
}

func TestMCPRemovalRevokesToolsClosesSessionAndSurvivesRestart(t *testing.T) {
	r, path, endpoint := mcpRemovalFixture(t)
	name := "Docs & research / 日本語"
	for _, n := range []string{name, "keep"} {
		if _, err := r.ConnectAndPersistMCP(n, endpoint); err != nil {
			t.Fatal(err)
		}
	}
	tool := mcpToolName(name, "calculator")
	args := json.RawMessage(`{"expression":"2+2"}`)
	if value, err := r.Execute(context.Background(), tool, args); err != nil || value != "4" {
		t.Fatalf("real MCP call: %q %v", value, err)
	}
	cachedExecutor := r.executors[tool]
	if err := r.DisableTool(tool); err != nil {
		t.Fatal(err)
	}
	if err := r.DisableTool("shell"); err != nil {
		t.Fatal(err)
	}
	if err := r.DisableTool(mcpToolName("keep", "calculator")); err != nil {
		t.Fatal(err)
	}
	broker := capabilities.NewBroker(nil)
	r.SetCapabilityBroker(broker)
	if _, err := r.ConnectAndPersistMCP(name, endpoint); err == nil {
		t.Fatal("duplicate silently replaced live connection")
	}
	if _, err := r.ConnectAndPersistMCP("KEEP", endpoint); err == nil {
		t.Fatal("different display name silently replaced another connection's tools")
	}
	removed, err := r.RemoveMCPServer(name)
	if err != nil || removed != 7 {
		t.Fatalf("remove=%d err=%v", removed, err)
	}
	if _, ok := r.GetTool(tool); ok {
		t.Fatal("removed tool still discoverable")
	}
	if _, ok := broker.Resolve(tool); ok {
		t.Fatal("removed capability still advertised")
	}
	if _, err := r.Execute(context.Background(), tool, args); err == nil {
		t.Fatal("removed tool executable")
	}
	if _, err := cachedExecutor(context.Background(), args); err == nil {
		t.Fatal("closed session remains executable through cached executor")
	}
	if _, ok := r.GetTool(mcpToolName("keep", "calculator")); !ok {
		t.Fatal("other connection removed")
	}
	if _, ok := r.GetTool("calculator"); !ok {
		t.Fatal("builtin removed")
	}
	if count, err := r.RemoveMCPServer(name); err != nil || count != 0 {
		t.Fatalf("retry: %d %v", count, err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var saved ToolsConfig
	if err := json.Unmarshal(data, &saved); err != nil {
		t.Fatal(err)
	}
	if len(saved.MCPServers) != 1 || saved.MCPServers[0].Name != "keep" {
		t.Fatalf("saved connections: %#v", saved.MCPServers)
	}
	if len(saved.DisabledTools) != 2 || saved.DisabledTools[0] != mcpToolName("keep", "calculator") || saved.DisabledTools[1] != "shell" {
		t.Fatalf("disabled tools: %#v", saved.DisabledTools)
	}
	reloaded := NewToolRegistry()
	if err := reloaded.LoadUserTools(path); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		for _, c := range reloaded.mcpClients {
			_ = c.official.Close()
		}
	})
	servers := reloaded.GetMCPServers()
	if len(servers) != 1 || servers[0]["name"] != "keep" {
		t.Fatalf("reloaded connections: %#v", servers)
	}
	if _, err := reloaded.Execute(context.Background(), mcpToolName("keep", "calculator"), args); err == nil {
		t.Fatal("unrelated disabled MCP tool was enabled by restart")
	}
}

func TestMCPRemovalRetainsConnectionOnCorruptStorage(t *testing.T) {
	r, path, endpoint := mcpRemovalFixture(t)
	if _, err := r.ConnectAndPersistMCP("docs", endpoint); err != nil {
		t.Fatal(err)
	}
	corrupt := []byte(`{"mcp_servers":`)
	if err := os.WriteFile(path, corrupt, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := r.RemoveMCPServer("docs"); err == nil {
		t.Fatal("corrupt settings silently discarded")
	}
	if len(r.GetMCPServers()) != 1 {
		t.Fatal("failed removal changed live connection")
	}
	if _, err := r.Execute(context.Background(), mcpToolName("docs", "calculator"), json.RawMessage(`{"expression":"2+2"}`)); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(path)
	if string(data) != string(corrupt) {
		t.Fatal("failed removal overwrote original")
	}
	if _, err := r.ConnectAndPersistMCP("new", endpoint); err == nil {
		t.Fatal("connection acknowledged without persistence")
	}
	if _, ok := r.GetTool(mcpToolName("new", "calculator")); ok {
		t.Fatal("failed connect leaked tools")
	}
}

func TestMCPRemovalIncludesDisconnectedAndDisabledConnections(t *testing.T) {
	path := filepath.Join(t.TempDir(), "tools.json")
	config := ToolsConfig{MCPServers: []MCPServerConfig{
		{Name: "offline", URL: "http://127.0.0.1:1/mcp", Enabled: true},
		{Name: "disabled", Command: "not-executed", Enabled: false},
	}, Tools: []UserDefinedTool{{Name: "kept-user-tool", Type: "http", URL: "https://example.com"}}}
	data, _ := json.Marshal(config)
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	r := NewToolRegistry()
	if err := r.LoadUserTools(path); err != nil {
		t.Fatal(err)
	}
	servers := r.GetMCPServers()
	if len(servers) != 2 || servers[0]["status"] != "disabled" || servers[1]["status"] != "disconnected" {
		t.Fatalf("missing offline connections: %#v", servers)
	}
	for _, name := range []string{"offline", "disabled"} {
		if _, err := r.RemoveMCPServer(name); err != nil {
			t.Fatal(err)
		}
	}
	reloaded := NewToolRegistry()
	if err := reloaded.LoadUserTools(path); err != nil {
		t.Fatal(err)
	}
	if len(reloaded.GetMCPServers()) != 0 {
		t.Fatal("offline connection returned after restart")
	}
	if _, ok := reloaded.GetTool("kept-user-tool"); !ok {
		t.Fatal("unrelated custom tool lost")
	}
}
