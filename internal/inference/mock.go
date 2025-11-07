package inference

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/takuphilchan/offgrid-llm/pkg/api"
)

// MockEngine is a simple mock implementation for testing
// This will be replaced with actual llama.cpp integration
type MockEngine struct {
	loaded    bool
	modelPath string
}

// NewMockEngine creates a new mock engine
func NewMockEngine() *MockEngine {
	return &MockEngine{}
}

// Load loads a model
func (e *MockEngine) Load(ctx context.Context, modelPath string, opts LoadOptions) error {
	e.modelPath = modelPath
	e.loaded = true
	return nil
}

// Unload unloads the model
func (e *MockEngine) Unload() error {
	e.loaded = false
	e.modelPath = ""
	return nil
}

// ChatCompletion performs a chat completion
func (e *MockEngine) ChatCompletion(ctx context.Context, req *api.ChatCompletionRequest) (*api.ChatCompletionResponse, error) {
	if !e.loaded {
		return nil, fmt.Errorf("no model loaded")
	}

	// Build conversation context
	var promptBuilder strings.Builder
	for _, msg := range req.Messages {
		promptBuilder.WriteString(fmt.Sprintf("%s: %s\n", msg.Role, msg.Content))
	}

	// Mock response
	responseText := "🔧 MOCK MODE: Real inference temporarily disabled due to llama.cpp version compatibility. See docs/INFERENCE_TODO.md for details. All API endpoints are functional and ready for integration."

	// Count tokens (very rough approximation)
	promptTokens := len(strings.Fields(promptBuilder.String()))
	completionTokens := len(strings.Fields(responseText))

	return &api.ChatCompletionResponse{
		ID:      fmt.Sprintf("chatcmpl-%d", time.Now().Unix()),
		Object:  "chat.completion",
		Created: time.Now().Unix(),
		Model:   req.Model,
		Choices: []api.ChatCompletionChoice{
			{
				Index: 0,
				Message: api.ChatMessage{
					Role:    "assistant",
					Content: responseText,
				},
				FinishReason: "stop",
			},
		},
		Usage: api.Usage{
			PromptTokens:     promptTokens,
			CompletionTokens: completionTokens,
			TotalTokens:      promptTokens + completionTokens,
		},
	}, nil
}

// ChatCompletionStream performs a streaming chat completion
func (e *MockEngine) ChatCompletionStream(ctx context.Context, req *api.ChatCompletionRequest, callback TokenCallback) error {
	if !e.loaded {
		return fmt.Errorf("no model loaded")
	}

	// Mock streaming response - send token by token
	responseText := "🔧 MOCK MODE: Real LLM inference is temporarily disabled while we resolve llama.cpp version compatibility. The API and all infrastructure are fully functional and ready. See docs/INFERENCE_TODO.md for the integration roadmap."
	words := strings.Fields(responseText)

	for _, word := range words {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			// Simulate processing time
			time.Sleep(50 * time.Millisecond)

			if err := callback(word + " "); err != nil {
				return err
			}
		}
	}

	return nil
}

// Completion performs a text completion
func (e *MockEngine) Completion(ctx context.Context, req *api.CompletionRequest) (*api.CompletionResponse, error) {
	if !e.loaded {
		return nil, fmt.Errorf("no model loaded")
	}

	// Mock response
	responseText := "🔧 MOCK MODE: Real inference coming soon. API infrastructure is ready."

	promptTokens := len(strings.Fields(req.Prompt))
	completionTokens := len(strings.Fields(responseText))

	return &api.CompletionResponse{
		ID:      fmt.Sprintf("cmpl-%d", time.Now().Unix()),
		Object:  "text_completion",
		Created: time.Now().Unix(),
		Model:   req.Model,
		Choices: []api.CompletionChoice{
			{
				Index:        0,
				Text:         responseText,
				FinishReason: "stop",
			},
		},
		Usage: api.Usage{
			PromptTokens:     promptTokens,
			CompletionTokens: completionTokens,
			TotalTokens:      promptTokens + completionTokens,
		},
	}, nil
}

// IsLoaded returns whether a model is loaded
func (e *MockEngine) IsLoaded() bool {
	return e.loaded
}

// GetModelInfo returns model information
func (e *MockEngine) GetModelInfo() (*ModelInfo, error) {
	if !e.loaded {
		return nil, fmt.Errorf("no model loaded")
	}

	return &ModelInfo{
		VocabSize:   32000,
		ContextSize: 4096,
		EmbedSize:   4096,
		NumLayers:   32,
		NumHeads:    32,
	}, nil
}
