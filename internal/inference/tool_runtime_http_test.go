//go:build !llama

package inference

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
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

func TestToolRuntimeMetadataRejectsAmbiguousIdentity(t *testing.T) {
	for _, raw := range []string{
		`null`,
		`[]`,
		`{"chat_template":"a","chat_template":"b"}`,
		`{"default_generation_settings":{"n_ctx":8192,"n_ctx":4096}}`,
		`{"chat_template":"a"} {"chat_template":"b"}`,
		`{"chat_template":"\uD800"}`,
		`{"chat_template":"partial`,
		`{"chat_template":true}`,
		`{"chat_template":"` + strings.Repeat("a", 1<<20) + `"}`,
	} {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { fmt.Fprint(w, raw) }))
		info, err := NewLlamaHTTPEngine(server.URL).ToolRuntimeInfo(context.Background())
		server.Close()
		if err == nil || info != nil {
			t.Fatal("ambiguous runtime identity accepted")
		}
	}
}

func TestToolRuntimeMissingTemplateDoesNotBecomeMeasuredDigest(t *testing.T) {
	for _, raw := range []string{
		`{"default_generation_settings":{"n_ctx":8192}}`,
		`{"chat_template":""}`,
		`{"chat_template":" \n\t"}`,
	} {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { fmt.Fprint(w, raw) }))
		info, err := NewLlamaHTTPEngine(server.URL).ToolRuntimeInfo(context.Background())
		server.Close()
		if err != nil || info == nil || info.TemplateSHA256 != "" || info.TemplateAdapter != "" {
			t.Fatal("missing template manufactured identity", info, err)
		}
	}
}

func TestToolRuntimeIdentityIsRefreshedAfterTemplateChange(t *testing.T) {
	var revision atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, `{"chat_template":"fixture revision %d","build_info":"b1-b95502b"}`, revision.Load())
	}))
	defer server.Close()
	engine := NewLlamaHTTPEngine(server.URL)
	before, err := engine.ToolRuntimeInfo(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	revision.Store(1)
	after, err := engine.ToolRuntimeInfo(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if before.TemplateSHA256 == after.TemplateSHA256 || before.TemplateAdapter != "" || after.TemplateAdapter != "" {
		t.Fatal("reused a stale template identity or inferred an adapter from runtime alone")
	}
}
