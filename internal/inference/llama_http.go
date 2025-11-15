//go:build !llama
// +build !llama

package inference

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/takuphilchan/offgrid-llm/pkg/api"
)

// LlamaHTTPEngine proxies requests to llama.cpp HTTP server
// This is the modern, recommended approach - no CGO complexity!
type LlamaHTTPEngine struct {
	baseURL    string
	httpClient *http.Client
	loaded     bool
	modelPath  string
}

// NewLlamaHTTPEngine creates an engine that proxies to llama.cpp server
func NewLlamaHTTPEngine(llamaServerURL string) *LlamaHTTPEngine {
	if llamaServerURL == "" {
		llamaServerURL = "http://localhost:42382"
	}

	return &LlamaHTTPEngine{
		baseURL: llamaServerURL,
		httpClient: &http.Client{
			Timeout: 120 * time.Second,
			Transport: &http.Transport{
				Proxy: func(req *http.Request) (*url.URL, error) {
					return nil, nil // Explicitly bypass all proxies for localhost
				},
			},
		},
		loaded: true, // llama-server handles model loading
	}
}

// Load is a no-op since llama-server manages the model
func (e *LlamaHTTPEngine) Load(ctx context.Context, modelPath string, opts LoadOptions) error {
	e.modelPath = modelPath

	// Check if llama-server is healthy
	req, err := http.NewRequestWithContext(ctx, "GET", e.baseURL+"/health", nil)
	if err != nil {
		return fmt.Errorf("failed to create health check request: %w", err)
	}

	resp, err := e.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("llama-server not responding at %s: %w (make sure llama-server is running)", e.baseURL, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("llama-server health check failed with status %d", resp.StatusCode)
	}

	e.loaded = true
	return nil
}

// Unload is a no-op
func (e *LlamaHTTPEngine) Unload() error {
	e.loaded = false
	return nil
}

// ChatCompletion performs a chat completion via llama-server
func (e *LlamaHTTPEngine) ChatCompletion(ctx context.Context, req *api.ChatCompletionRequest) (*api.ChatCompletionResponse, error) {
	if !e.loaded {
		return nil, fmt.Errorf("no model loaded")
	}

	// Forward request to llama-server
	reqBody, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", e.baseURL+"/v1/chat/completions", bytes.NewReader(reqBody))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := e.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("request to llama-server failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("llama-server returned status %d: %s", resp.StatusCode, string(body))
	}

	var chatResp api.ChatCompletionResponse
	if err := json.NewDecoder(resp.Body).Decode(&chatResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &chatResp, nil
}

// ChatCompletionStream performs a streaming chat completion
func (e *LlamaHTTPEngine) ChatCompletionStream(ctx context.Context, req *api.ChatCompletionRequest, callback TokenCallback) error {
	if !e.loaded {
		return fmt.Errorf("no model loaded")
	}

	// Enable streaming in request
	req.Stream = true

	reqBody, err := json.Marshal(req)
	if err != nil {
		return fmt.Errorf("failed to marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", e.baseURL+"/v1/chat/completions", bytes.NewReader(reqBody))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "text/event-stream")

	resp, err := e.httpClient.Do(httpReq)
	if err != nil {
		return fmt.Errorf("request to llama-server failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("llama-server returned status %d: %s", resp.StatusCode, string(body))
	}

	// Read SSE stream
	scanner := bufio.NewScanner(resp.Body)
	for scanner.Scan() {
		line := scanner.Text()

		if !strings.HasPrefix(line, "data: ") {
			continue
		}

		data := strings.TrimPrefix(line, "data: ")
		if data == "[DONE]" {
			break
		}

		var chunk struct {
			Choices []struct {
				Delta struct {
					Content string `json:"content"`
				} `json:"delta"`
			} `json:"choices"`
		}

		if err := json.Unmarshal([]byte(data), &chunk); err != nil {
			continue // Skip malformed chunks
		}

		if len(chunk.Choices) > 0 && chunk.Choices[0].Delta.Content != "" {
			if err := callback(chunk.Choices[0].Delta.Content); err != nil {
				return err
			}
		}
	}

	return scanner.Err()
}

// Completion performs a text completion
func (e *LlamaHTTPEngine) Completion(ctx context.Context, req *api.CompletionRequest) (*api.CompletionResponse, error) {
	if !e.loaded {
		return nil, fmt.Errorf("no model loaded")
	}

	reqBody, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", e.baseURL+"/v1/completions", bytes.NewReader(reqBody))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := e.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("request to llama-server failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("llama-server returned status %d: %s", resp.StatusCode, string(body))
	}

	var completionResp api.CompletionResponse
	if err := json.NewDecoder(resp.Body).Decode(&completionResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &completionResp, nil
}

// IsLoaded returns whether the engine is connected to llama-server
func (e *LlamaHTTPEngine) IsLoaded() bool {
	return e.loaded
}

// GetModelInfo returns information about the model from llama-server
func (e *LlamaHTTPEngine) GetModelInfo() (*ModelInfo, error) {
	// llama-server doesn't expose detailed model info via API
	// Return basic info
	return &ModelInfo{
		VocabSize:   32000, // Typical for Llama models
		ContextSize: 2048,  // Default
		EmbedSize:   4096,
		NumLayers:   32,
		NumHeads:    32,
	}, nil
}
