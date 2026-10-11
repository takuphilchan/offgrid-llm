//go:build !llama

package inference

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/takuphilchan/offgrid-llm/pkg/api"
)

func (e *LlamaEngine) CountPromptTokens(ctx context.Context, request *api.ChatCompletionRequest) (int, error) {
	return e.httpEngine.CountPromptTokens(ctx, request)
}

func (e *LlamaHTTPEngine) CountPromptTokens(ctx context.Context, request *api.ChatCompletionRequest) (int, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	copy := *request
	copy.Model, copy.Stream, copy.StreamOptions = "", false, nil
	body, err := json.Marshal(copy)
	if err != nil {
		return 0, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, e.baseURL+"/v1/chat/completions/input_tokens", bytes.NewReader(body))
	if err != nil {
		return 0, err
	}
	req.Header.Set("Content-Type", "application/json")
	response, err := e.httpClient.Do(req)
	if err != nil {
		return 0, err
	}
	defer response.Body.Close()
	if response.StatusCode == http.StatusNotFound || response.StatusCode == http.StatusNotImplemented {
		return 0, ErrPromptCountUnavailable
	}
	if response.StatusCode != http.StatusOK {
		// Runtime error bodies can contain private prompts. Do not relay or log.
		return 0, fmt.Errorf("runtime prompt count failed (HTTP %d)", response.StatusCode)
	}
	data, err := io.ReadAll(io.LimitReader(response.Body, 4097))
	if err != nil || len(data) > 4096 {
		return 0, fmt.Errorf("invalid runtime prompt count response")
	}
	var count struct {
		Tokens int `json:"input_tokens"`
	}
	if json.Unmarshal(data, &count) != nil || count.Tokens <= 0 || count.Tokens > 100000000 {
		return 0, fmt.Errorf("invalid runtime prompt count response")
	}
	return count.Tokens, nil
}
