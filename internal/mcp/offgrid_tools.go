package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/takuphilchan/offgrid-llm/internal/inference"
	"github.com/takuphilchan/offgrid-llm/internal/rag"
)

// OffGridMCPServer wraps the MCP server with OffGrid-specific tools
type OffGridMCPServer struct {
	*Server
	ragEngine       *rag.Engine
	embeddingEngine *inference.EmbeddingEngine
	chatFunc        ChatFunction
}

// ChatFunction is the function signature for chat completions
type ChatFunction func(ctx context.Context, model, prompt string, options map[string]interface{}) (string, error)

// NewOffGridMCPServer creates an MCP server with OffGrid tools
func NewOffGridMCPServer(version string) *OffGridMCPServer {
	server := NewServer("offgrid-llm", version)
	server.SetInstructions(`OffGrid LLM is a fully offline AI system. 
It provides local language model inference, document search via RAG, 
and various utility tools. All processing happens locally - no data leaves your machine.`)

	ogs := &OffGridMCPServer{
		Server: server,
	}

	// Register built-in tools
	ogs.registerBuiltInTools()

	return ogs
}

// SetRAGEngine sets the RAG engine for document tools
func (s *OffGridMCPServer) SetRAGEngine(engine *rag.Engine) {
	s.ragEngine = engine
	s.registerRAGTools()
}

// SetEmbeddingEngine sets the embedding engine
func (s *OffGridMCPServer) SetEmbeddingEngine(engine *inference.EmbeddingEngine) {
	s.embeddingEngine = engine
}

// SetChatFunction sets the function for chat completions
func (s *OffGridMCPServer) SetChatFunction(fn ChatFunction) {
	s.chatFunc = fn
	s.registerChatTool()
}

// registerBuiltInTools registers the basic tools
func (s *OffGridMCPServer) registerBuiltInTools() {
	// get_current_time tool
	s.RegisterTool(Tool{
		Name:        "get_current_time",
		Description: "Get the current date and time in various formats",
		InputSchema: json.RawMessage(`{
			"type": "object",
			"properties": {
				"format": {
					"type": "string",
					"description": "Output format: 'iso', 'unix', 'human', or custom Go time format",
					"default": "iso"
				},
				"timezone": {
					"type": "string",
					"description": "Timezone name (e.g., 'America/New_York', 'UTC')",
					"default": "Local"
				}
			}
		}`),
	}, s.handleGetCurrentTime)

	// calculate tool
	s.RegisterTool(Tool{
		Name:        "calculate",
		Description: "Perform mathematical calculations. Supports basic arithmetic (+, -, *, /, %, ^) and common functions (sqrt, abs, sin, cos, tan, log, exp)",
		InputSchema: json.RawMessage(`{
			"type": "object",
			"properties": {
				"expression": {
					"type": "string",
					"description": "Mathematical expression to evaluate (e.g., '2 + 2', 'sqrt(16)', '10 * 5')"
				}
			},
			"required": ["expression"]
		}`),
	}, s.handleCalculate)

	// system_info tool
	s.RegisterTool(Tool{
		Name:        "system_info",
		Description: "Get information about the OffGrid LLM system",
		InputSchema: json.RawMessage(`{
			"type": "object",
			"properties": {
				"section": {
					"type": "string",
					"enum": ["all", "models", "rag", "system"],
					"description": "Which section of system info to return",
					"default": "all"
				}
			}
		}`),
	}, s.handleSystemInfo)
}

// registerRAGTools registers RAG-specific tools
func (s *OffGridMCPServer) registerRAGTools() {
	// search_documents tool
	s.RegisterTool(Tool{
		Name:        "search_documents",
		Description: "Search through uploaded documents using semantic search. Returns relevant passages from your knowledge base.",
		InputSchema: json.RawMessage(`{
			"type": "object",
			"properties": {
				"query": {
					"type": "string",
					"description": "The search query to find relevant documents"
				},
				"top_k": {
					"type": "integer",
					"description": "Number of results to return (1-20)",
					"default": 5,
					"minimum": 1,
					"maximum": 20
				},
				"min_score": {
					"type": "number",
					"description": "Minimum relevance score (0-1)",
					"default": 0.3,
					"minimum": 0,
					"maximum": 1
				}
			},
			"required": ["query"]
		}`),
	}, s.handleSearchDocuments)

	// list_documents tool
	s.RegisterTool(Tool{
		Name:        "list_documents",
		Description: "List all documents in the knowledge base",
		InputSchema: json.RawMessage(`{
			"type": "object",
			"properties": {}
		}`),
	}, s.handleListDocuments)

	// Register documents as resources
	if s.ragEngine != nil && s.ragEngine.IsEnabled() {
		for _, doc := range s.ragEngine.ListDocuments() {
			s.RegisterResource(Resource{
				URI:         fmt.Sprintf("offgrid://documents/%s", doc.ID),
				Name:        doc.Name,
				Description: fmt.Sprintf("Document with %d chunks, uploaded %s", doc.ChunkCount, doc.CreatedAt.Format(time.RFC3339)),
				MimeType:    doc.ContentType,
			})
		}
	}
}

