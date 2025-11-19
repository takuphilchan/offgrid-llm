//go:build llama
// +build llama

package inference

import (
	"context"
	"fmt"
	"strings"
	"time"

	llama "github.com/go-skynet/go-llama.cpp"
	"github.com/takuphilchan/offgrid-llm/pkg/api"
)

// LlamaEngine implements the Engine interface using llama.cpp
type LlamaEngine struct {
	model     *llama.LLama
	modelPath string
	opts      LoadOptions
	loaded    bool
}

// NewLlamaEngine creates a new llama.cpp engine
func NewLlamaEngine() *LlamaEngine {
	return &LlamaEngine{
		loaded: false,
	}
}

// Load loads a GGUF model file
func (e *LlamaEngine) Load(ctx context.Context, modelPath string, opts LoadOptions) error {
	if e.loaded {
		if err := e.Unload(); err != nil {
			return fmt.Errorf("failed to unload previous model: %w", err)
		}
	}

	// Convert our options to llama.cpp options
	llamaOpts := []llama.ModelOption{
		llama.SetContext(opts.ContextSize),
		llama.SetGPULayers(opts.NumGPULayers),
		llama.SetThreads(opts.NumThreads),
	}

	if opts.UseMlock {
		llamaOpts = append(llamaOpts, llama.EnableMLock)
	}

	if opts.UseMmap {
		llamaOpts = append(llamaOpts, llama.EnableMemoryMapping)
	}

	// Load the model
	model, err := llama.New(modelPath, llamaOpts...)
	if err != nil {
		return fmt.Errorf("failed to load model: %w", err)
	}

	e.model = model
	e.modelPath = modelPath
	e.opts = opts
	e.loaded = true

	return nil
}

// Unload unloads the current model
func (e *LlamaEngine) Unload() error {
	if !e.loaded {
		return nil
	}

	if e.model != nil {
		e.model.Free()
		e.model = nil
	}

	e.loaded = false
	e.modelPath = ""

	return nil
}

// ChatCompletion performs a chat completion
func (e *LlamaEngine) ChatCompletion(ctx context.Context, req *api.ChatCompletionRequest) (*api.ChatCompletionResponse, error) {
	if !e.loaded {
		return nil, fmt.Errorf("no model loaded")
	}

	// Build prompt from messages
	prompt := e.buildChatPrompt(req.Model, req.Messages)

	// Set up prediction options
	predictOpts := []llama.PredictOption{
		llama.SetTemperature(float64(req.Temperature)),
		llama.SetTopP(float64(req.TopP)),
		llama.SetTopK(req.TopK),
		llama.SetTokens(req.MaxTokens),
	}

	// Add default stop tokens if not provided
	stopTokens := req.Stop
	if len(stopTokens) == 0 {
		if strings.Contains(strings.ToLower(req.Model), "llama-3") || strings.Contains(strings.ToLower(req.Model), "llama3") {
			stopTokens = []string{"<|eot_id|>", "<|end_of_text|>"}
		} else {
			// Default ChatML stop tokens
			stopTokens = []string{"<|im_end|>"}
		}
	}

	if len(stopTokens) > 0 {
		predictOpts = append(predictOpts, llama.SetStopWords(stopTokens...))
	}

	// Generate response
	response, err := e.model.Predict(prompt, predictOpts...)
	if err != nil {
		return nil, fmt.Errorf("prediction failed: %w", err)
	}

	// Estimate token counts (rough approximation)
	promptTokens := len(strings.Fields(prompt))
	completionTokens := len(strings.Fields(response))

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
					Content: strings.TrimSpace(response),
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
func (e *LlamaEngine) ChatCompletionStream(ctx context.Context, req *api.ChatCompletionRequest, callback TokenCallback) error {
	if !e.loaded {
		return fmt.Errorf("no model loaded")
	}

	// Build prompt from messages
	prompt := e.buildChatPrompt(req.Model, req.Messages)

	// Set up prediction options
	predictOpts := []llama.PredictOption{
		llama.SetTemperature(float64(req.Temperature)),
		llama.SetTopP(float64(req.TopP)),
		llama.SetTopK(req.TopK),
		llama.SetTokens(req.MaxTokens),
	}

	// Add default stop tokens if not provided
	stopTokens := req.Stop
	if len(stopTokens) == 0 {
		if strings.Contains(strings.ToLower(req.Model), "llama-3") || strings.Contains(strings.ToLower(req.Model), "llama3") {
			stopTokens = []string{"<|eot_id|>", "<|end_of_text|>"}
		} else {
			// Default ChatML stop tokens
			stopTokens = []string{"<|im_end|>"}
		}
	}

	if len(stopTokens) > 0 {
		predictOpts = append(predictOpts, llama.SetStopWords(stopTokens...))
	}

	// Create a channel for tokens
	tokenChan := make(chan string, 10)
	doneChan := make(chan struct{})
	errorChan := make(chan error, 1)

	// Start prediction in goroutine
	go func() {
		defer close(tokenChan)
		defer close(doneChan)

		_, err := e.model.Predict(prompt, append(predictOpts, llama.SetTokenCallback(func(token string) bool {
			select {
			case tokenChan <- token:
				return true
			case <-ctx.Done():
				return false
			}
		}))...)

		if err != nil {
			errorChan <- err
		}
	}()

	// Stream tokens to callback
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case err := <-errorChan:
			return err
		case token, ok := <-tokenChan:
			if !ok {
				return nil
			}
			if err := callback(token); err != nil {
				return err
			}
		case <-doneChan:
			return nil
		}
	}
}

