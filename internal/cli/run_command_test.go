package cli

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/takuphilchan/offgrid-llm/internal/config"
	"github.com/takuphilchan/offgrid-llm/internal/models"
	"github.com/takuphilchan/offgrid-llm/internal/resource"
)

func TestRunBoundaryRequiresExplicitInteractiveAdapter(t *testing.T) {
	var out, errOut bytes.Buffer
	env := testEnvironment(&out, &errOut, &fakeRunner{})
	if code := Execute(context.Background(), []string{"run", "fixture"}, env); code != 1 || !strings.Contains(errOut.String(), "interactive run adapter unavailable") {
		t.Fatalf("missing adapter did not fail closed: code=%d stderr=%q", code, errOut.String())
	}
}

func TestRunHelpAndMissingModelStayInsideInjectedBoundary(t *testing.T) {
	var out, errOut bytes.Buffer
	env := testEnvironment(&out, &errOut, &fakeRunner{})
	env.ReadFile = os.ReadFile
	env.LocalAdmin.LoadConfig = func() *config.Config { return &config.Config{ModelsDir: t.TempDir()} }
	unexpected := func() error { return errors.New("unexpected process operation") }
	env.Interactive = InteractiveDependencies{
		DownloadHF:         func(context.Context, []string) error { t.Fatal("unexpected download"); return nil },
		EnsureServer:       unexpected,
		StartServer:        unexpected,
		EnsureLlama:        unexpected,
		StartLlama:         func(string) error { return unexpected() },
		ReadLlamaPort:      func(string) string { t.Fatal("unexpected port read"); return "" },
		WaitModelReady:     func(string, int) error { return unexpected() },
		Wait:               func(context.Context, int) error { return unexpected() },
		ReadDir:            os.ReadDir,
		ShouldUseTUI:       func() bool { t.Fatal("unexpected TUI check"); return false },
		RunTUI:             func(context.Context, *InteractiveTUIState) error { return unexpected() },
		IsEmbeddingModel:   func(...string) bool { t.Fatal("unexpected model check"); return false },
		EmbeddingChatError: func(string) string { t.Fatal("unexpected embedding error"); return "" },
		DetectResources:    func() (*resource.SystemResources, error) { t.Fatal("unexpected hardware probe"); return nil, nil },
	}
	if code := Execute(context.Background(), []string{"run", "--help"}, env); code != 0 || !strings.Contains(out.String(), "Interactive Chat") {
		t.Fatalf("help failed: code=%d stdout=%q stderr=%q", code, out.String(), errOut.String())
	}
	out.Reset()
	errOut.Reset()
	if code := Execute(context.Background(), []string{"run", "definitely-uninstalled-test-model"}, env); code != 1 || !strings.Contains(out.String(), "Model Not Found") {
		t.Fatalf("missing model changed: code=%d stdout=%q stderr=%q", code, out.String(), errOut.String())
	}
}

func TestRunAliasDownloadUsesTypedArgumentsAndContext(t *testing.T) {
	alias, ok := models.GetBuiltInAlias("tiny")
	if !ok {
		t.Fatal("missing built-in alias fixture")
	}
	var out, errOut bytes.Buffer
	env := testEnvironment(&out, &errOut, &fakeRunner{})
	env.Stdin = strings.NewReader("y\n")
	env.ReadFile = os.ReadFile
	env.LocalAdmin.LoadConfig = func() *config.Config { return &config.Config{ModelsDir: t.TempDir()} }
	sentinel := errors.New("fixture download stopped before network")
	called := false
	env.Interactive = InteractiveDependencies{
		DownloadHF: func(ctx context.Context, args []string) error {
			called = true
			if ctx.Err() != nil || len(args) != 3 || args[0] != alias.HFRepo || args[1] != "--file" || args[2] != alias.HFFile {
				t.Fatalf("wrong download handoff: %#v", args)
			}
			return sentinel
		},
		EnsureServer:       func() error { t.Fatal("unexpected server access"); return nil },
		StartServer:        func() error { t.Fatal("unexpected server start"); return nil },
		EnsureLlama:        func() error { t.Fatal("unexpected model access"); return nil },
		StartLlama:         func(string) error { t.Fatal("unexpected model start"); return nil },
		ReadLlamaPort:      func(string) string { return "" },
		WaitModelReady:     func(string, int) error { return nil },
		Wait:               func(context.Context, int) error { return nil },
		ReadDir:            os.ReadDir,
		ShouldUseTUI:       func() bool { return false },
		RunTUI:             func(context.Context, *InteractiveTUIState) error { return nil },
		IsEmbeddingModel:   func(...string) bool { return false },
		EmbeddingChatError: func(string) string { return "" },
		DetectResources:    func() (*resource.SystemResources, error) { return nil, nil },
	}
	if code := Execute(context.Background(), []string{"run", "tiny"}, env); code != 1 || !called || !strings.Contains(errOut.String(), sentinel.Error()) {
		t.Fatalf("alias handoff: code=%d called=%v stdout=%q stderr=%q", code, called, out.String(), errOut.String())
	}
}

