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
}

// ChatMessage represents a single message in a chat
type ChatMessage struct {
	Role    string `json:"role"` // "system", "user", "assistant"
	Content string `json:"content"`
	Name    string `json:"name,omitempty"`
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
	FinishReason string       `json:"finish_reason"`   // "stop", "length", "content_filter"
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
	ID           string    `json:"id"`
	Name         string    `json:"name"`
	Path         string    `json:"path"`
	Size         int64     `json:"size"`         // Size in bytes
	Format       string    `json:"format"`       // "gguf", "ggml", etc.
	Quantization string    `json:"quantization"` // "Q4_0", "Q5_K_M", etc.
	ContextSize  int       `json:"context_size"` // Max context window
	Parameters   string    `json:"parameters"`   // "7B", "13B", etc.
	LoadedAt     time.Time `json:"loaded_at,omitempty"`
	IsLoaded     bool      `json:"is_loaded"`
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