// Completion performs a text completion
func (e *LlamaEngine) Completion(ctx context.Context, req *api.CompletionRequest) (*api.CompletionResponse, error) {
	if !e.loaded {
		return nil, fmt.Errorf("no model loaded")
	}

	// Set up prediction options
	predictOpts := []llama.PredictOption{
		llama.SetTemperature(float64(req.Temperature)),
		llama.SetTopP(float64(req.TopP)),
		llama.SetTopK(req.TopK),
		llama.SetTokens(req.MaxTokens),
	}

	if len(req.Stop) > 0 {
		predictOpts = append(predictOpts, llama.SetStopWords(req.Stop...))
	}

	// Generate response
	response, err := e.model.Predict(req.Prompt, predictOpts...)
	if err != nil {
		return nil, fmt.Errorf("prediction failed: %w", err)
	}

	// Estimate token counts
	promptTokens := len(strings.Fields(req.Prompt))
	completionTokens := len(strings.Fields(response))

	return &api.CompletionResponse{
		ID:      fmt.Sprintf("cmpl-%d", time.Now().Unix()),
		Object:  "text_completion",
		Created: time.Now().Unix(),
		Model:   req.Model,
		Choices: []api.CompletionChoice{
			{
				Index:        0,
				Text:         strings.TrimSpace(response),
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
func (e *LlamaEngine) IsLoaded() bool {
	return e.loaded
}

// GetModelInfo returns information about the loaded model
func (e *LlamaEngine) GetModelInfo() (*ModelInfo, error) {
	if !e.loaded {
		return nil, fmt.Errorf("no model loaded")
	}

	// llama.cpp doesn't expose all this info easily, so we return basic info
	return &ModelInfo{
		VocabSize:   32000, // Common for Llama models
		ContextSize: e.opts.ContextSize,
		EmbedSize:   4096, // Common for 7B models
		NumLayers:   32,   // Common for 7B models
		NumHeads:    32,   // Common for 7B models
	}, nil
}

// buildChatPrompt constructs a prompt from chat messages
func (e *LlamaEngine) buildChatPrompt(model string, messages []api.ChatMessage) string {
	// Check for Llama 3
	if strings.Contains(strings.ToLower(model), "llama-3") || strings.Contains(strings.ToLower(model), "llama3") {
		return e.buildLlama3Prompt(messages)
	}

	// Default to ChatML
	return e.buildChatMLPrompt(messages)
}

func (e *LlamaEngine) buildLlama3Prompt(messages []api.ChatMessage) string {
	var builder strings.Builder

	builder.WriteString("<|begin_of_text|>")

	for _, msg := range messages {
		builder.WriteString(fmt.Sprintf("<|start_header_id|>%s<|end_header_id|>\n\n", msg.Role))
		builder.WriteString(strings.TrimSpace(msg.Content))
		builder.WriteString("<|eot_id|>")
	}

	builder.WriteString("<|start_header_id|>assistant<|end_header_id|>\n\n")
	return builder.String()
}

func (e *LlamaEngine) buildChatMLPrompt(messages []api.ChatMessage) string {
	var builder strings.Builder

	for _, msg := range messages {
		switch msg.Role {
		case "system":
			builder.WriteString("<|im_start|>system\n")
			builder.WriteString(msg.Content)
			builder.WriteString("<|im_end|>\n")
		case "user":
			builder.WriteString("<|im_start|>user\n")
			builder.WriteString(msg.Content)
			builder.WriteString("<|im_end|>\n")
		case "assistant":
			builder.WriteString("<|im_start|>assistant\n")
			builder.WriteString(msg.Content)
			builder.WriteString("<|im_end|>\n")
		}
	}

	builder.WriteString("<|im_start|>assistant\n")
	return builder.String()
}
