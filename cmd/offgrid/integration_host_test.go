package main

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/takuphilchan/offgrid-llm/internal/integrations"
)

func TestHermesHostConfigurationKeepsSecretsOutOfArgv(t *testing.T) {
	dir := t.TempDir()
	config := filepath.Join(dir, "config.yaml")
	env := filepath.Join(dir, ".env")
	if err := os.WriteFile(config, []byte("user_field: keep\n"), 0600); err != nil {
		t.Fatal(err)
	}
	store := integrations.ConnectionStore{Directory: filepath.Join(dir, "managed")}
	tx, err := store.Begin("hermes", []string{config, env})
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Close()
	previous := runIntegrationProcess
	t.Cleanup(func() { runIntegrationProcess = previous })
	count := 0
	runIntegrationProcess = func(cmd *exec.Cmd) error {
		count++
		if strings.Contains(strings.Join(cmd.Args, " "), "test-secret") {
			t.Fatal("credential exposed in argv")
		}
		if strings.Join(cmd.Args[1:3], " ") != "config set" {
			t.Fatalf("unexpected command: %v", cmd.Args)
		}
		return nil
	}
	o := integrations.HostOptions{ServerURL: "http://localhost:11611", APIKey: "test-secret"}
	s := integrations.HostState{Executable: "fixture-hermes", ConfigFiles: []string{config, env}}
	setup := integrations.SetupResponse{Integration: integrations.ServiceStatus{ModelID: "downloaded-model", ContextWindow: 65536}}
	if err = newHermesHost(dir).Configure(context.Background(), o, s, setup, tx); err != nil {
		t.Fatal(err)
	}
	if count != 5 {
		t.Fatalf("config commands=%d", count)
	}
	data, _ := os.ReadFile(config)
	if string(data) != "user_field: keep\n" {
		t.Fatal("unrelated config replaced")
	}
	data, _ = os.ReadFile(env)
	if !strings.Contains(string(data), "OFFGRID_API_KEY='test-secret'") {
		t.Fatal("credential not saved privately")
	}
}

func TestHermesVersionProbeDoesNotUseUserUpdateSettings(t *testing.T) {
	userHome := t.TempDir()
	t.Setenv("HERMES_HOME", userHome)
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	previousLookup := hermesLookPath
	hermesLookPath = func(string) (string, error) { return executable, nil }
	t.Cleanup(func() { hermesLookPath = previousLookup })
	previous := runIntegrationProcess
	t.Cleanup(func() { runIntegrationProcess = previous })
	versionCalls, configCalls := 0, 0
	runIntegrationProcess = func(cmd *exec.Cmd) error {
		arguments := strings.Join(cmd.Args, " ")
		if strings.Contains(arguments, "--version") {
			versionCalls++
			var probeHome string
			for _, entry := range cmd.Env {
				if strings.HasPrefix(entry, "HERMES_HOME=") {
					probeHome = strings.TrimPrefix(entry, "HERMES_HOME=")
				}
			}
			if probeHome == "" || probeHome == userHome {
				t.Fatal("version probe used the user's Hermes home")
			}
			config, err := os.ReadFile(filepath.Join(probeHome, "config.yaml"))
			if err != nil || string(config) != "updates:\n  check: false\n" {
				t.Fatal("version probe lacks a scoped update opt-out", err)
			}
			_, err = io.WriteString(cmd.Stdout, "Hermes Agent v0.21.2 (fixture)\n")
			return err
		}
		if strings.Contains(arguments, "config get") {
			configCalls++
			if cmd.Env != nil {
				t.Fatal("user configuration read inherited the probe override")
			}
			value := "offgrid"
			if strings.Contains(arguments, "model.context_length") {
				value = "65536"
			}
			_, err := io.WriteString(cmd.Stdout, value+"\n")
			return err
		}
		t.Fatalf("unexpected Hermes process: %v", cmd.Args)
		return nil
	}
	state, err := newHermesHost(userHome).Inspect(context.Background(), integrations.HostOptions{})
	if err != nil || versionCalls != 1 || configCalls != 4 || state.Version != "Hermes Agent v0.21.2 (fixture)" {
		t.Fatalf("version=%d config=%d state=%+v err=%v", versionCalls, configCalls, state, err)
	}
}

