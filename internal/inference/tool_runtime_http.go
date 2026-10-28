//go:build !llama

package inference

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/takuphilchan/offgrid-llm/internal/jsonwire"
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
	const metadataLimit = 1 << 20
	data, err := io.ReadAll(io.LimitReader(resp.Body, metadataLimit+1))
	if err != nil {
		return nil, err
	}
	if len(data) > metadataLimit || jsonwire.Validate(data) != nil || !bytes.HasPrefix(bytes.TrimSpace(data), []byte("{")) {
		return nil, fmt.Errorf("runtime metadata is incomplete or ambiguous")
	}
	if err := json.Unmarshal(data, &props); err != nil {
		return nil, fmt.Errorf("runtime metadata has an invalid shape")
	}
	info := &ToolRuntimeInfo{Build: props.Build, Context: props.Settings.Context, SupportsTools: props.Caps.Tools, SupportsToolCalls: props.Caps.Calls}
	// Missing identity must remain missing. SHA256("") is a valid digest, but
	// does not identify a measured template. Older runtimes may still report
	// context for conversation without supplying a native tool template.
	if strings.TrimSpace(props.Template) != "" {
		digest := sha256.Sum256([]byte(props.Template))
		info.TemplateSHA256 = hex.EncodeToString(digest[:])
	}
	if props.Build == "b1-b95502b" || props.Build == "b1-b95502b-offgrid-seq1" || props.Build == "b1-b95502b-offgrid-seq2" {
		info.TemplateAdapter = toolTemplateAdapterIdentity(info.TemplateSHA256)
		if info.TemplateAdapter == resultToolsAdapter && props.Build != "b1-b95502b-offgrid-seq2" {
			info.TemplateAdapter = ""
		}
	}
	return info, nil
}
