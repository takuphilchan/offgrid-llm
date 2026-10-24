package main

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/takuphilchan/offgrid-llm/pkg/api"
)

func TestMergeOpenClawModelsPreservesExistingEntries(t *testing.T) {
	provider := map[string]any{"models": []any{map[string]any{"id": "llama", "contextWindow": float64(32768), "maxTokens": float64(4096)}}}
	qwen := json.RawMessage(`{"id":"qwen","contextWindow":8192,"maxTokens":4096,"custom":{"keep":true}}`)
	catalog := []api.Model{{ID: "qwen", Type: "llm", ContextWindow: 32768}, {ID: "llama", Type: "llm", ContextWindow: 32768}}
	if err := mergeOpenClawModels(provider, []json.RawMessage{qwen}, catalog); err != nil {
		t.Fatal(err)
	}
	models, ok := provider["models"].([]json.RawMessage)
	if !ok || len(models) != 2 || string(models[1]) != string(qwen) {
		t.Fatalf("existing model was not preserved: %#v", provider["models"])
	}
	var added struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(models[0], &added); err != nil || added.ID != "llama" {
		t.Fatalf("selected model not preferred: %s, %v", models[0], err)
	}
	provider = map[string]any{"models": []any{map[string]any{"id": "llama", "contextWindow": float64(32768), "maxTokens": float64(4096)}}}
	llama := json.RawMessage(`{"id":"llama","contextWindow":32768,"maxTokens":4096,"custom":{"keep":true}}`)
	if err := mergeOpenClawModels(provider, []json.RawMessage{qwen, llama}, catalog); err != nil {
		t.Fatal(err)
	}
	models = provider["models"].([]json.RawMessage)
	if len(models) != 2 || !strings.Contains(string(models[0]), `"id":"llama"`) || !strings.Contains(string(models[0]), `"custom":{"keep":true}`) || !strings.Contains(string(models[1]), `"id":"qwen"`) {
		t.Fatalf("existing selected model was not preferred intact: %#v", models)
	}
	provider = map[string]any{"models": []any{map[string]any{"id": "qwen", "contextWindow": float64(32768), "maxTokens": float64(4096)}}}
	if err := mergeOpenClawModels(provider, []json.RawMessage{qwen}, catalog); err != nil {
		t.Fatal(err)
	}
	models = provider["models"].([]json.RawMessage)
	if len(models) != 1 || !strings.Contains(string(models[0]), `"custom":{"keep":true}`) || !strings.Contains(string(models[0]), `"contextWindow":32768`) {
		t.Fatalf("reinstall changed the existing model: %#v", models)
	}
}

func TestMergeOpenClawModelsRejectsUnsafeConfig(t *testing.T) {
	for _, existing := range [][]json.RawMessage{
		{json.RawMessage(`{"name":"missing id"}`)},
		{json.RawMessage(`{"id":"qwen"}`), json.RawMessage(`{"id":"qwen"}`)},
		{json.RawMessage(`not json`)},
	} {
		provider := map[string]any{"models": []any{map[string]any{"id": "llama", "contextWindow": float64(32768), "maxTokens": float64(4096)}}}
		if err := mergeOpenClawModels(provider, existing, []api.Model{{ID: "llama", Type: "llm", ContextWindow: 32768}}); err == nil {
			t.Fatalf("unsafe existing models accepted: %s", existing)
		}
	}
}

func TestMergeOpenClawModelsRejectsChangedCatalogContext(t *testing.T) {
	provider := map[string]any{"models": []any{map[string]any{"id": "llama", "contextWindow": float64(32768), "maxTokens": float64(4096)}}}
	if err := mergeOpenClawModels(provider, nil, []api.Model{{ID: "llama", Type: "llm", ContextWindow: 16384}}); err == nil {
		t.Fatal("changed model context was accepted")
	}
}

