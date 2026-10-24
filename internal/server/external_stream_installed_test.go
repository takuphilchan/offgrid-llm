//go:build linux || darwin

package server

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync/atomic"
	"syscall"
	"testing"
	"time"

	"github.com/takuphilchan/offgrid-llm/internal/inference"
	"github.com/takuphilchan/offgrid-llm/internal/integrations"
	"github.com/takuphilchan/offgrid-llm/pkg/api"
)

type slowInstalledStreamEngine struct {
	inference.Engine
	calls atomic.Int32
	delay time.Duration
}

func (e *slowInstalledStreamEngine) ChatCompletionStreamRaw(ctx context.Context, _ *api.ChatCompletionRequest, emit inference.ChatCompletionStreamCallback) error {
	e.calls.Add(1)
	timer := time.NewTimer(e.delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return emit(json.RawMessage(`{"choices":[{"index":0,"delta":{"content":"OFFGRID_SLOW_STREAM_OK"},"finish_reason":"stop"}]}`))
	}
}

// Opt-in installed-runtime interoperability, NOT real-model qualification.
// The actual OffGrid HTTP stream waits longer than OpenClaw's 300s watchdog.
// This uses no live service, model, user configuration or accounts. Only file
// tools scoped to the disposable workspace are allowed; the response uses none.
func TestInstalledOpenClawSlowStream(t *testing.T) {
	if os.Getenv("OFFGRID_TEST_INSTALLED_OPENCLAW_STREAM") != "1" {
		t.Skip("opt-in installed OpenClaw slow-stream check")
	}
	executable, err := exec.LookPath("openclaw")
	if err != nil {
		t.Fatal(err)
	}
	s := newTestServer(t)
	engine := &slowInstalledStreamEngine{delay: 310 * time.Second}
	s.engine = engine
	s.llamaProcess = &retryDetectLlamaProcess{}
	if err := os.WriteFile(filepath.Join(s.config.ModelsDir, "fixture.gguf"), []byte("fixture"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := s.registry.ScanModels(); err != nil {
		t.Fatal(err)
	}
	model := s.registry.ListModels()[0].ID
	server := httptest.NewServer(s.loggingMiddleware(http.HandlerFunc(s.handleChatCompletions)))
	defer server.Close()
	root := t.TempDir()
	adapter, _ := integrations.NewDefaultRegistry().Get("openclaw")
	setup, err := adapter.Render(integrations.Options{BaseURL: server.URL, ModelID: model, ContextWindow: 65536, MaxOutputTokens: 512})
	if err != nil {
		t.Fatal(err)
	}
	var configuration map[string]any
	if err := json.Unmarshal([]byte(setup.Content), &configuration); err != nil {
		t.Fatal(err)
	}
	// Use the real generated provider settings: a hand-authored provider omitted
	// the old 300-second deadline and allowed a misleading interoperability pass.
	configuration["agents"] = map[string]any{"defaults": map[string]any{"workspace": root}}
	configuration["tools"] = map[string]any{"allow": []string{"read", "write"}, "fs": map[string]any{"workspaceOnly": true}}
	data, _ := json.Marshal(configuration)
	configPath := filepath.Join(root, "openclaw.json")
	if err := os.WriteFile(configPath, data, 0600); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 380*time.Second)
	defer cancel()
	// Exercise the normal launch, without an explicit timeout masking the
	// installed runtime's separate first-output watchdog.
	command := exec.CommandContext(ctx, executable, "agent", "exec", "Reply briefly without using tools.", "--model", "offgrid/"+model, "--code-mode", "direct", "--config", configPath)
	// The launcher may fork a runtime that inherits stdout. Bound cleanup of
	// this private fixture's whole process group, not only its launcher.
	command.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	command.Cancel = func() error { return syscall.Kill(-command.Process.Pid, syscall.SIGKILL) }
	command.WaitDelay = 2 * time.Second
	command.Dir = root
	command.Env = []string{"PATH=" + os.Getenv("PATH"), "HOME=" + root, "OPENCLAW_STATE_DIR=" + filepath.Join(root, "state"), "OPENCLAW_CONFIG_PATH=" + configPath, "NO_COLOR=1", "TERM=dumb"}
	started := time.Now()
	out, err := command.CombinedOutput()
	if err != nil || !strings.Contains(string(out), "OFFGRID_SLOW_STREAM_OK") || engine.calls.Load() != 1 {
		t.Fatalf("slow stream failed or retried: calls=%d error=%v output=%s", engine.calls.Load(), err, out)
	}
	t.Log(fmt.Sprintf("Installed OpenClaw accepted one delayed 310s response in %s without retry; fixture only", time.Since(started).Round(time.Millisecond)))
}
