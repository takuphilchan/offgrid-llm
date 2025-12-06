package agents

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/takuphilchan/offgrid-llm/pkg/api"
)

// ToolRegistry provides a unified registry for all agent tools
// - Built-in tools (calculator, file ops, shell, etc.)
// - User-defined tools from config
// - MCP server tools (future)
type ToolRegistry struct {
	mu            sync.RWMutex
	tools         map[string]api.Tool
	executors     map[string]SimpleExecutor
	mcpClients    map[string]*MCPClient
	configPath    string
	disabledTools map[string]bool   // Tools that are disabled
	toolSources   map[string]string // Maps tool name to source (builtin, mcp:servername, user)
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
	Name      string   `json:"name"`
	URL       string   `json:"url,omitempty"`       // For HTTP transport
	Command   string   `json:"command,omitempty"`   // For stdio transport (e.g., "npx")
	Args      []string `json:"args,omitempty"`      // Command arguments
	Transport string   `json:"transport,omitempty"` // "http" or "stdio" (default: auto-detect)
	APIKey    string   `json:"api_key,omitempty"`
	Enabled   bool     `json:"enabled"`
}

// MCPClient wraps an MCP connection (HTTP or stdio)
type MCPClient struct {
	Name      string
	URL       string
	Transport string
	Tools     []api.Tool
	// Underlying client (either HTTP or Stdio)
	httpClient  *MCPHTTPClient
	stdioClient *MCPStdioClient
}

// NewToolRegistry creates a new tool registry with built-in tools
func NewToolRegistry() *ToolRegistry {
	r := &ToolRegistry{
		tools:         make(map[string]api.Tool),
		executors:     make(map[string]SimpleExecutor),
		mcpClients:    make(map[string]*MCPClient),
		disabledTools: make(map[string]bool),
		toolSources:   make(map[string]string),
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
		r.toolSources[tool.Function.Name] = "builtin"
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
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	// Determine transport type
	transport := config.Transport
	if transport == "" {
		// Auto-detect based on config
		if config.Command != "" {
			transport = "stdio"
		} else if config.URL != "" {
			transport = "http"
		} else {
			return fmt.Errorf("MCP server %s: must specify either 'url' (for HTTP) or 'command' (for stdio)", config.Name)
		}
	}

	var tools []api.Tool
	var mcpClient *MCPClient

	if transport == "stdio" {
		// Stdio transport - spawn subprocess
		client := NewMCPStdioClient(config.Name, config.Command, config.Args)
		if err := client.Connect(ctx); err != nil {
			return fmt.Errorf("failed to connect to MCP server %s: %w", config.Name, err)
		}

		tools = client.GetTools()
		mcpClient = &MCPClient{
			Name:        config.Name,
			Transport:   "stdio",
			Tools:       tools,
			stdioClient: client,
		}

		// Register tools with stdio executor
		for _, tool := range tools {
			toolName := tool.Function.Name
			stdioClient := client // Capture for closure

			r.tools[toolName] = tool
			r.executors[toolName] = func(ctx context.Context, args json.RawMessage) (string, error) {
				return stdioClient.CallTool(ctx, toolName, args)
			}
			fmt.Printf("  - Registered MCP tool (stdio): %s\n", toolName)
		}
	} else {
		// HTTP transport
		client := NewMCPHTTPClient(config.Name, config.URL, config.APIKey)
		if err := client.Connect(ctx); err != nil {
			return fmt.Errorf("failed to connect to MCP server %s: %w", config.Name, err)
		}

		tools = client.GetTools()
		mcpClient = &MCPClient{
			Name:       config.Name,
			URL:        config.URL,
			Transport:  "http",
			Tools:      tools,
			httpClient: client,
		}

		// Register tools with HTTP executor
		for _, tool := range tools {
			toolName := tool.Function.Name
			httpClient := client // Capture for closure

			r.tools[toolName] = tool
			r.executors[toolName] = func(ctx context.Context, args json.RawMessage) (string, error) {
				return httpClient.CallTool(ctx, toolName, args)
			}
			fmt.Printf("  - Registered MCP tool (http): %s\n", toolName)
		}
	}

	// Store client reference
	r.mcpClients[config.Name] = mcpClient

	return nil
}

// RegisterTool registers a tool dynamically
func (r *ToolRegistry) RegisterTool(tool api.Tool, executor SimpleExecutor) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.tools[tool.Function.Name] = tool
	r.executors[tool.Function.Name] = executor
}

