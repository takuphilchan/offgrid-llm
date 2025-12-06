package mcp

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"sync"
)

// Protocol version
const MCPVersion = "2024-11-05"

// Message types
type MessageType string

const (
	MessageTypeRequest      MessageType = "request"
	MessageTypeResponse     MessageType = "response"
	MessageTypeNotification MessageType = "notification"
)

// JSON-RPC message structure
type JSONRPCMessage struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      *int            `json:"id,omitempty"`
	Method  string          `json:"method,omitempty"`
	Params  json.RawMessage `json:"params,omitempty"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *RPCError       `json:"error,omitempty"`
}

// RPCError represents a JSON-RPC error
type RPCError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    any    `json:"data,omitempty"`
}

// Standard error codes
const (
	ErrCodeParse          = -32700
	ErrCodeInvalidRequest = -32600
	ErrCodeMethodNotFound = -32601
	ErrCodeInvalidParams  = -32602
	ErrCodeInternal       = -32603
)

// InitializeParams for initialize request
type InitializeParams struct {
	ProtocolVersion string       `json:"protocolVersion"`
	Capabilities    Capabilities `json:"capabilities"`
	ClientInfo      ClientInfo   `json:"clientInfo"`
}

// ClientInfo identifies the client
type ClientInfo struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

// ServerInfo identifies the server
type ServerInfo struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

// Capabilities describes what the server/client supports
type Capabilities struct {
	Tools     *ToolsCapability     `json:"tools,omitempty"`
	Resources *ResourcesCapability `json:"resources,omitempty"`
	Prompts   *PromptsCapability   `json:"prompts,omitempty"`
	Logging   *LoggingCapability   `json:"logging,omitempty"`
}

// ToolsCapability for tool support
type ToolsCapability struct {
	ListChanged bool `json:"listChanged,omitempty"`
}

// ResourcesCapability for resource support
type ResourcesCapability struct {
	Subscribe   bool `json:"subscribe,omitempty"`
	ListChanged bool `json:"listChanged,omitempty"`
}

// PromptsCapability for prompt support
type PromptsCapability struct {
	ListChanged bool `json:"listChanged,omitempty"`
}

// LoggingCapability for logging support
type LoggingCapability struct{}

// InitializeResult is the response to initialize
type InitializeResult struct {
	ProtocolVersion string       `json:"protocolVersion"`
	Capabilities    Capabilities `json:"capabilities"`
	ServerInfo      ServerInfo   `json:"serverInfo"`
	Instructions    string       `json:"instructions,omitempty"`
}

// Tool represents an MCP tool
type Tool struct {
	Name        string          `json:"name"`
	Description string          `json:"description,omitempty"`
	InputSchema json.RawMessage `json:"inputSchema"`
}

// ToolsListResult is the response to tools/list
type ToolsListResult struct {
	Tools []Tool `json:"tools"`
}

// ToolCallParams for tools/call
type ToolCallParams struct {
	Name      string          `json:"name"`
	Arguments json.RawMessage `json:"arguments,omitempty"`
}

// ToolCallResult is the response to tools/call
type ToolCallResult struct {
	Content []ContentItem `json:"content"`
	IsError bool          `json:"isError,omitempty"`
}

// ContentItem represents content in a tool result
type ContentItem struct {
	Type     string `json:"type"` // "text", "image", "resource"
	Text     string `json:"text,omitempty"`
	MimeType string `json:"mimeType,omitempty"`
	Data     string `json:"data,omitempty"` // base64 for images
}

// Resource represents an MCP resource
type Resource struct {
	URI         string `json:"uri"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	MimeType    string `json:"mimeType,omitempty"`
}

// ResourcesListResult is the response to resources/list
type ResourcesListResult struct {
	Resources []Resource `json:"resources"`
}

// ResourceReadParams for resources/read
type ResourceReadParams struct {
	URI string `json:"uri"`
}

// ResourceReadResult is the response to resources/read
type ResourceReadResult struct {
	Contents []ResourceContent `json:"contents"`
}

