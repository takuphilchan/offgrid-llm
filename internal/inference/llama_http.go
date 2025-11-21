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
			Timeout: 5 * time.Minute, // Increased for slow model loading on low-end systems
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

	// Don't check health here - let the actual chat request handle retries
	// The model cache already waits for server to start, and chat requests
	// have retry logic for 503 errors during model loading
	e.loaded = true
	return nil
}

// SetPort updates the llama-server port for this engine
func (e *LlamaHTTPEngine) SetPort(port int) {
	e.baseURL = fmt.Sprintf("http://localhost:%d", port)
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
	// Ensure we don't pass the model name to llama-server, as it might confuse it
	// llama-server only serves one model at a time
	reqCopy := *req
	reqCopy.Model = "" // Clear model name for the backend request

	// Add default stop tokens if not provided
	if len(reqCopy.Stop) == 0 {
		modelName := strings.ToLower(req.Model)
		if strings.Contains(modelName, "llama-3") || strings.Contains(modelName, "llama3") {
			reqCopy.Stop = []string{"<|eot_id|>", "<|end_of_text|>"}
		} else if strings.Contains(modelName, "phi-3") {
			reqCopy.Stop = []string{"<|end|>", "<|endoftext|>"}
		} else {
			// Default ChatML stop tokens
			reqCopy.Stop = []string{"<|im_end|>"}
		}
	}

	// Log the backend URL for debugging model switching
	fmt.Printf("Sending request to backend: %s (Model: %s)\n", e.baseURL, req.Model)

	reqBody, err := json.Marshal(reqCopy)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	// Retry logic for model loading (503 errors)
	maxRetries := 60
	for attempt := 0; attempt <= maxRetries; attempt++ {
		httpReq, err := http.NewRequestWithContext(ctx, "POST", e.baseURL+"/v1/chat/completions", bytes.NewReader(reqBody))
		if err != nil {
			return nil, fmt.Errorf("failed to create request: %w", err)
		}
		httpReq.Header.Set("Content-Type", "application/json")

		resp, err := e.httpClient.Do(httpReq)
		if err != nil {
			return nil, fmt.Errorf("request to llama-server failed: %w", err)
		}

		// Check for 503 (model still loading)
		if resp.StatusCode == http.StatusServiceUnavailable {
			resp.Body.Close()
			if attempt < maxRetries {
				time.Sleep(1 * time.Second)
				continue
			}
			return nil, fmt.Errorf("model failed to load within 60 seconds")
		}

		if resp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(resp.Body)
			resp.Body.Close()
			return nil, classifyLlamaServerError(resp.StatusCode, body)
		}

		defer resp.Body.Close()
		var chatResp api.ChatCompletionResponse
		if err := json.NewDecoder(resp.Body).Decode(&chatResp); err != nil {
			return nil, fmt.Errorf("failed to decode response: %w", err)
		}

		return &chatResp, nil
	}

	return nil, fmt.Errorf("unexpected: exceeded max retries")
}

// ChatCompletionStream performs a streaming chat completion
func (e *LlamaHTTPEngine) ChatCompletionStream(ctx context.Context, req *api.ChatCompletionRequest, callback TokenCallback) error {
	if !e.loaded {
		return fmt.Errorf("no model loaded")
	}

	// Enable streaming in request
	reqCopy := *req
	reqCopy.Stream = true
	reqCopy.Model = "" // Clear model name for the backend request

	// Add default stop tokens if not provided
	if len(reqCopy.Stop) == 0 {
		modelName := strings.ToLower(req.Model)
		if strings.Contains(modelName, "llama-3") || strings.Contains(modelName, "llama3") {
			reqCopy.Stop = []string{"<|eot_id|>", "<|end_of_text|>"}
		} else if strings.Contains(modelName, "phi-3") {
			reqCopy.Stop = []string{"<|end|>", "<|endoftext|>"}
		} else {
			// Default ChatML stop tokens
			reqCopy.Stop = []string{"<|im_end|>"}
		}
	}

	// Log the backend URL for debugging model switching
	fmt.Printf("Sending stream request to backend: %s (Model: %s)\n", e.baseURL, req.Model)

	reqBody, err := json.Marshal(reqCopy)
	if err != nil {
		return fmt.Errorf("failed to marshal request: %w", err)
	}

	// Retry logic for model loading (503 errors)
	maxRetries := 60 // 60 retries = up to 60 seconds for model loading
	for attempt := 0; attempt <= maxRetries; attempt++ {
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

		// Check for 503 (model still loading)
		if resp.StatusCode == http.StatusServiceUnavailable {
			resp.Body.Close()
			if attempt < maxRetries {
				time.Sleep(1 * time.Second)
				continue // Retry
			}
			return fmt.Errorf("model failed to load within 60 seconds")
		}

		if resp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(resp.Body)
			resp.Body.Close()
			return classifyLlamaServerError(resp.StatusCode, body)
		}

		defer resp.Body.Close()

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

	return fmt.Errorf("unexpected: exceeded max retries")
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
		return nil, classifyLlamaServerError(resp.StatusCode, body)
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

func classifyLlamaServerError(status int, body []byte) error {
	var backendErr struct {
		Error struct {
			Message string `json:"message"`
		} `json:"error"`
	}

	message := strings.TrimSpace(string(body))
	if err := json.Unmarshal(body, &backendErr); err == nil {
		if backendErr.Error.Message != "" {
			message = backendErr.Error.Message
		}
	}

	lower := strings.ToLower(message)
	if strings.Contains(lower, "mmproj") || strings.Contains(lower, "image input is not supported") {
		return &EngineError{
			Code:    ErrCodeMissingMmproj,
			Message: "Image input is not supported because the model's mmproj adapter is missing. Download the matching .mmproj file, place it next to the GGUF, and reload the model.",
			Details: message,
		}
	}

	if message == "" {
		message = string(body)
	}

	return fmt.Errorf("llama-server returned status %d: %s", status, message)
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