func TestReadOpenClawModelsDistinguishesUnsetFromFailure(t *testing.T) {
	original := runIntegrationProcess
	t.Cleanup(func() { runIntegrationProcess = original })
	for _, test := range []struct {
		name, stdout string
		runError     error
		wantCount    int
		wantError    bool
	}{
		{name: "configured", stdout: `[{"id":"qwen"}]`, wantCount: 1},
		{name: "fresh", stdout: `{"ok":false,"error":{"message":"Config path is valid but unset: models.providers.offgrid.models"}}`, runError: errors.New("exit status 1")},
		{name: "malformed", stdout: `{}`, wantError: true},
		{name: "other failure", stdout: `{"error":"permission denied"}`, runError: errors.New("exit status 1"), wantError: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			runIntegrationProcess = func(command *exec.Cmd) error {
				if got := command.Args[1:]; len(got) == 3 && got[0] == "config" && got[1] == "get" && got[2] == "agents.defaults.model.primary" {
					_, _ = io.WriteString(command.Stdout, "Config path is valid but unset: agents.defaults.model.primary")
					return errors.New("exit status 1")
				} else if len(got) != 4 || got[0] != "config" || got[1] != "get" || got[2] != "models.providers.offgrid.models" || got[3] != "--json" {
					t.Fatalf("unexpected OpenClaw command: %v", got)
				}
				_, _ = io.WriteString(command.Stdout, test.stdout)
				return test.runError
			}
			models, err := readOpenClawModels(context.Background(), "openclaw")
			if (err != nil) != test.wantError || len(models) != test.wantCount {
				t.Fatalf("models = %s, error = %v", models, err)
			}
		})
	}
}

func TestOpenClawInstallerHostIsNarrow(t *testing.T) {
	for _, host := range []string{"openclaw.ai", "raw.githubusercontent.com"} {
		if !trustedOpenClawInstallerHost(host) {
			t.Fatalf("trusted installer host %q rejected", host)
		}
	}
	for _, host := range []string{"evil.openclaw.ai", "openclaw.ai.evil.example", "github.com"} {
		if trustedOpenClawInstallerHost(host) {
			t.Fatalf("untrusted installer host %q accepted", host)
		}
	}
}

func TestOpenClawExecutableOverride(t *testing.T) {
	path := filepath.Join(t.TempDir(), "openclaw.mjs")
	if err := os.WriteFile(path, []byte("// test entry\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("OFFGRID_OPENCLAW_BIN", path)
	got, err := findOpenClawExecutable(t.TempDir())
	if err != nil || got != path {
		t.Fatalf("findOpenClawExecutable() = %q, %v", got, err)
	}
	t.Setenv("OFFGRID_OPENCLAW_BIN", filepath.Join(t.TempDir(), "missing"))
	if _, err := findOpenClawExecutable(t.TempDir()); err == nil || !strings.Contains(err.Error(), "OFFGRID_OPENCLAW_BIN") {
		t.Fatalf("invalid override was accepted: %v", err)
	}
}

func TestParseOpenClawInferResult(t *testing.T) {
	valid := []byte(`{"ok":true,"provider":"offgrid","model":"chat","outputs":[{"text":"hello"}]}`)
	if reply, err := parseOpenClawInferResult(valid, "chat"); err != nil || reply != "hello" {
		t.Fatalf("valid model result = %q, %v", reply, err)
	}
	for _, data := range [][]byte{
		[]byte(`not json`),
		[]byte(`{"ok":false,"provider":"offgrid","model":"chat","outputs":[{"text":"hello"}]}`),
		[]byte(`{"ok":true,"provider":"other","model":"chat","outputs":[{"text":"hello"}]}`),
		[]byte(`{"ok":true,"provider":"offgrid","model":"wrong","outputs":[{"text":"hello"}]}`),
		[]byte(`{"ok":true,"provider":"offgrid","model":"chat","outputs":[{"text":" "}]}`),
	} {
		if reply, err := parseOpenClawInferResult(data, "chat"); err == nil {
			t.Fatalf("invalid result accepted with reply %q: %s", reply, data)
		}
	}
}
