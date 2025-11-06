//go:build !llama
// +build !llama

package inference

import (
	"context"
	"fmt"

	"github.com/takuphilchan/offgrid-llm/pkg/api"
)

// LlamaEngine stub when llama.cpp is not available
// To enable real llama.cpp support:
// 1. Install llama.cpp: git clone https://github.com/ggerganov/llama.cpp && cd llama.cpp && make
// 2. Set environment: export C_INCLUDE_PATH=/path/to/llama.cpp:$C_INCLUDE_PATH
// 3. Build with: go build -tags llama
type LlamaEngine struct {
	MockEngine
}

// NewLlamaEngine creates a stub that delegates to MockEngine
func NewLlamaEngine() *LlamaEngine {
	return &LlamaEngine{
		MockEngine: *NewMockEngine(),
	}
}

// Load shows a warning and delegates to mock
func (e *LlamaEngine) Load(ctx context.Context, modelPath string, opts LoadOptions) error {
	fmt.Println("⚠️  Warning: llama.cpp not compiled in. Using mock responses.")
	fmt.Println("   To enable real inference:")
	fmt.Println("   1. Install llama.cpp dependencies")
	fmt.Println("   2. Build with: go build -tags llama")
	return e.MockEngine.Load(ctx, modelPath, opts)
}

// All other methods delegate to MockEngine
func (e *LlamaEngine) ChatCompletion(ctx context.Context, req *api.ChatCompletionRequest) (*api.ChatCompletionResponse, error) {
	return e.MockEngine.ChatCompletion(ctx, req)
}

func (e *LlamaEngine) ChatCompletionStream(ctx context.Context, req *api.ChatCompletionRequest, callback TokenCallback) error {
	return e.MockEngine.ChatCompletionStream(ctx, req, callback)
}

func (e *LlamaEngine) Completion(ctx context.Context, req *api.CompletionRequest) (*api.CompletionResponse, error) {
	return e.MockEngine.Completion(ctx, req)
}
