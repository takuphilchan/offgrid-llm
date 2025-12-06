package agents

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
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
