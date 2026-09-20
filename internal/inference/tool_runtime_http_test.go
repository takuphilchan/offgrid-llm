//go:build !llama

package inference

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestToolRuntimeMetadataDoesNotExposeTemplateOrPaths(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/props" {
			t.Errorf("wrong route %s", r.URL.Path)
		}
		fmt.Fprint(w, `{"chat_template":"private template","model_path":"private/model","build_info":"fixture-runtime","default_generation_settings":{"n_ctx":8192},"chat_template_caps":{"supports_tools":false,"supports_tool_calls":false}}`)
	}))
	defer server.Close()
	info, err := NewLlamaHTTPEngine(server.URL).ToolRuntimeInfo(context.Background())
	if err != nil || info.Build != "fixture-runtime" || info.Context != 8192 || len(info.TemplateSHA256) != 64 || info.SupportsTools == nil || *info.SupportsTools {
		t.Fatalf("metadata: %+v %v", info, err)
	}
}