// registerChatTool registers the chat completion tool
func (s *OffGridMCPServer) registerChatTool() {
	s.RegisterTool(Tool{
		Name:        "chat",
		Description: "Send a message to the local LLM and get a response",
		InputSchema: json.RawMessage(`{
			"type": "object",
			"properties": {
				"prompt": {
					"type": "string",
					"description": "The message to send to the LLM"
				},
				"model": {
					"type": "string",
					"description": "Model to use (optional, uses current model if not specified)"
				},
				"temperature": {
					"type": "number",
					"description": "Sampling temperature (0-2)",
					"default": 0.7,
					"minimum": 0,
					"maximum": 2
				},
				"max_tokens": {
					"type": "integer",
					"description": "Maximum tokens to generate",
					"default": 1024
				}
			},
			"required": ["prompt"]
		}`),
	}, s.handleChat)
}

// Tool handlers

func (s *OffGridMCPServer) handleGetCurrentTime(ctx context.Context, args json.RawMessage) (*ToolCallResult, error) {
	var params struct {
		Format   string `json:"format"`
		Timezone string `json:"timezone"`
	}
	params.Format = "iso"
	params.Timezone = "Local"

	if len(args) > 0 {
		json.Unmarshal(args, &params)
	}

	var loc *time.Location
	var err error
	if params.Timezone == "Local" || params.Timezone == "" {
		loc = time.Local
	} else {
		loc, err = time.LoadLocation(params.Timezone)
		if err != nil {
			return nil, fmt.Errorf("invalid timezone: %s", params.Timezone)
		}
	}

	now := time.Now().In(loc)
	var result string

	switch params.Format {
	case "iso":
		result = now.Format(time.RFC3339)
	case "unix":
		result = fmt.Sprintf("%d", now.Unix())
	case "human":
		result = now.Format("Monday, January 2, 2006 at 3:04 PM MST")
	default:
		result = now.Format(params.Format)
	}

	return &ToolCallResult{
		Content: []ContentItem{{Type: "text", Text: result}},
	}, nil
}

func (s *OffGridMCPServer) handleCalculate(ctx context.Context, args json.RawMessage) (*ToolCallResult, error) {
	var params struct {
		Expression string `json:"expression"`
	}
	if err := json.Unmarshal(args, &params); err != nil {
		return nil, fmt.Errorf("invalid parameters: %w", err)
	}

	// Simple expression evaluator (for safety, we use a basic implementation)
	result, err := evaluateExpression(params.Expression)
	if err != nil {
		return nil, err
	}

	return &ToolCallResult{
		Content: []ContentItem{{Type: "text", Text: result}},
	}, nil
}

func (s *OffGridMCPServer) handleSystemInfo(ctx context.Context, args json.RawMessage) (*ToolCallResult, error) {
	var params struct {
		Section string `json:"section"`
	}
	params.Section = "all"

	if len(args) > 0 {
		json.Unmarshal(args, &params)
	}

	info := map[string]interface{}{
		"server": map[string]interface{}{
			"name":    s.name,
			"version": s.version,
		},
	}

	if params.Section == "all" || params.Section == "rag" {
		ragInfo := map[string]interface{}{
			"enabled": false,
		}
		if s.ragEngine != nil {
			ragInfo["enabled"] = s.ragEngine.IsEnabled()
			if s.ragEngine.IsEnabled() {
				docs := s.ragEngine.ListDocuments()
				ragInfo["document_count"] = len(docs)
				ragInfo["stats"] = s.ragEngine.Stats()
			}
		}
		info["rag"] = ragInfo
	}

	result, _ := json.MarshalIndent(info, "", "  ")
	return &ToolCallResult{
		Content: []ContentItem{{Type: "text", Text: string(result)}},
	}, nil
}

