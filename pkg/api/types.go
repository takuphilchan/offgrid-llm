package api

import "time"

// ChatCompletionRequest represents an OpenAI-compatible chat completion request
type ChatCompletionRequest struct {
	Model            string        `json:"model"`
	Messages         []ChatMessage `json:"messages"`
	Temperature      *float32      `json:"temperature,omitempty"`
	TopP             *float32      `json:"top_p,omitempty"`
	N                *int          `json:"n,omitempty"`
	Stream           bool          `json:"stream,omitempty"`
	Stop             []string      `json:"stop,omitempty"`
	MaxTokens        *int          `json:"max_tokens,omitempty"`
	PresencePenalty  *float32      `json:"presence_penalty,omitempty"`
	FrequencyPenalty *float32      `json:"frequency_penalty,omitempty"`
	User             string        `json:"user,omitempty"`
	// Function calling
	Tools      []Tool      `json:"tools,omitempty"`
	ToolChoice interface{} `json:"tool_choice,omitempty"` // "none", "auto", or {"type": "function", "function": {"name": "..."}}
	// RAG / Knowledge Base
	UseKnowledgeBase *bool `json:"use_knowledge_base,omitempty"` // Enable RAG context injection
}

// Tool represents a tool that can be called by the model
type Tool struct {
	Type     string      `json:"type"` // "function"
	Function FunctionDef `json:"function"`
}

// FunctionDef represents a function definition
type FunctionDef struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description,omitempty"`
	Parameters  map[string]interface{} `json:"parameters,omitempty"` // JSON Schema
}

// ToolCall represents a tool call made by the model
type ToolCall struct {
	ID       string       `json:"id"`
	Type     string       `json:"type"` // "function"
	Function FunctionCall `json:"function"`
}

// FunctionCall represents the function being called
type FunctionCall struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"` // JSON string of arguments
}

// ChatMessage represents a single message in a chat
type ChatMessage struct {
	Role       string      `json:"role"` // "system", "user", "assistant", "tool"
	Content    interface{} `json:"content"`
	Name       string      `json:"name,omitempty"`
	ToolCalls  []ToolCall  `json:"tool_calls,omitempty"`   // For assistant messages with function calls
	ToolCallID string      `json:"tool_call_id,omitempty"` // For tool response messages
}

// StringContent returns the string representation of the content
func (m ChatMessage) StringContent() string {
	if m.Content == nil {
		return ""
	}
	if str, ok := m.Content.(string); ok {
		return str
	}
	// Handle array of content parts (for VLM)
	if parts, ok := m.Content.([]interface{}); ok {
		var text string
		for _, part := range parts {
			if p, ok := part.(map[string]interface{}); ok {
				if t, ok := p["type"].(string); ok && t == "text" {
					if val, ok := p["text"].(string); ok {
						text += val
					}
				}
			}
		}
		return text
	}
	return ""
}

// ChatCompletionResponse represents an OpenAI-compatible chat completion response
type ChatCompletionResponse struct {
	ID      string                 `json:"id"`
	Object  string                 `json:"object"` // "chat.completion"
	Created int64                  `json:"created"`
	Model   string                 `json:"model"`
	Choices []ChatCompletionChoice `json:"choices"`
	Usage   Usage                  `json:"usage"`
}

// ChatCompletionChoice represents a single completion choice
type ChatCompletionChoice struct {
	Index        int          `json:"index"`
	Message      ChatMessage  `json:"message"`
	FinishReason string       `json:"finish_reason"`   // "stop", "length", "content_filter", "tool_calls"
	Delta        *ChatMessage `json:"delta,omitempty"` // For streaming responses
}

// ChatCompletionChunk represents a streaming chunk response
type ChatCompletionChunk struct {
	ID      string                      `json:"id"`
	Object  string                      `json:"object"` // "chat.completion.chunk"
	Created int64                       `json:"created"`
	Model   string                      `json:"model"`
	Choices []ChatCompletionChoiceChunk `json:"choices"`
}

// ChatCompletionChoiceChunk represents a chunk in streaming mode
type ChatCompletionChoiceChunk struct {
	Index        int         `json:"index"`
	Delta        ChatMessage `json:"delta"`
	FinishReason *string     `json:"finish_reason"`
}