// ResourceContent contains the actual resource data
type ResourceContent struct {
	URI      string `json:"uri"`
	MimeType string `json:"mimeType,omitempty"`
	Text     string `json:"text,omitempty"`
	Blob     string `json:"blob,omitempty"` // base64
}

// Prompt represents an MCP prompt template
type Prompt struct {
	Name        string           `json:"name"`
	Description string           `json:"description,omitempty"`
	Arguments   []PromptArgument `json:"arguments,omitempty"`
}

// PromptArgument describes a prompt argument
type PromptArgument struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	Required    bool   `json:"required,omitempty"`
}

// PromptsListResult is the response to prompts/list
type PromptsListResult struct {
	Prompts []Prompt `json:"prompts"`
}

// PromptGetParams for prompts/get
type PromptGetParams struct {
	Name      string            `json:"name"`
	Arguments map[string]string `json:"arguments,omitempty"`
}

// PromptGetResult is the response to prompts/get
type PromptGetResult struct {
	Description string          `json:"description,omitempty"`
	Messages    []PromptMessage `json:"messages"`
}

// PromptMessage is a message in a prompt
type PromptMessage struct {
	Role    string      `json:"role"` // "user" or "assistant"
	Content ContentItem `json:"content"`
}

// ToolHandler is a function that handles a tool call
type ToolHandler func(ctx context.Context, arguments json.RawMessage) (*ToolCallResult, error)

// ResourceHandler is a function that handles resource reads
type ResourceHandler func(ctx context.Context, uri string) (*ResourceReadResult, error)

// PromptHandler is a function that handles prompt gets
type PromptHandler func(ctx context.Context, name string, arguments map[string]string) (*PromptGetResult, error)

// Server is the MCP server
type Server struct {
	mu              sync.RWMutex
	name            string
	version         string
	instructions    string
	tools           map[string]Tool
	toolHandlers    map[string]ToolHandler
	resources       []Resource
	resourceHandler ResourceHandler
	prompts         []Prompt
	promptHandler   PromptHandler
	initialized     bool
	input           io.Reader
	output          io.Writer
	logger          *log.Logger
}

// NewServer creates a new MCP server
func NewServer(name, version string) *Server {
	return &Server{
		name:         name,
		version:      version,
		tools:        make(map[string]Tool),
		toolHandlers: make(map[string]ToolHandler),
		resources:    make([]Resource, 0),
		prompts:      make([]Prompt, 0),
		input:        os.Stdin,
		output:       os.Stdout,
		logger:       log.New(os.Stderr, "[MCP] ", log.LstdFlags),
	}
}

// SetInstructions sets the server instructions
func (s *Server) SetInstructions(instructions string) {
	s.instructions = instructions
}

// SetIO sets custom input/output streams
func (s *Server) SetIO(input io.Reader, output io.Writer) {
	s.input = input
	s.output = output
}

// RegisterTool registers a tool with its handler
func (s *Server) RegisterTool(tool Tool, handler ToolHandler) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.tools[tool.Name] = tool
	s.toolHandlers[tool.Name] = handler
}

// RegisterResource registers a resource
func (s *Server) RegisterResource(resource Resource) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.resources = append(s.resources, resource)
}

// SetResourceHandler sets the handler for resource reads
func (s *Server) SetResourceHandler(handler ResourceHandler) {
	s.resourceHandler = handler
}

// RegisterPrompt registers a prompt template
func (s *Server) RegisterPrompt(prompt Prompt) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.prompts = append(s.prompts, prompt)
}

// SetPromptHandler sets the handler for prompt gets
func (s *Server) SetPromptHandler(handler PromptHandler) {
	s.promptHandler = handler
}

