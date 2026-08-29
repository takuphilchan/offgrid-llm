package agents

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/takuphilchan/offgrid-llm/internal/capabilities"
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
	broker        *capabilities.Broker
}

// ToolExecution describes the authenticated run that is requesting a tool.
// Approval is deliberately scoped to one tool invocation rather than a whole
// agent run.
type ToolExecution struct {
	RunID    string
	Actor    string
	Approved bool
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
	Tools         []UserDefinedTool `json:"tools"`
	MCPServers    []MCPServerConfig `json:"mcp_servers"`
	DisabledTools []string          `json:"disabled_tools,omitempty"`
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
	official  *OfficialMCPClient
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
		r.toolSources[ut.Name] = "user"
		r.registerCapabilityLocked(ut.Name)
	}
	for _, name := range config.DisabledTools {
		if _, exists := r.tools[name]; exists {
			r.disabledTools[name] = true
		}
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
		client, err := ConnectMCPStdio(ctx, config.Name, config.Command, config.Args)
		if err != nil {
			return fmt.Errorf("failed to connect to MCP server %s: %w", config.Name, err)
		}

		tools = client.GetTools()
		mcpClient = &MCPClient{
			Name:      config.Name,
			Transport: "stdio",
			Tools:     tools,
			official:  client,
		}

		// Register tools with stdio executor
		for _, tool := range tools {
			remoteToolName := tool.Function.Name
			toolName := mcpToolName(config.Name, remoteToolName)
			tool.Function.Name = toolName
			sdkClient := client // Capture for closure

			r.tools[toolName] = tool
			r.toolSources[toolName] = "mcp:" + config.Name
			r.executors[toolName] = func(ctx context.Context, args json.RawMessage) (string, error) {
				return sdkClient.CallTool(ctx, remoteToolName, args)
			}
			r.registerCapabilityLocked(toolName)
			fmt.Printf("  - Registered MCP tool (stdio): %s\n", toolName)
		}
	} else {
		// HTTP transport
		client, err := ConnectMCPHTTP(ctx, config.Name, config.URL, config.APIKey)
		if err != nil {
			return fmt.Errorf("failed to connect to MCP server %s: %w", config.Name, err)
		}

		tools = client.GetTools()
		mcpClient = &MCPClient{
			Name:      config.Name,
			URL:       config.URL,
			Transport: "http",
			Tools:     tools,
			official:  client,
		}

		// Register tools with HTTP executor
		for _, tool := range tools {
			remoteToolName := tool.Function.Name
			toolName := mcpToolName(config.Name, remoteToolName)
			tool.Function.Name = toolName
			sdkClient := client // Capture for closure

			r.tools[toolName] = tool
			r.toolSources[toolName] = "mcp:" + config.Name
			r.executors[toolName] = func(ctx context.Context, args json.RawMessage) (string, error) {
				return sdkClient.CallTool(ctx, remoteToolName, args)
			}
			r.registerCapabilityLocked(toolName)
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
	r.toolSources[tool.Function.Name] = "extension"
	r.registerCapabilityLocked(tool.Function.Name)
}

// SetCapabilityBroker enables centralized authorization for all registered
// tools. Registries without a broker retain their original behavior, which is
// useful for isolated library consumers and tests.
func (r *ToolRegistry) SetCapabilityBroker(broker *capabilities.Broker) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.broker = broker
	for name := range r.tools {
		r.registerCapabilityLocked(name)
	}
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

		client, err := ConnectMCPStdio(ctx, name, parts[0], parts[1:])
		if err != nil {
			return 0, fmt.Errorf("failed to connect to MCP server %s: %w", name, err)
		}

		tools := client.GetTools()
		r.mcpClients[name] = &MCPClient{
			Name:      name,
			Transport: "stdio",
			Tools:     tools,
			official:  client,
		}

		count := 0
		for _, tool := range tools {
			remoteToolName := tool.Function.Name
			toolName := mcpToolName(name, remoteToolName)
			tool.Function.Name = toolName
			sdkClient := client

			r.tools[toolName] = tool
			r.toolSources[toolName] = "mcp:" + name
			r.executors[toolName] = func(ctx context.Context, args json.RawMessage) (string, error) {
				return sdkClient.CallTool(ctx, remoteToolName, args)
			}
			r.registerCapabilityLocked(toolName)
			fmt.Printf("  - Registered MCP tool (stdio): %s\n", toolName)
			count++
		}

		return count, nil
	}

	// HTTP transport
	client, err := ConnectMCPHTTP(ctx, name, urlOrCommand, "")
	if err != nil {
		return 0, fmt.Errorf("failed to connect to MCP server %s: %w", name, err)
	}

	tools := client.GetTools()
	r.mcpClients[name] = &MCPClient{
		Name:      name,
		URL:       urlOrCommand,
		Transport: "http",
		Tools:     tools,
		official:  client,
	}

	count := 0
	for _, tool := range tools {
		remoteToolName := tool.Function.Name
		toolName := mcpToolName(name, remoteToolName)
		tool.Function.Name = toolName
		sdkClient := client

		r.tools[toolName] = tool
		r.toolSources[toolName] = "mcp:" + name
		r.executors[toolName] = func(ctx context.Context, args json.RawMessage) (string, error) {
			return sdkClient.CallTool(ctx, remoteToolName, args)
		}
		r.registerCapabilityLocked(toolName)
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

		client, err := ConnectMCPStdio(ctx, "test", parts[0], parts[1:])
		if err != nil {
			return 0, err
		}
		defer client.Close()

		return len(client.GetTools()), nil
	}

	// HTTP transport
	client, err := ConnectMCPHTTP(ctx, "test", urlOrCommand, "")
	if err != nil {
		return 0, err
	}
	defer client.Close()

	// Return tool count without registering
	return len(client.GetTools()), nil
}