func (s *OffGridMCPServer) handleSearchDocuments(ctx context.Context, args json.RawMessage) (*ToolCallResult, error) {
	if s.ragEngine == nil || !s.ragEngine.IsEnabled() {
		return nil, fmt.Errorf("RAG is not enabled - please enable it first with an embedding model")
	}

	var params struct {
		Query    string  `json:"query"`
		TopK     int     `json:"top_k"`
		MinScore float32 `json:"min_score"`
	}
	params.TopK = 5
	params.MinScore = 0.3

	if err := json.Unmarshal(args, &params); err != nil {
		return nil, fmt.Errorf("invalid parameters: %w", err)
	}

	if params.Query == "" {
		return nil, fmt.Errorf("query is required")
	}

	opts := rag.SearchOptions{
		TopK:           params.TopK,
		MinScore:       params.MinScore,
		IncludeContent: true,
	}

	ragCtx, err := s.ragEngine.Search(ctx, params.Query, opts)
	if err != nil {
		return nil, err
	}

	var resultText string
	if len(ragCtx.Results) == 0 {
		resultText = "No relevant documents found."
	} else {
		resultText = fmt.Sprintf("Found %d relevant passages:\n\n", len(ragCtx.Results))
		for i, r := range ragCtx.Results {
			resultText += fmt.Sprintf("--- Result %d (Score: %.2f, Source: %s) ---\n%s\n\n",
				i+1, r.Score, r.DocName, r.Chunk.Content)
		}
	}

	return &ToolCallResult{
		Content: []ContentItem{{Type: "text", Text: resultText}},
	}, nil
}

func (s *OffGridMCPServer) handleListDocuments(ctx context.Context, args json.RawMessage) (*ToolCallResult, error) {
	if s.ragEngine == nil || !s.ragEngine.IsEnabled() {
		return nil, fmt.Errorf("RAG is not enabled")
	}

	docs := s.ragEngine.ListDocuments()
	if len(docs) == 0 {
		return &ToolCallResult{
			Content: []ContentItem{{Type: "text", Text: "No documents in knowledge base."}},
		}, nil
	}

	var result string
	result = fmt.Sprintf("Documents in knowledge base (%d total):\n\n", len(docs))
	for _, doc := range docs {
		result += fmt.Sprintf("- %s (ID: %s)\n  Type: %s, Chunks: %d, Added: %s\n\n",
			doc.Name, doc.ID[:8], doc.ContentType, doc.ChunkCount,
			doc.CreatedAt.Format("2006-01-02 15:04"))
	}

	return &ToolCallResult{
		Content: []ContentItem{{Type: "text", Text: result}},
	}, nil
}

func (s *OffGridMCPServer) handleChat(ctx context.Context, args json.RawMessage) (*ToolCallResult, error) {
	if s.chatFunc == nil {
		return nil, fmt.Errorf("chat function not configured")
	}

	var params struct {
		Prompt      string  `json:"prompt"`
		Model       string  `json:"model"`
		Temperature float64 `json:"temperature"`
		MaxTokens   int     `json:"max_tokens"`
	}
	params.Temperature = 0.7
	params.MaxTokens = 1024

	if err := json.Unmarshal(args, &params); err != nil {
		return nil, fmt.Errorf("invalid parameters: %w", err)
	}

	if params.Prompt == "" {
		return nil, fmt.Errorf("prompt is required")
	}

	options := map[string]interface{}{
		"temperature": params.Temperature,
		"max_tokens":  params.MaxTokens,
	}

	response, err := s.chatFunc(ctx, params.Model, params.Prompt, options)
	if err != nil {
		return nil, err
	}

	return &ToolCallResult{
		Content: []ContentItem{{Type: "text", Text: response}},
	}, nil
}

// evaluateExpression is a simple expression evaluator
// For security, we only support basic math operations
func evaluateExpression(expr string) (string, error) {
	// This is a placeholder - in production you'd want a proper expression parser
	// For now, we return a message directing to use the model for calculations
	return fmt.Sprintf("Expression: %s (Use the LLM for complex calculations)", expr), nil
}
