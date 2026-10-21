package cli

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/takuphilchan/offgrid-llm/internal/config"
)

func TestBenchmarkUsesInjectedTerminalAndDisposableModels(t *testing.T) {
	var out, errOut bytes.Buffer
	env := testEnvironment(&out, &errOut, &fakeRunner{})
	env.LocalAdmin.LoadConfig = func() *config.Config {
		return &config.Config{ModelsDir: t.TempDir(), ServerPort: 12999}
	}
	for _, tc := range []struct {
		args []string
		want string
	}{
		{[]string{"benchmark"}, "Benchmark Model"},
		{[]string{"bench", "missing"}, "Model not found"},
		{[]string{"compare"}, "Benchmark Compare"},
	} {
		out.Reset()
		errOut.Reset()
		if code := Execute(context.Background(), tc.args, env); code != 1 || !strings.Contains(out.String()+errOut.String(), tc.want) {
			t.Fatalf("%v: code=%d out=%q err=%q", tc.args, code, out.String(), errOut.String())
		}
	}
}

func TestBenchmarkRunsAgainstInjectedHTTPWithDisposableModels(t *testing.T) {
	root := t.TempDir()
	for _, name := range []string{"alpha.gguf", "beta.gguf"} {
		if err := os.WriteFile(filepath.Join(root, name), []byte("fixture"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	var out, errOut bytes.Buffer
	env := testEnvironment(&out, &errOut, &fakeRunner{})
	env.LocalAdmin.LoadConfig = func() *config.Config { return &config.Config{ModelsDir: root, ServerPort: 12999} }
	requests := 0
	env.HTTP = &http.Client{Transport: localRoundTrip(func(req *http.Request) (*http.Response, error) {
		if req.URL.Host != "127.0.0.1:12999" {
			t.Fatalf("unexpected host %s", req.URL.Host)
		}
		body := ""
		switch req.URL.Path {
		case "/health":
			return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader("ok")), Header: http.Header{}}, nil
		case "/v1/completions":
			requests++
			body = `{"usage":{"prompt_tokens":4,"completion_tokens":8}}`
		default:
			t.Fatalf("unexpected path %s", req.URL.Path)
		}
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(body)), Header: http.Header{}}, nil
	})}
	if code := Execute(context.Background(), []string{"benchmark", "alpha", "--iterations", "1"}, env); code != 0 || !strings.Contains(out.String(), "Results") {
		t.Fatalf("benchmark: code=%d stdout=%q stderr=%q", code, out.String(), errOut.String())
	}
	out.Reset()
	errOut.Reset()
	if code := Execute(context.Background(), []string{"compare", "alpha", "beta", "--iterations", "1"}, env); code != 0 || !strings.Contains(out.String(), "Comparison Results") {
		t.Fatalf("comparison: code=%d stdout=%q stderr=%q", code, out.String(), errOut.String())
	}
	if requests != 4 {
		t.Fatalf("completion requests = %d, want 4", requests)
	}
}

func TestBenchmarkCancellationReturns130(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "alpha.gguf"), []byte("fixture"), 0600); err != nil {
		t.Fatal(err)
	}
	var out, errOut bytes.Buffer
	env := testEnvironment(&out, &errOut, &fakeRunner{})
	env.LocalAdmin.LoadConfig = func() *config.Config { return &config.Config{ModelsDir: root, ServerPort: 12999} }
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	env.HTTP = &http.Client{Transport: localRoundTrip(func(req *http.Request) (*http.Response, error) {
		if req.URL.Path == "/health" {
			return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader("ok")), Header: http.Header{}}, nil
		}
		cancel()
		return nil, ctx.Err()
	})}
	if code := Execute(ctx, []string{"benchmark", "alpha", "--iterations", "1"}, env); code != 130 {
		t.Fatalf("cancelled benchmark exit=%d stderr=%q", code, errOut.String())
	}
}