// GetMCPServers returns a list of connected MCP servers
func (r *ToolRegistry) GetMCPServers() []map[string]interface{} {
	r.mu.RLock()
	defer r.mu.RUnlock()

	servers := make([]map[string]interface{}, 0, len(r.mcpClients))
	for name, client := range r.mcpClients {
		servers = append(servers, map[string]interface{}{
			"name":      name,
			"url":       client.URL,
			"transport": client.Transport,
			"tools":     len(client.Tools),
			"status":    "connected",
		})
	}
	sort.Slice(servers, func(i, j int) bool { return servers[i]["name"].(string) < servers[j]["name"].(string) })
	return servers
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
	sort.Slice(tools, func(i, j int) bool { return tools[i].Function.Name < tools[j].Function.Name })
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
			"capability":  r.capabilityLocked(name),
		})
	}
	sort.Slice(tools, func(i, j int) bool { return tools[i]["name"].(string) < tools[j]["name"].(string) })
	return tools
}

// EnableTool enables a tool by name
func (r *ToolRegistry) EnableTool(name string) error {
	return r.SetToolEnabled(name, true)
}

// DisableTool disables a tool by name
func (r *ToolRegistry) DisableTool(name string) error {
	return r.SetToolEnabled(name, false)
}

// SetToolEnabled sets whether a tool is enabled or disabled
func (r *ToolRegistry) SetToolEnabled(name string, enabled bool) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.tools[name]; !ok {
		return fmt.Errorf("tool not found: %s", name)
	}
	if enabled {
		delete(r.disabledTools, name)
	} else {
		r.disabledTools[name] = true
	}
	return r.persistSettingsLocked(nil)
}

// PersistMCPServer records a successfully connected MCP server so the same
// connection and its discovered tools are restored after a service restart.
func (r *ToolRegistry) PersistMCPServer(server MCPServerConfig) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	server.Enabled = true
	return r.persistSettingsLocked(func(config *ToolsConfig) {
		for index := range config.MCPServers {
			if config.MCPServers[index].Name == server.Name {
				config.MCPServers[index] = server
				return
			}
		}
		config.MCPServers = append(config.MCPServers, server)
	})
}

