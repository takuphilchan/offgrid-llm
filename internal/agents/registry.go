package agents

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/takuphilchan/offgrid-llm/pkg/api"
)

// ToolRegistry provides a unified registry for all agent tools
// - Built-in tools (calculator, file ops, shell, etc.)
// - User-defined tools from config
// - MCP server tools (future)
type ToolRegistry struct {
	mu         sync.RWMutex
	tools      map[string]api.Tool
	executors  map[string]SimpleExecutor
	mcpClients map[string]*MCPClient
	configPath string
}

// SimpleExecutor wraps tool execution with just context and args
type SimpleExecutor func(ctx context.Context, args json.RawMessage) (string, error)

// UserDefinedTool represents a tool defined in user config
type UserDefinedTool struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	Parameters  map[string]interface{} `json:"parameters"`
	Type        string                 `json:"type"` // "shell", "http", "script"
	Command     string                 `json:"command,omitempty"`
	URL         string                 `json:"url,omitempty"`
	Script      string                 `json:"script,omitempty"`
}

// ToolsConfig is the user's tools configuration file
type ToolsConfig struct {
	Tools      []UserDefinedTool `json:"tools"`
	MCPServers []MCPServerConfig `json:"mcp_servers"`
}

// MCPServerConfig configures an MCP server connection
type MCPServerConfig struct {
	Name    string `json:"name"`
	URL     string `json:"url"`
	APIKey  string `json:"api_key,omitempty"`
	Enabled bool   `json:"enabled"`
}

// MCPClient is a placeholder for MCP server integration
type MCPClient struct {
	Name   string
	URL    string
	APIKey string
	Tools  []api.Tool
}

// NewToolRegistry creates a new tool registry with built-in tools
func NewToolRegistry() *ToolRegistry {
	r := &ToolRegistry{
		tools:      make(map[string]api.Tool),
		executors:  make(map[string]SimpleExecutor),
		mcpClients: make(map[string]*MCPClient),
	}

	// Register built-in tools
	r.registerBuiltInTools()

	return r
}

// registerBuiltInTools registers all built-in agent tools
func (r *ToolRegistry) registerBuiltInTools() {
	builtIns := BuiltInTools()
	for _, tool := range builtIns {
		r.tools[tool.Function.Name] = tool
	}

	// Register executors for built-in tools - they all use ExecuteTool
	builtInNames := []string{"calculator", "read_file", "write_file", "list_files", "shell", "http_get", "current_time"}
	for _, name := range builtInNames {
		toolName := name // capture for closure
		r.executors[name] = func(ctx context.Context, args json.RawMessage) (string, error) {
			return ExecuteTool(ctx, toolName, args)
		}
	}
}

// LoadUserTools loads user-defined tools from config file
func (r *ToolRegistry) LoadUserTools(configPath string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.configPath = configPath

	// Check if config file exists
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		// Create default config
		return r.createDefaultConfig(configPath)
	}

	data, err := os.ReadFile(configPath)
	if err != nil {
		return fmt.Errorf("failed to read tools config: %w", err)
	}

	var config ToolsConfig
	if err := json.Unmarshal(data, &config); err != nil {
		return fmt.Errorf("failed to parse tools config: %w", err)
	}

	// Register user-defined tools
	for _, ut := range config.Tools {
		tool := api.Tool{
			Type: "function",
			Function: api.FunctionDef{
				Name:        ut.Name,
				Description: ut.Description,
				Parameters:  ut.Parameters,
			},
		}
		r.tools[ut.Name] = tool
		r.executors[ut.Name] = r.createUserToolExecutor(ut)
	}

	// Connect to MCP servers
	for _, mcp := range config.MCPServers {
		if mcp.Enabled {
			if err := r.connectMCPServer(mcp); err != nil {
				// Log but don't fail
				fmt.Printf("Warning: Failed to connect to MCP server %s: %v\n", mcp.Name, err)
			}
		}
	}

	return nil
}

