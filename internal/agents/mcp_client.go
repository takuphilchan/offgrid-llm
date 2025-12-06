package agents

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/takuphilchan/offgrid-llm/pkg/api"
)

// MCPTransport defines how to communicate with an MCP server
type MCPTransport string

const (
	MCPTransportHTTP  MCPTransport = "http"
	MCPTransportStdio MCPTransport = "stdio"
)

// MCPRequest is a JSON-RPC 2.0 request
type MCPRequest struct {
	JSONRPC string      `json:"jsonrpc"`
	ID      int64       `json:"id"`
	Method  string      `json:"method"`
	Params  interface{} `json:"params,omitempty"`
}

// MCPResponse is a JSON-RPC 2.0 response
type MCPResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      int64           `json:"id"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *MCPError       `json:"error,omitempty"`
}

// MCPError represents a JSON-RPC error
type MCPError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    any    `json:"data,omitempty"`
}

func (e *MCPError) Error() string {
	return fmt.Sprintf("MCP error %d: %s", e.Code, e.Message)
}

// MCPToolInfo represents tool information from MCP
type MCPToolInfo struct {
	Name        string          `json:"name"`
	Description string          `json:"description,omitempty"`
	InputSchema json.RawMessage `json:"inputSchema,omitempty"`
}

// MCPToolsListResult is the result of tools/list
type MCPToolsListResult struct {
	Tools []MCPToolInfo `json:"tools"`
}

// MCPCallToolParams are params for tools/call
type MCPCallToolParams struct {
	Name      string                 `json:"name"`
	Arguments map[string]interface{} `json:"arguments,omitempty"`
}

// MCPCallToolResult is the result of tools/call
type MCPCallToolResult struct {
	Content []MCPContent `json:"content"`
	IsError bool         `json:"isError,omitempty"`
}

// MCPContent represents content in MCP responses
type MCPContent struct {
	Type string `json:"type"`
	Text string `json:"text,omitempty"`
}

// MCPHTTPClient connects to an MCP server over HTTP
type MCPHTTPClient struct {
	name      string
	baseURL   string
	apiKey    string
	client    *http.Client
	requestID int64
	tools     []api.Tool
}

// NewMCPHTTPClient creates a new MCP client for HTTP transport
func NewMCPHTTPClient(name, baseURL, apiKey string) *MCPHTTPClient {
	return &MCPHTTPClient{
		name:    name,
		baseURL: baseURL,
		apiKey:  apiKey,
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// Connect initializes the connection and discovers tools
func (c *MCPHTTPClient) Connect(ctx context.Context) error {
	// First, try to initialize
	_, err := c.call(ctx, "initialize", map[string]interface{}{
		"protocolVersion": "2024-11-05",
		"capabilities":    map[string]interface{}{},
		"clientInfo": map[string]interface{}{
			"name":    "offgrid-llm",
			"version": "0.2.2",
		},
	})
	if err != nil {
		// Some servers don't require initialize, try to list tools anyway
		fmt.Printf("MCP initialize skipped for %s: %v\n", c.name, err)
	}

	// Discover available tools
	tools, err := c.ListTools(ctx)
	if err != nil {
		return fmt.Errorf("failed to list tools from %s: %w", c.name, err)
	}

	c.tools = tools
	fmt.Printf("MCP server %s: discovered %d tools\n", c.name, len(tools))
	return nil
}

// ListTools retrieves available tools from the MCP server
func (c *MCPHTTPClient) ListTools(ctx context.Context) ([]api.Tool, error) {
	result, err := c.call(ctx, "tools/list", nil)
	if err != nil {
		return nil, err
	}

	var listResult MCPToolsListResult
	if err := json.Unmarshal(result, &listResult); err != nil {
		return nil, fmt.Errorf("failed to parse tools list: %w", err)
	}

	tools := make([]api.Tool, 0, len(listResult.Tools))
	for _, t := range listResult.Tools {
		// Convert MCP tool schema to our api.Tool format
		var params map[string]interface{}
		if len(t.InputSchema) > 0 {
			json.Unmarshal(t.InputSchema, &params)
		}

		tools = append(tools, api.Tool{
			Type: "function",
			Function: api.FunctionDef{
				Name:        t.Name,
				Description: t.Description,
				Parameters:  params,
			},
		})
	}

	return tools, nil
}

// CallTool invokes a tool on the MCP server
func (c *MCPHTTPClient) CallTool(ctx context.Context, name string, args json.RawMessage) (string, error) {
	var argsMap map[string]interface{}
	if len(args) > 0 {
		if err := json.Unmarshal(args, &argsMap); err != nil {
			return "", fmt.Errorf("invalid arguments: %w", err)
		}
	}

	result, err := c.call(ctx, "tools/call", MCPCallToolParams{
		Name:      name,
		Arguments: argsMap,
	})
	if err != nil {
		return "", err
	}

	var callResult MCPCallToolResult
	if err := json.Unmarshal(result, &callResult); err != nil {
		return "", fmt.Errorf("failed to parse tool result: %w", err)
	}

	// Extract text content
	var output string
	for _, content := range callResult.Content {
		if content.Type == "text" {
			output += content.Text
		}
	}

	if callResult.IsError {
		return "", fmt.Errorf("tool error: %s", output)
	}

	return output, nil
}

// GetTools returns the discovered tools
func (c *MCPHTTPClient) GetTools() []api.Tool {
	return c.tools
}

// call makes a JSON-RPC call to the MCP server
func (c *MCPHTTPClient) call(ctx context.Context, method string, params interface{}) (json.RawMessage, error) {
	id := atomic.AddInt64(&c.requestID, 1)

	req := MCPRequest{
		JSONRPC: "2.0",
		ID:      id,
		Method:  method,
		Params:  params,
	}

	reqBody, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", c.baseURL, bytes.NewReader(reqBody))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	if c.apiKey != "" {
		httpReq.Header.Set("Authorization", "Bearer "+c.apiKey)
	}

	resp, err := c.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("server returned %d: %s", resp.StatusCode, string(body))
	}

	var mcpResp MCPResponse
	if err := json.Unmarshal(body, &mcpResp); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	if mcpResp.Error != nil {
		return nil, mcpResp.Error
	}

	return mcpResp.Result, nil
}

// Name returns the client name
func (c *MCPHTTPClient) Name() string {
	return c.name
}

// MCPStdioClient connects to an MCP server over stdio (spawns subprocess)
type MCPStdioClient struct {
	name      string
	command   string
	args      []string
	cmd       *exec.Cmd
	stdin     io.WriteCloser
	stdout    *bufio.Reader
	requestID int64
	tools     []api.Tool
	mu        sync.Mutex
}

// NewMCPStdioClient creates a new MCP client for stdio transport
func NewMCPStdioClient(name, command string, args []string) *MCPStdioClient {
	return &MCPStdioClient{
		name:    name,
		command: command,
		args:    args,
	}
}

// Connect starts the subprocess and initializes the connection
func (c *MCPStdioClient) Connect(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Resolve the command path - check common locations for npx/node
	command := c.command
	if command == "npx" || command == "node" {
		// Try to find in common locations
		searchPaths := []string{
			command, // Current PATH
			"/usr/local/bin/" + command,
			"/usr/bin/" + command,
		}

		// Check nvm locations
		homeDir, _ := os.UserHomeDir()
		if homeDir != "" {
			// Add common nvm paths
			nvmPaths, _ := filepath.Glob(homeDir + "/.nvm/versions/node/*/bin/" + command)
			if len(nvmPaths) > 0 {
				// Use the latest (last) version
				searchPaths = append(searchPaths, nvmPaths[len(nvmPaths)-1])
			}
		}

		// Find first existing path
		for _, path := range searchPaths {
			if _, err := exec.LookPath(path); err == nil {
				command = path
				break
			}
		}
	}

	// Start the subprocess with inherited environment
	c.cmd = exec.CommandContext(ctx, command, c.args...)

	// Inherit environment so npm/npx work correctly
	c.cmd.Env = os.Environ()

	var err error
	c.stdin, err = c.cmd.StdinPipe()
	if err != nil {
		return fmt.Errorf("failed to get stdin pipe: %w", err)
	}

	stdout, err := c.cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("failed to get stdout pipe: %w", err)
	}
	c.stdout = bufio.NewReader(stdout)

	// Capture stderr for debugging
	var stderr bytes.Buffer
	c.cmd.Stderr = &stderr

	if err := c.cmd.Start(); err != nil {
		return fmt.Errorf("failed to start MCP server '%s': %w", c.command, err)
	}

	// Give the server a moment to initialize
	time.Sleep(500 * time.Millisecond)

	// Check if process is still running
	select {
	case <-ctx.Done():
		c.cmd.Process.Kill()
		return ctx.Err()
	default:
	}

	// Initialize the connection
	_, err = c.callLocked(ctx, "initialize", map[string]interface{}{
		"protocolVersion": "2024-11-05",
		"capabilities":    map[string]interface{}{},
		"clientInfo": map[string]interface{}{
			"name":    "offgrid-llm",
			"version": "0.2.2",
		},
	})
	if err != nil {
		c.Close()
		return fmt.Errorf("failed to initialize: %w", err)
	}

	// Send initialized notification
	c.sendNotificationLocked("notifications/initialized", nil)

	// Discover tools - need to release lock and use public method
	c.mu.Unlock()
	tools, err := c.ListTools(ctx)
	c.mu.Lock()
	if err != nil {
		c.Close()
		return fmt.Errorf("failed to list tools: %w", err)
	}
	c.tools = tools

	return nil
}

// ListTools returns available tools from the MCP server
func (c *MCPStdioClient) ListTools(ctx context.Context) ([]api.Tool, error) {
	result, err := c.call(ctx, "tools/list", nil)
	if err != nil {
		return nil, err
	}

	var toolsResult MCPToolsListResult
	if err := json.Unmarshal(result, &toolsResult); err != nil {
		return nil, fmt.Errorf("failed to parse tools: %w", err)
	}

	// Convert to api.Tool format
	tools := make([]api.Tool, 0, len(toolsResult.Tools))
	for _, t := range toolsResult.Tools {
		var params map[string]interface{}
		if len(t.InputSchema) > 0 {
			json.Unmarshal(t.InputSchema, &params)
		}
		if params == nil {
			params = map[string]interface{}{"type": "object", "properties": map[string]interface{}{}}
		}

		tools = append(tools, api.Tool{
			Type: "function",
			Function: api.FunctionDef{
				Name:        t.Name,
				Description: t.Description,
				Parameters:  params,
			},
		})
	}

	return tools, nil
}

// CallTool invokes a tool on the MCP server
func (c *MCPStdioClient) CallTool(ctx context.Context, name string, args json.RawMessage) (string, error) {
	var argsMap map[string]interface{}
	if len(args) > 0 {
		if err := json.Unmarshal(args, &argsMap); err != nil {
			return "", fmt.Errorf("invalid arguments: %w", err)
		}
	}

	result, err := c.call(ctx, "tools/call", MCPCallToolParams{
		Name:      name,
		Arguments: argsMap,
	})
	if err != nil {
		return "", err
	}

	var callResult MCPCallToolResult
	if err := json.Unmarshal(result, &callResult); err != nil {
		return "", fmt.Errorf("failed to parse result: %w", err)
	}

	// Extract text content
	var texts []string
	for _, content := range callResult.Content {
		if content.Type == "text" && content.Text != "" {
			texts = append(texts, content.Text)
		}
	}

	if callResult.IsError {
		return "", fmt.Errorf("tool error: %s", strings.Join(texts, "\n"))
	}

	return strings.Join(texts, "\n"), nil
}

// GetTools returns the discovered tools
func (c *MCPStdioClient) GetTools() []api.Tool {
	return c.tools
}

// Name returns the client name
func (c *MCPStdioClient) Name() string {
	return c.name
}

// Close stops the subprocess
func (c *MCPStdioClient) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.stdin != nil {
		c.stdin.Close()
	}
	if c.cmd != nil && c.cmd.Process != nil {
		c.cmd.Process.Kill()
		c.cmd.Wait()
	}
	return nil
}

// call makes a JSON-RPC call to the MCP server via stdio
// Note: caller must NOT hold c.mu lock (or use callLocked instead)
func (c *MCPStdioClient) call(ctx context.Context, method string, params interface{}) (json.RawMessage, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.callLocked(ctx, method, params)
}

// callLocked makes a JSON-RPC call - caller must already hold c.mu
func (c *MCPStdioClient) callLocked(ctx context.Context, method string, params interface{}) (json.RawMessage, error) {
	id := atomic.AddInt64(&c.requestID, 1)

	req := MCPRequest{
		JSONRPC: "2.0",
		ID:      id,
		Method:  method,
		Params:  params,
	}

	reqBytes, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	// Write request followed by newline
	if _, err := c.stdin.Write(append(reqBytes, '\n')); err != nil {
		return nil, fmt.Errorf("failed to write request: %w", err)
	}

	// Read response line
	line, err := c.stdout.ReadBytes('\n')
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	var resp MCPResponse
	if err := json.Unmarshal(line, &resp); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	if resp.Error != nil {
		return nil, resp.Error
	}

	return resp.Result, nil
}

// sendNotification sends a JSON-RPC notification (no response expected)
func (c *MCPStdioClient) sendNotification(method string, params interface{}) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.sendNotificationLocked(method, params)
}

// sendNotificationLocked sends a notification - caller must hold c.mu
func (c *MCPStdioClient) sendNotificationLocked(method string, params interface{}) error {
	notification := map[string]interface{}{
		"jsonrpc": "2.0",
		"method":  method,
	}
	if params != nil {
		notification["params"] = params
	}

	notifBytes, err := json.Marshal(notification)
	if err != nil {
		return err
	}

	_, err = c.stdin.Write(append(notifBytes, '\n'))
	return err
}