// Run starts the MCP server (stdio mode)
func (s *Server) Run(ctx context.Context) error {
	s.logger.Println("Starting MCP server in stdio mode...")

	scanner := bufio.NewScanner(s.input)
	// Increase buffer size for large messages
	buf := make([]byte, 0, 64*1024)
	scanner.Buffer(buf, 10*1024*1024) // 10MB max

	for scanner.Scan() {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		line := scanner.Text()
		if line == "" {
			continue
		}

		var msg JSONRPCMessage
		if err := json.Unmarshal([]byte(line), &msg); err != nil {
			s.sendError(nil, ErrCodeParse, "Failed to parse JSON", err.Error())
			continue
		}

		s.handleMessage(ctx, &msg)
	}

	if err := scanner.Err(); err != nil {
		return fmt.Errorf("scanner error: %w", err)
	}

	return nil
}

// handleMessage processes an incoming message
func (s *Server) handleMessage(ctx context.Context, msg *JSONRPCMessage) {
	switch msg.Method {
	case "initialize":
		s.handleInitialize(ctx, msg)
	case "initialized":
		s.logger.Println("Client sent initialized notification")
	case "ping":
		s.handlePing(msg)
	case "tools/list":
		s.handleToolsList(msg)
	case "tools/call":
		s.handleToolsCall(ctx, msg)
	case "resources/list":
		s.handleResourcesList(msg)
	case "resources/read":
		s.handleResourcesRead(ctx, msg)
	case "prompts/list":
		s.handlePromptsList(msg)
	case "prompts/get":
		s.handlePromptsGet(ctx, msg)
	case "notifications/cancelled":
		// Handle cancellation
		s.logger.Println("Received cancellation notification")
	default:
		if msg.ID != nil {
			s.sendError(msg.ID, ErrCodeMethodNotFound, "Method not found", msg.Method)
		}
	}
}

// handleInitialize processes the initialize request
func (s *Server) handleInitialize(ctx context.Context, msg *JSONRPCMessage) {
	var params InitializeParams
	if err := json.Unmarshal(msg.Params, &params); err != nil {
		s.sendError(msg.ID, ErrCodeInvalidParams, "Invalid params", err.Error())
		return
	}

	s.logger.Printf("Initializing with client: %s %s", params.ClientInfo.Name, params.ClientInfo.Version)

	caps := Capabilities{
		Tools:     &ToolsCapability{ListChanged: true},
		Resources: &ResourcesCapability{Subscribe: false, ListChanged: true},
		Prompts:   &PromptsCapability{ListChanged: true},
		Logging:   &LoggingCapability{},
	}

	result := InitializeResult{
		ProtocolVersion: MCPVersion,
		Capabilities:    caps,
		ServerInfo: ServerInfo{
			Name:    s.name,
			Version: s.version,
		},
		Instructions: s.instructions,
	}

	s.initialized = true
	s.sendResult(msg.ID, result)
}

// handlePing responds to ping
func (s *Server) handlePing(msg *JSONRPCMessage) {
	s.sendResult(msg.ID, map[string]string{})
}

// handleToolsList returns the list of available tools
func (s *Server) handleToolsList(msg *JSONRPCMessage) {
	s.mu.RLock()
	tools := make([]Tool, 0, len(s.tools))
	for _, tool := range s.tools {
		tools = append(tools, tool)
	}
	s.mu.RUnlock()

	s.sendResult(msg.ID, ToolsListResult{Tools: tools})
}

// handleToolsCall executes a tool
func (s *Server) handleToolsCall(ctx context.Context, msg *JSONRPCMessage) {
	var params ToolCallParams
	if err := json.Unmarshal(msg.Params, &params); err != nil {
		s.sendError(msg.ID, ErrCodeInvalidParams, "Invalid params", err.Error())
		return
	}

	s.mu.RLock()
	handler, exists := s.toolHandlers[params.Name]
	s.mu.RUnlock()

	if !exists {
		s.sendError(msg.ID, ErrCodeInvalidParams, "Tool not found", params.Name)
		return
	}

	result, err := handler(ctx, params.Arguments)
	if err != nil {
		s.sendResult(msg.ID, ToolCallResult{
			Content: []ContentItem{{Type: "text", Text: err.Error()}},
			IsError: true,
		})
		return
	}

	s.sendResult(msg.ID, result)
}