// LoadMCPTools connects to an MCP server and loads its tools dynamically
// Supports both HTTP URLs and stdio commands
func (r *ToolRegistry) LoadMCPTools(name, urlOrCommand string) (int, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Detect if it's a command (starts with common executables) or URL
	isCommand := strings.HasPrefix(urlOrCommand, "npx ") ||
		strings.HasPrefix(urlOrCommand, "node ") ||
		strings.HasPrefix(urlOrCommand, "python ") ||
		strings.HasPrefix(urlOrCommand, "./") ||
		strings.HasPrefix(urlOrCommand, "/")

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	if isCommand {
		// Parse command and args
		parts := strings.Fields(urlOrCommand)
		if len(parts) == 0 {
			return 0, fmt.Errorf("empty command")
		}

		client := NewMCPStdioClient(name, parts[0], parts[1:])
		if err := client.Connect(ctx); err != nil {
			return 0, fmt.Errorf("failed to connect to MCP server %s: %w", name, err)
		}

		tools := client.GetTools()
		r.mcpClients[name] = &MCPClient{
			Name:        name,
			Transport:   "stdio",
			Tools:       tools,
			stdioClient: client,
		}

		count := 0
		for _, tool := range tools {
			toolName := tool.Function.Name
			stdioClient := client

			r.tools[toolName] = tool
			r.toolSources[toolName] = "mcp:" + name
			r.executors[toolName] = func(ctx context.Context, args json.RawMessage) (string, error) {
				return stdioClient.CallTool(ctx, toolName, args)
			}
			fmt.Printf("  - Registered MCP tool (stdio): %s\n", toolName)
			count++
		}

		return count, nil
	}

	// HTTP transport
	client := NewMCPHTTPClient(name, urlOrCommand, "")
	if err := client.Connect(ctx); err != nil {
		return 0, fmt.Errorf("failed to connect to MCP server %s: %w", name, err)
	}

	tools := client.GetTools()
	r.mcpClients[name] = &MCPClient{
		Name:       name,
		URL:        urlOrCommand,
		Transport:  "http",
		Tools:      tools,
		httpClient: client,
	}

	count := 0
	for _, tool := range tools {
		toolName := tool.Function.Name
		httpClient := client

		r.tools[toolName] = tool
		r.toolSources[toolName] = "mcp:" + name
		r.executors[toolName] = func(ctx context.Context, args json.RawMessage) (string, error) {
			return httpClient.CallTool(ctx, toolName, args)
		}
		fmt.Printf("  - Registered MCP tool (http): %s\n", toolName)
		count++
	}

	return count, nil
}

// TestMCPConnection tests connectivity to an MCP server without registering tools
// Supports both HTTP URLs and stdio commands
func (r *ToolRegistry) TestMCPConnection(urlOrCommand string) (int, error) {
	// Detect if it's a command or URL
	isCommand := strings.HasPrefix(urlOrCommand, "npx ") ||
		strings.HasPrefix(urlOrCommand, "node ") ||
		strings.HasPrefix(urlOrCommand, "python ") ||
		strings.HasPrefix(urlOrCommand, "./") ||
		strings.HasPrefix(urlOrCommand, "/")

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	if isCommand {
		parts := strings.Fields(urlOrCommand)
		if len(parts) == 0 {
			return 0, fmt.Errorf("empty command")
		}

		client := NewMCPStdioClient("test", parts[0], parts[1:])
		if err := client.Connect(ctx); err != nil {
			return 0, err
		}
		defer client.Close()

		return len(client.GetTools()), nil
	}

	// HTTP transport
	client := NewMCPHTTPClient("test", urlOrCommand, "")
	if err := client.Connect(ctx); err != nil {
		return 0, err
	}

	// Return tool count without registering
	return len(client.GetTools()), nil
}

// GetTools returns all enabled tools (filters out disabled ones)
func (r *ToolRegistry) GetTools() []api.Tool {
	r.mu.RLock()
	defer r.mu.RUnlock()

	tools := make([]api.Tool, 0, len(r.tools))
	for name, tool := range r.tools {
		if !r.disabledTools[name] {
			tools = append(tools, tool)
		}
	}
	return tools
}

// GetAllToolsWithStatus returns all tools with their enabled/disabled status
func (r *ToolRegistry) GetAllToolsWithStatus() []map[string]interface{} {
	r.mu.RLock()
	defer r.mu.RUnlock()

	tools := make([]map[string]interface{}, 0, len(r.tools))
	for name, tool := range r.tools {
		source := r.toolSources[name]
		if source == "" {
			source = "unknown"
		}
		tools = append(tools, map[string]interface{}{
			"name":        name,
			"description": tool.Function.Description,
			"enabled":     !r.disabledTools[name],
			"source":      source,
		})
	}
	return tools
}

// EnableTool enables a tool by name
func (r *ToolRegistry) EnableTool(name string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, ok := r.tools[name]; !ok {
		return fmt.Errorf("tool not found: %s", name)
	}
	delete(r.disabledTools, name)
	return nil
}

// DisableTool disables a tool by name
func (r *ToolRegistry) DisableTool(name string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, ok := r.tools[name]; !ok {
		return fmt.Errorf("tool not found: %s", name)
	}
	r.disabledTools[name] = true
	return nil
}

// SetToolEnabled sets whether a tool is enabled or disabled
func (r *ToolRegistry) SetToolEnabled(name string, enabled bool) error {
	if enabled {
		return r.EnableTool(name)
	}
	return r.DisableTool(name)
}

// GetEnabledCount returns the count of enabled tools
func (r *ToolRegistry) GetEnabledCount() int {
	r.mu.RLock()
	defer r.mu.RUnlock()

	count := 0
	for name := range r.tools {
		if !r.disabledTools[name] {
			count++
		}
	}
	return count
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
