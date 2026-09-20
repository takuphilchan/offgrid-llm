//go:build !llama

package inference

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

func (e *LlamaEngine) ToolRuntimeInfo(ctx context.Context) (*ToolRuntimeInfo, error) {
	return e.httpEngine.ToolRuntimeInfo(ctx)
}
func (e *LlamaHTTPEngine) ToolRuntimeInfo(ctx context.Context) (*ToolRuntimeInfo, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, "GET", e.baseURL+"/props", nil)
	if err != nil {
		return nil, err
	}
	resp, err := e.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("runtime metadata unavailable")
	}
	var props struct {
		Build    string `json:"build_info"`
		Template string `json:"chat_template"`
		Settings struct {
			Context int `json:"n_ctx"`
		} `json:"default_generation_settings"`
		Caps struct {
			Tools *bool `json:"supports_tools"`
			Calls *bool `json:"supports_tool_calls"`
		} `json:"chat_template_caps"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&props); err != nil {
		return nil, err
	}
	digest := sha256.Sum256([]byte(props.Template))
	return &ToolRuntimeInfo{Build: props.Build, TemplateSHA256: hex.EncodeToString(digest[:]), Context: props.Settings.Context, SupportsTools: props.Caps.Tools, SupportsToolCalls: props.Caps.Calls}, nil
}