func TestOpenClawHostUsesNarrowValidatedPatch(t *testing.T) {
	dir := t.TempDir()
	config := filepath.Join(dir, "openclaw.json")
	if err := os.WriteFile(config, []byte(`{"agents":{"defaults":{"model":{"primary":"another/provider"}}}}`), 0600); err != nil {
		t.Fatal(err)
	}
	store := integrations.ConnectionStore{Directory: filepath.Join(dir, "managed")}
	tx, err := store.Begin("openclaw", []string{config})
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Close()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer test-secret" {
			t.Error("catalog not authenticated")
		}
		io.WriteString(w, `{"data":[{"id":"new-model","type":"llm","context_window":65536}]}`)
	}))
	defer server.Close()
	adapter, _ := integrations.NewDefaultRegistry().Get("openclaw")
	rendered, err := adapter.Render(integrations.Options{ModelID: "new-model", BaseURL: server.URL, ContextWindow: 32768, MaxOutputTokens: 4096})
	if err != nil {
		t.Fatal(err)
	}
	previous := runIntegrationProcess
	t.Cleanup(func() { runIntegrationProcess = previous })
	patches := 0
	runIntegrationProcess = func(cmd *exec.Cmd) error {
		if strings.Contains(strings.Join(cmd.Args, " "), "test-secret") {
			t.Fatal("secret in argv")
		}
		if cmd.Args[2] == "get" {
			_, err := io.WriteString(cmd.Stdout, `[{"id":"older-model","contextWindow":8192,"maxTokens":1024,"user":true}]`)
			return err
		}
		if strings.Join(cmd.Args[1:4], " ") != "config patch --stdin" {
			t.Fatalf("unexpected: %v", cmd.Args)
		}
		if (patches == 0) != strings.Contains(strings.Join(cmd.Args, " "), "--dry-run") {
			t.Fatal("patch applied before validation")
		}
		patches++
		var patch map[string]json.RawMessage
		if err := json.NewDecoder(cmd.Stdin).Decode(&patch); err != nil {
			t.Fatal(err)
		}
		if len(patch) != 1 || patch["models"] == nil {
			t.Fatal("global defaults changed")
		}
		if strings.Contains(string(patch["models"]), `"timeoutSeconds"`) {
			t.Fatal("managed merge must neither impose nor remove a user-owned provider timeout")
		}
		var models struct {
			Mode      string `json:"mode"`
			Providers map[string]struct {
				Key    string `json:"apiKey"`
				Models []struct {
					ID      string `json:"id"`
					Context int    `json:"contextWindow"`
					User    bool   `json:"user"`
				} `json:"models"`
			} `json:"providers"`
		}
		if err := json.Unmarshal(patch["models"], &models); err != nil {
			t.Fatal(err)
		}
		provider := models.Providers["offgrid"]
		if models.Mode != "" || len(models.Providers) != 1 || provider.Key != "test-secret" || len(provider.Models) != 2 || provider.Models[1].Context != 8192 || !provider.Models[1].User {
			t.Fatal("patch changed unrelated configuration")
		}
		return nil
	}
	err = newOpenClawHost(dir).Configure(context.Background(), integrations.HostOptions{ServerURL: server.URL, APIKey: "test-secret"}, integrations.HostState{Executable: "fixture-openclaw"}, integrations.SetupResponse{Setup: rendered, Integration: integrations.ServiceStatus{ModelID: "new-model"}}, tx)
	if err != nil || patches != 2 {
		t.Fatalf("patches=%d error=%v", patches, err)
	}
}

func TestManagedHermesRejectsPostSeparatorModelOverride(t *testing.T) {
	for _, args := range [][]string{{"--model=other"}, {"-m", "other"}, {"--provider", "cloud"}} {
		if err := newHermesHost(t.TempDir()).Run(context.Background(), integrations.HostOptions{}, integrations.HostState{}, integrations.SetupResponse{}, args); err == nil {
			t.Fatal("preflight override accepted")
		}
	}
}

func TestOpenClawRunPreservesToolsAndDefaultsWithExplicitBudget(t *testing.T) {
	previous := runIntegrationProcess
	t.Cleanup(func() { runIntegrationProcess = previous })
	calls := 0
	runIntegrationProcess = func(cmd *exec.Cmd) error {
		calls++
		joined := strings.Join(cmd.Args[1:], " ")
		for _, required := range []string{"agent exec hello", "--model offgrid/chosen", "--code-mode direct"} {
			if !strings.Contains(joined, required) {
				t.Fatalf("missing %s: %s", required, joined)
			}
		}
		for _, forbidden := range []string{"--local-model-lean", "--isolated", "--auth-env-only", "--config", "--tools", "--fallback", "--thinking"} {
			if strings.Contains(joined, forbidden) {
				t.Fatalf("configured capability changed: %s", joined)
			}
		}
		if (calls == 2) != strings.Contains(joined, "--timeout 900") {
			t.Fatalf("timeout is not invocation-local: %s", joined)
		}
		return nil
	}
	host := newOpenClawHost(t.TempDir())
	state := integrations.HostState{Executable: "fixture-openclaw"}
	setup := integrations.SetupResponse{Integration: integrations.ServiceStatus{ModelID: "chosen"}}
	for _, args := range [][]string{{"hello"}, {"hello", "--timeout", "900"}} {
		if err := host.Run(context.Background(), integrations.HostOptions{}, state, setup, args); err != nil {
			t.Fatal(err)
		}
	}
	for _, args := range [][]string{{"hello", "--timeout", "0"}, {"hello", "--timeout", "-1"}, {"hello", "--timeout", "86401"}, {"hello", "--timeout", "bad"}, {"hello", "--tools", "none"}, {"hello", "--timeout"}} {
		if err := host.Run(context.Background(), integrations.HostOptions{}, state, setup, args); err == nil {
			t.Fatalf("invalid controls accepted: %v", args)
		}
	}
	if calls != 2 {
		t.Fatalf("invalid run dispatched: %d", calls)
	}
}

func TestManagedExternalProcessCancellation(t *testing.T) {
	if os.Getenv("OFFGRID_INTEGRATION_CANCEL_HELPER") == "1" {
		time.Sleep(30 * time.Second)
		return
	}
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	started := time.Now()
	err = externalCommand(ctx, hermesCommand, executable, []string{"-test.run=^TestManagedExternalProcessCancellation$"}, append(os.Environ(), "OFFGRID_INTEGRATION_CANCEL_HELPER=1"), nil, io.Discard, io.Discard)
	if err != context.DeadlineExceeded || time.Since(started) > 5*time.Second {
		t.Fatalf("cancellation: %v in %v", err, time.Since(started))
	}
}