func (r *ToolRegistry) persistSettingsLocked(update func(*ToolsConfig)) error {
	if r.configPath == "" {
		return nil
	}
	var config ToolsConfig
	data, err := os.ReadFile(r.configPath)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("read tools config: %w", err)
	}
	if len(data) > 0 {
		if err := json.Unmarshal(data, &config); err != nil {
			return fmt.Errorf("parse tools config: %w", err)
		}
	}
	if update != nil {
		update(&config)
	}
	config.DisabledTools = config.DisabledTools[:0]
	for name := range r.disabledTools {
		config.DisabledTools = append(config.DisabledTools, name)
	}
	sort.Strings(config.DisabledTools)
	encoded, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return fmt.Errorf("encode tools config: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(r.configPath), 0o755); err != nil {
		return err
	}
	return os.WriteFile(r.configPath, encoded, 0o600)
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
	return r.ExecuteWithPolicy(ctx, name, args, ToolExecution{})
}

// ExecuteWithPolicy authorizes and executes one tool invocation.
func (r *ToolRegistry) ExecuteWithPolicy(ctx context.Context, name string, args json.RawMessage, execution ToolExecution) (string, error) {
	r.mu.RLock()
	executor, ok := r.executors[name]
	disabled := r.disabledTools[name]
	broker := r.broker
	descriptor := r.capabilityLocked(name)
	r.mu.RUnlock()

	if !ok {
		return "", fmt.Errorf("unknown tool: %s", name)
	}
	if disabled {
		return "", fmt.Errorf("tool is disabled: %s", name)
	}
	if broker != nil {
		arguments := make(map[string]any)
		if len(args) > 0 && string(args) != "null" {
			if err := json.Unmarshal(args, &arguments); err != nil {
				return "", fmt.Errorf("decode tool arguments: %w", err)
			}
		}
		_, err := broker.Authorize(ctx, capabilities.Request{
			RunID:      execution.RunID,
			Actor:      execution.Actor,
			Capability: descriptor,
			Arguments:  arguments,
			Approved:   execution.Approved,
		})
		if err != nil {
			return "", fmt.Errorf("authorize %s: %w", name, err)
		}
	}

	return executor(ctx, args)
}

// Capability returns the policy descriptor associated with a tool.
func (r *ToolRegistry) Capability(name string) (capabilities.Descriptor, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if _, ok := r.tools[name]; !ok {
		return capabilities.Descriptor{}, false
	}
	return r.capabilityLocked(name), true
}

func (r *ToolRegistry) registerCapabilityLocked(name string) {
	if r.broker == nil {
		return
	}
	descriptor := r.capabilityLocked(name)
	if _, exists := r.broker.Resolve(descriptor.Name); exists {
		return
	}
	_ = r.broker.Register(descriptor)
}

func (r *ToolRegistry) capabilityLocked(name string) capabilities.Descriptor {
	source := r.toolSources[name]
	if source == "" {
		source = "unknown"
	}
	descriptor := capabilities.Descriptor{
		Name:      name,
		Namespace: "tools",
		Source:    source,
		Kind:      capabilities.Execute,
		Risk:      capabilities.RiskHigh,
	}
	if tool, ok := r.tools[name]; ok {
		descriptor.Description = tool.Function.Description
	}
	if source == "builtin" {
		switch name {
		case "calculator", "current_time":
			descriptor.Kind = capabilities.Read
			descriptor.Risk = capabilities.RiskLow
		case "read_file", "list_files":
			descriptor.Kind = capabilities.Read
			descriptor.Risk = capabilities.RiskMedium
		case "http_get":
			descriptor.Kind = capabilities.Network
			descriptor.Risk = capabilities.RiskMedium
		case "write_file":
			descriptor.Kind = capabilities.Write
		case "shell":
			descriptor.Kind = capabilities.Execute
		}
	} else if strings.HasPrefix(source, "mcp:") {
		descriptor.Namespace = "mcp"
	}
	return descriptor
}

func mcpToolName(server, tool string) string {
	clean := func(value string) string {
		var result strings.Builder
		for _, char := range strings.ToLower(value) {
			if (char >= 'a' && char <= 'z') || (char >= '0' && char <= '9') || char == '_' || char == '-' {
				result.WriteRune(char)
			} else {
				result.WriteByte('_')
			}
		}
		return strings.Trim(result.String(), "_")
	}
	return "mcp__" + clean(server) + "__" + clean(tool)
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
