package agents

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// ConnectAndPersistMCP serializes connection changes with removal so a pending
// connect cannot recreate a saved connection after RemoveMCPServer returns.
func (r *ToolRegistry) ConnectAndPersistMCP(name, endpoint string) (int, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	count, err := r.loadMCPToolsLocked(name, endpoint)
	if err != nil {
		return 0, err
	}
	config := MCPServerConfig{Name: name, URL: endpoint, Transport: "http"}
	if client := r.mcpClients[name]; client.Transport == "stdio" {
		parts := strings.Fields(endpoint)
		config.URL, config.Transport, config.Command, config.Args = "", "stdio", parts[0], parts[1:]
	}
	if err := r.persistMCPServerLocked(config); err != nil {
		client := r.mcpClients[name]
		r.unregisterMCPToolsLocked(name)
		delete(r.mcpClients, name)
		if client != nil {
			_ = client.official.Close()
		}
		return 0, fmt.Errorf("save MCP connection: %w", err)
	}
	return count, nil
}

func (r *ToolRegistry) validateMCPNameLocked(name string) error {
	if strings.TrimSpace(name) == "" {
		return fmt.Errorf("MCP connection name is required")
	}
	if _, exists := r.mcpClients[name]; exists {
		return fmt.Errorf("MCP connection already exists; remove it before reconnecting")
	}
	// Different display names must not own the same normalized tool namespace.
	// Otherwise removing either connection could silently strand the other.
	for other := range r.mcpConfigs {
		if other != name && mcpToolName(other, "") == mcpToolName(name, "") {
			return fmt.Errorf("MCP connection name conflicts with a saved connection; choose a distinct name")
		}
	}
	for other := range r.mcpClients {
		if mcpToolName(other, "") == mcpToolName(name, "") {
			return fmt.Errorf("MCP connection name conflicts with an active connection; choose a distinct name")
		}
	}
	return nil
}

func (r *ToolRegistry) validateMCPToolsLocked(name string, client *OfficialMCPClient) error {
	seen := make(map[string]bool)
	for _, tool := range client.GetTools() {
		key := mcpToolName(name, tool.Function.Name)
		_, exists := r.tools[key]
		if seen[key] || exists {
			_ = client.Close()
			return fmt.Errorf("MCP tool names conflict with existing tools")
		}
		seen[key] = true
	}
	return nil
}

// RemoveMCPServer forgets the saved connection before revoking its executors.
// On a persistence error the connection remains intact. Repeated removal is
// idempotent. Closing the transport does not undo effects already dispatched.
func (r *ToolRegistry) RemoveMCPServer(name string) (int, error) {
	r.mu.Lock()
	if strings.TrimSpace(name) == "" {
		r.mu.Unlock()
		return 0, fmt.Errorf("MCP connection name is required")
	}
	removedTools := make(map[string]bool)
	for tool, source := range r.toolSources {
		if source == "mcp:"+name {
			removedTools[tool] = true
		}
	}
	err := r.persistSettingsLocked(func(config *ToolsConfig) {
		kept := config.MCPServers[:0]
		for _, server := range config.MCPServers {
			if server.Name != name {
				kept = append(kept, server)
			}
		}
		config.MCPServers = kept
		disabled := config.DisabledTools[:0]
		for _, tool := range config.DisabledTools {
			if !removedTools[tool] {
				disabled = append(disabled, tool)
			}
		}
		config.DisabledTools = disabled
	})
	if err != nil {
		r.mu.Unlock()
		return 0, err
	}
	client := r.mcpClients[name]
	r.unregisterMCPToolsLocked(name)
	delete(r.mcpClients, name)
	delete(r.mcpConfigs, name)
	r.mu.Unlock()
	// Close outside the registry lock: a remote close must not stall unrelated
	// local tools. The client has already been removed from dispatch/discovery.
	if client != nil {
		_ = client.official.Close()
	}
	return len(removedTools), nil
}

func (r *ToolRegistry) unregisterMCPToolsLocked(name string) {
	for tool, source := range r.toolSources {
		if source != "mcp:"+name {
			continue
		}
		delete(r.tools, tool)
		delete(r.executors, tool)
		delete(r.toolSources, tool)
		delete(r.disabledTools, tool)
		if r.broker != nil {
			r.broker.Unregister(tool, source)
		}
	}
}

// Replace atomically rather than truncating tools.json: a failed write must
// neither erase other connections nor falsely acknowledge a durable removal.
func writeToolsConfig(path string, data []byte) error {
	f, err := os.CreateTemp(filepath.Dir(path), ".tools-*.tmp")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	if err = f.Chmod(0o600); err == nil {
		_, err = f.Write(data)
	}
	if err == nil {
		err = f.Sync()
	}
	closeErr := f.Close()
	if err != nil {
		return err
	}
	if closeErr != nil {
		return closeErr
	}
	return os.Rename(f.Name(), path)
}