// handleResourcesList returns the list of available resources
func (s *Server) handleResourcesList(msg *JSONRPCMessage) {
	s.mu.RLock()
	resources := make([]Resource, len(s.resources))
	copy(resources, s.resources)
	s.mu.RUnlock()

	s.sendResult(msg.ID, ResourcesListResult{Resources: resources})
}

// handleResourcesRead reads a resource
func (s *Server) handleResourcesRead(ctx context.Context, msg *JSONRPCMessage) {
	var params ResourceReadParams
	if err := json.Unmarshal(msg.Params, &params); err != nil {
		s.sendError(msg.ID, ErrCodeInvalidParams, "Invalid params", err.Error())
		return
	}

	if s.resourceHandler == nil {
		s.sendError(msg.ID, ErrCodeInternal, "No resource handler", nil)
		return
	}

	result, err := s.resourceHandler(ctx, params.URI)
	if err != nil {
		s.sendError(msg.ID, ErrCodeInternal, "Resource read failed", err.Error())
		return
	}

	s.sendResult(msg.ID, result)
}

// handlePromptsList returns the list of available prompts
func (s *Server) handlePromptsList(msg *JSONRPCMessage) {
	s.mu.RLock()
	prompts := make([]Prompt, len(s.prompts))
	copy(prompts, s.prompts)
	s.mu.RUnlock()

	s.sendResult(msg.ID, PromptsListResult{Prompts: prompts})
}

// handlePromptsGet returns a prompt
func (s *Server) handlePromptsGet(ctx context.Context, msg *JSONRPCMessage) {
	var params PromptGetParams
	if err := json.Unmarshal(msg.Params, &params); err != nil {
		s.sendError(msg.ID, ErrCodeInvalidParams, "Invalid params", err.Error())
		return
	}

	if s.promptHandler == nil {
		s.sendError(msg.ID, ErrCodeInternal, "No prompt handler", nil)
		return
	}

	result, err := s.promptHandler(ctx, params.Name, params.Arguments)
	if err != nil {
		s.sendError(msg.ID, ErrCodeInternal, "Prompt get failed", err.Error())
		return
	}

	s.sendResult(msg.ID, result)
}

// sendResult sends a successful response
func (s *Server) sendResult(id *int, result any) {
	resultJSON, err := json.Marshal(result)
	if err != nil {
		s.logger.Printf("Failed to marshal result: %v", err)
		return
	}

	msg := JSONRPCMessage{
		JSONRPC: "2.0",
		ID:      id,
		Result:  resultJSON,
	}

	s.sendMessage(msg)
}

// sendError sends an error response
func (s *Server) sendError(id *int, code int, message string, data any) {
	msg := JSONRPCMessage{
		JSONRPC: "2.0",
		ID:      id,
		Error: &RPCError{
			Code:    code,
			Message: message,
			Data:    data,
		},
	}

	s.sendMessage(msg)
}

// sendNotification sends a notification (no response expected)
func (s *Server) sendNotification(method string, params any) {
	paramsJSON, err := json.Marshal(params)
	if err != nil {
		s.logger.Printf("Failed to marshal notification params: %v", err)
		return
	}

	msg := JSONRPCMessage{
		JSONRPC: "2.0",
		Method:  method,
		Params:  paramsJSON,
	}

	s.sendMessage(msg)
}

// sendMessage sends a JSON-RPC message
func (s *Server) sendMessage(msg JSONRPCMessage) {
	data, err := json.Marshal(msg)
	if err != nil {
		s.logger.Printf("Failed to marshal message: %v", err)
		return
	}

	fmt.Fprintln(s.output, string(data))
}

// LogMessage sends a log message notification
func (s *Server) LogMessage(level, message string, data any) {
	s.sendNotification("notifications/message", map[string]any{
		"level":   level,
		"logger":  s.name,
		"message": message,
		"data":    data,
	})
}
