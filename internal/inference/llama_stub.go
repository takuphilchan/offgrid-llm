//go:build !llama
// +build !llama

package inference

import (
	"context"
	"fmt"
	"os"

	"github.com/takuphilchan/offgrid-llm/internal/config"
	"github.com/takuphilchan/offgrid-llm/pkg/api"
)

// LlamaEngine stub - uses HTTP proxy to llama.cpp server instead of CGO
// This is the modern, recommended approach!
type LlamaEngine struct {
	httpEngine *LlamaHTTPEngine
}

// NewLlamaEngine creates an engine that uses llama.cpp HTTP server
func NewLlamaEngine() *LlamaEngine {
	llamaServerURL := os.Getenv("LLAMA_SERVER_URL")
	if llamaServerURL == "" {
		port := config.ReadLlamaPort("42382")
		llamaServerURL = fmt.Sprintf("http://127.0.0.1:%s", port)
	}

	return &LlamaEngine{
		httpEngine: NewLlamaHTTPEngine(llamaServerURL),
	}
}

// Load delegates to HTTP engine
func (e *LlamaEngine) Load(ctx context.Context, modelPath string, opts LoadOptions) error {
	if err := e.httpEngine.Load(ctx, modelPath, opts); err != nil {
		fmt.Println(" Warning: llama-server not reachable.")
		fmt.Println("   To enable real inference:")
		fmt.Println("   1. Check llama-server status:")
		fmt.Println("      sudo systemctl status llama-server")
		fmt.Println("   2. Or set LLAMA_SERVER_URL environment variable")
		fmt.Println("   Note: llama-server runs on internal localhost-only port for security")
		return err
	}
	fmt.Println("[OK] Connected to llama-server - REAL INFERENCE ENABLED")
	return nil
}

// Unload delegates to HTTP engine
func (e *LlamaEngine) Unload() error {
	return e.httpEngine.Unload()
}

// ChatCompletion delegates to HTTP engine
func (e *LlamaEngine) ChatCompletion(ctx context.Context, req *api.ChatCompletionRequest) (*api.ChatCompletionResponse, error) {
	return e.httpEngine.ChatCompletion(ctx, req)
}

// ChatCompletionStream delegates to HTTP engine
func (e *LlamaEngine) ChatCompletionStream(ctx context.Context, req *api.ChatCompletionRequest, callback TokenCallback) error {
	return e.httpEngine.ChatCompletionStream(ctx, req, callback)
}

func (e *LlamaEngine) ChatCompletionStreamRaw(ctx context.Context, req *api.ChatCompletionRequest, callback ChatCompletionStreamCallback) error {
	return e.httpEngine.ChatCompletionStreamRaw(ctx, req, callback)
}

// Completion delegates to HTTP engine
func (e *LlamaEngine) Completion(ctx context.Context, req *api.CompletionRequest) (*api.CompletionResponse, error) {
	return e.httpEngine.Completion(ctx, req)
}

// IsLoaded delegates to HTTP engine
func (e *LlamaEngine) IsLoaded() bool {
	return e.httpEngine.IsLoaded()
}

// GetModelInfo delegates to HTTP engine
func (e *LlamaEngine) GetModelInfo() (*ModelInfo, error) {
	return e.httpEngine.GetModelInfo()
}

// SetPort updates the llama-server port for this engine
func (e *LlamaEngine) SetPort(port int) {
	e.httpEngine.SetPort(port)
}