// CompletionRequest represents an OpenAI-compatible completion request
type CompletionRequest struct {
	Model            string   `json:"model"`
	Prompt           string   `json:"prompt"`
	Temperature      *float32 `json:"temperature,omitempty"`
	TopP             *float32 `json:"top_p,omitempty"`
	N                *int     `json:"n,omitempty"`
	Stream           bool     `json:"stream,omitempty"`
	Stop             []string `json:"stop,omitempty"`
	MaxTokens        *int     `json:"max_tokens,omitempty"`
	PresencePenalty  *float32 `json:"presence_penalty,omitempty"`
	FrequencyPenalty *float32 `json:"frequency_penalty,omitempty"`
	User             string   `json:"user,omitempty"`
}

// CompletionResponse represents an OpenAI-compatible completion response
type CompletionResponse struct {
	ID      string             `json:"id"`
	Object  string             `json:"object"` // "text_completion"
	Created int64              `json:"created"`
	Model   string             `json:"model"`
	Choices []CompletionChoice `json:"choices"`
	Usage   Usage              `json:"usage"`
}

// CompletionChoice represents a single completion choice
type CompletionChoice struct {
	Index        int    `json:"index"`
	Text         string `json:"text"`
	FinishReason string `json:"finish_reason"`
}

// Usage represents token usage information
type Usage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

// Model represents a model in the registry
type Model struct {
	ID         string   `json:"id"`
	Object     string   `json:"object"` // "model"
	Created    int64    `json:"created"`
	OwnedBy    string   `json:"owned_by"`
	Permission []string `json:"permission,omitempty"`
	Root       string   `json:"root,omitempty"`
	Parent     string   `json:"parent,omitempty"`
	Type       string   `json:"type,omitempty"`    // "llm" or "embedding"
	Size       int64    `json:"size,omitempty"`    // Size in bytes
	SizeGB     string   `json:"size_gb,omitempty"` // Human-readable size
}

// ModelListResponse represents the response for listing models
type ModelListResponse struct {
	Object string  `json:"object"` // "list"
	Data   []Model `json:"data"`
}

// ErrorResponse represents an error response
type ErrorResponse struct {
	Error ErrorDetail `json:"error"`
}

// ErrorDetail contains error details
type ErrorDetail struct {
	Message string `json:"message"`
	Type    string `json:"type"`
	Code    string `json:"code,omitempty"`
}

// ModelMetadata contains additional metadata about a model
type ModelMetadata struct {
	ID            string    `json:"id"`
	Name          string    `json:"name"`
	Path          string    `json:"path"`
	Size          int64     `json:"size"`                     // Size in bytes
	Format        string    `json:"format"`                   // "gguf", "ggml", etc.
	Quantization  string    `json:"quantization"`             // "Q4_0", "Q5_K_M", etc.
	ContextSize   int       `json:"context_size"`             // Max context window
	Parameters    string    `json:"parameters"`               // "7B", "13B", etc.
	Type          string    `json:"type"`                     // "llm" or "embedding"
	ProjectorPath string    `json:"projector_path,omitempty"` // Path to vision projector file
	LoadedAt      time.Time `json:"loaded_at,omitempty"`
	IsLoaded      bool      `json:"is_loaded"`
}

// EmbeddingRequest represents an OpenAI-compatible embedding request
type EmbeddingRequest struct {
	Model          string      `json:"model"`                     // Model ID to use for embeddings
	Input          interface{} `json:"input"`                     // String or array of strings
	EncodingFormat string      `json:"encoding_format,omitempty"` // "float" (default) or "base64"
	User           string      `json:"user,omitempty"`            // Optional user identifier
	Dimensions     *int        `json:"dimensions,omitempty"`      // Number of dimensions (for models that support it)
}

// EmbeddingResponse represents an OpenAI-compatible embedding response
type EmbeddingResponse struct {
	Object string          `json:"object"` // "list"
	Data   []EmbeddingData `json:"data"`
	Model  string          `json:"model"`
	Usage  EmbeddingUsage  `json:"usage"`
}

// EmbeddingData represents a single embedding vector
type EmbeddingData struct {
	Object    string    `json:"object"` // "embedding"
	Embedding []float32 `json:"embedding"`
	Index     int       `json:"index"`
}

// EmbeddingUsage represents token usage for embeddings
type EmbeddingUsage struct {
	PromptTokens int `json:"prompt_tokens"`
	TotalTokens  int `json:"total_tokens"`
}