func TestRunPlainChatUsesInjectedModelHTTPAndTerminal(t *testing.T) {
	t.Run("plain", func(t *testing.T) { testInjectedRunConversation(t, false, false) })
	t.Run("TUI failure falls back to plain", func(t *testing.T) { testInjectedRunConversation(t, true, false) })
	t.Run("TUI cancellation does not replay in plain mode", func(t *testing.T) { testInjectedRunConversation(t, true, true) })
}

func testInjectedRunConversation(t *testing.T, useTUI, cancelTUI bool) {
	modelDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(modelDir, "fixture.gguf"), []byte("isolated test fixture"), 0600); err != nil {
		t.Fatal(err)
	}
	var out, errOut bytes.Buffer
	env := testEnvironment(&out, &errOut, &fakeRunner{})
	env.Stdin = strings.NewReader("hello\n/exit\n")
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	env.ReadFile = os.ReadFile
	env.LocalAdmin.LoadConfig = func() *config.Config {
		return &config.Config{ModelsDir: modelDir, DataDir: t.TempDir(), ServerPort: 49491}
	}
	requests := 0
	env.HTTP = &http.Client{Transport: localRoundTrip(func(req *http.Request) (*http.Response, error) {
		var body string
		switch req.URL.Path {
		case "/health", "/models/refresh":
		case "/v1/models":
			body = `{"data":[{"id":"fixture"}]}`
		case "/v1/chat/completions":
			requests++
			data, err := io.ReadAll(req.Body)
			if err != nil || !bytes.Contains(data, []byte(`"content":"hello"`)) {
				t.Fatalf("wrong chat request: %q %v", data, err)
			}
			body = "data: {\"choices\":[{\"delta\":{\"content\":\"world\"}}]}\n\ndata: [DONE]\n\n"
		default:
			t.Fatalf("unexpected HTTP path %s", req.URL.Path)
		}
		return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(body)), Header: make(http.Header)}, nil
	})}
	unexpected := func() error { t.Fatal("unexpected process start"); return nil }
	env.Interactive = InteractiveDependencies{
		DownloadHF:     func(context.Context, []string) error { t.Fatal("unexpected download"); return nil },
		EnsureServer:   func() error { return nil },
		StartServer:    unexpected,
		EnsureLlama:    func() error { return nil },
		StartLlama:     func(string) error { return unexpected() },
		ReadLlamaPort:  func(string) string { t.Fatal("unexpected port read"); return "" },
		WaitModelReady: func(string, int) error { return unexpected() },
		Wait:           func(context.Context, int) error { return unexpected() },
		ReadDir:        os.ReadDir,
		ShouldUseTUI:   func() bool { return useTUI },
		RunTUI: func(ctx context.Context, _ *InteractiveTUIState) error {
			if !useTUI {
				t.Fatal("unexpected TUI run")
			}
			if cancelTUI {
				cancel()
				return ctx.Err()
			}
			return errors.New("unsupported terminal")
		},
		IsEmbeddingModel:   func(...string) bool { return false },
		EmbeddingChatError: func(string) string { return "" },
		DetectResources: func() (*resource.SystemResources, error) {
			return &resource.SystemResources{AvailableRAM: 8192, CPUCores: 4}, nil
		},
	}
	code := Execute(ctx, []string{"run", "fixture"}, env)
	if cancelTUI {
		if code != 130 || requests != 0 || strings.Contains(errOut.String(), "TUI unavailable") {
			t.Fatalf("cancellation replayed: code=%d requests=%d stdout=%q stderr=%q", code, requests, out.String(), errOut.String())
		}
		return
	}
	if code != 0 || requests != 1 || !strings.Contains(out.String(), "world") {
		t.Fatalf("plain run failed: code=%d requests=%d stdout=%q stderr=%q", code, requests, out.String(), errOut.String())
	}
	if useTUI && !strings.Contains(errOut.String(), "TUI unavailable") {
		t.Fatalf("missing fallback explanation: %q", errOut.String())
	}
}