// createDefaultConfig creates a default tools config file with helpful comments
func (r *ToolRegistry) createDefaultConfig(configPath string) error {
	dir := filepath.Dir(configPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	// Create empty config - users can add their own tools and MCP servers
	config := ToolsConfig{
		Tools:      []UserDefinedTool{},
		MCPServers: []MCPServerConfig{},
	}

	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(configPath, data, 0644)
}

// createUserToolExecutor creates an executor for a user-defined tool
func (r *ToolRegistry) createUserToolExecutor(ut UserDefinedTool) SimpleExecutor {
	return func(ctx context.Context, args json.RawMessage) (string, error) {
		switch ut.Type {
		case "shell":
			// Execute shell command with argument substitution
			return executeUserShellTool(ctx, ut.Command, args)
		case "http":
			// Make HTTP request
			return executeUserHTTPTool(ctx, ut.URL, args)
		case "script":
			// Execute script file
			return executeUserScriptTool(ctx, ut.Script, args)
		default:
			return "", fmt.Errorf("unknown tool type: %s", ut.Type)
		}
	}
}

// connectMCPServer connects to an MCP server and registers its tools
func (r *ToolRegistry) connectMCPServer(config MCPServerConfig) error {
	// Create HTTP MCP client
	client := NewMCPHTTPClient(config.Name, config.URL, config.APIKey)

	// Connect and discover tools
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := client.Connect(ctx); err != nil {
		return fmt.Errorf("failed to connect to MCP server %s: %w", config.Name, err)
	}

	// Store client reference
	r.mcpClients[config.Name] = &MCPClient{
		Name:   config.Name,
		URL:    config.URL,
		APIKey: config.APIKey,
		Tools:  client.GetTools(),
	}

	// Register each tool from the MCP server
	for _, tool := range client.GetTools() {
		toolName := tool.Function.Name
		mcpClient := client // Capture for closure

		r.tools[toolName] = tool
		r.executors[toolName] = func(ctx context.Context, args json.RawMessage) (string, error) {
			return mcpClient.CallTool(ctx, toolName, args)
		}

		fmt.Printf("  - Registered MCP tool: %s\n", toolName)
	}

	return nil
}

// RegisterTool registers a tool dynamically
func (r *ToolRegistry) RegisterTool(tool api.Tool, executor SimpleExecutor) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.tools[tool.Function.Name] = tool
	r.executors[tool.Function.Name] = executor
}

// GetTools returns all registered tools
func (r *ToolRegistry) GetTools() []api.Tool {
	r.mu.RLock()
	defer r.mu.RUnlock()

	tools := make([]api.Tool, 0, len(r.tools))
	for _, tool := range r.tools {
		tools = append(tools, tool)
	}
	return tools
}

// GetTool returns a specific tool by name
func (r *ToolRegistry) GetTool(name string) (api.Tool, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	tool, ok := r.tools[name]
	return tool, ok
}

// Execute executes a tool by name
func (r *ToolRegistry) Execute(ctx context.Context, name string, args json.RawMessage) (string, error) {
	r.mu.RLock()
	executor, ok := r.executors[name]
	r.mu.RUnlock()

	if !ok {
		return "", fmt.Errorf("unknown tool: %s", name)
	}

	return executor(ctx, args)
}

// ListTools returns a formatted list of available tools
func (r *ToolRegistry) ListTools() string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var result string
	for name, tool := range r.tools {
		result += fmt.Sprintf("- %s: %s\n", name, tool.Function.Description)
	}
	return result
}

// Helper functions for user-defined tool execution

func executeUserShellTool(ctx context.Context, command string, args json.RawMessage) (string, error) {
	// Parse args and substitute into command
	var argsMap map[string]interface{}
	if len(args) > 0 {
		json.Unmarshal(args, &argsMap)
	}

	// Simple variable substitution: ${varname}
	for key, val := range argsMap {
		placeholder := fmt.Sprintf("${%s}", key)
		command = replaceAll(command, placeholder, fmt.Sprintf("%v", val))
	}

	return executeShell(ctx, command)
}

func executeUserHTTPTool(ctx context.Context, url string, args json.RawMessage) (string, error) {
	// Parse args for URL parameters
	var argsMap map[string]interface{}
	if len(args) > 0 {
		json.Unmarshal(args, &argsMap)
	}

	// Substitute URL parameters
	for key, val := range argsMap {
		placeholder := fmt.Sprintf("{%s}", key)
		url = replaceAll(url, placeholder, fmt.Sprintf("%v", val))
	}

	return executeHTTPGet(ctx, url)
}

func executeUserScriptTool(ctx context.Context, script string, args json.RawMessage) (string, error) {
	// Execute script file with args as JSON input
	return executeShell(ctx, fmt.Sprintf("%s '%s'", script, string(args)))
}

func replaceAll(s, old, new string) string {
	for {
		replaced := replaceOnce(s, old, new)
		if replaced == s {
			return s
		}
		s = replaced
	}
}

func replaceOnce(s, old, new string) string {
	i := indexOf(s, old)
	if i < 0 {
		return s
	}
	return s[:i] + new + s[i+len(old):]
}

func indexOf(s, substr string) int {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
}
