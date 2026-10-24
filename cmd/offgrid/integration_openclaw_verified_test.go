package main

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/takuphilchan/offgrid-llm/internal/integrations"
	"github.com/takuphilchan/offgrid-llm/internal/integrations/verification"
)

func TestVerifiedOpenClawRoster(t *testing.T) {
	for _, input := range []string{`null`, `[]`, `{}`, `[{"id":"../escape"}]`, `[{"id":"main"},{"id":"main"}]`, `[{"id":"one"},{"id":"two"}]`, `[{"id":"one","isDefault":true},{"id":"two","isDefault":true}]`} {
		if _, _, err := verifiedOpenClawAgents([]byte(input)); err == nil {
			t.Fatalf("accepted %s", input)
		}
	}
	ids, selected, err := verifiedOpenClawAgents([]byte(`[{"id":"work","isDefault":true},{"id":"research"}]`))
	if err != nil || selected != "work" || !reflect.DeepEqual(ids, []string{"work", "research"}) {
		t.Fatal(ids, selected, err)
	}
}

func TestVerifiedOpenClawLaunchPreservesConfigAndIsolatesState(t *testing.T) {
	configDir, workspace := t.TempDir(), t.TempDir()
	config := filepath.Join(configDir, "openclaw.json")
	original := []byte(`{"models":{"secret":"do-not-export"},"tools":{"deny":["exec"]}}`)
	if err := os.WriteFile(config, original, 0600); err != nil {
		t.Fatal(err)
	}
	digest, _ := integrations.ConfigurationDigest([]string{config})
	s := integrations.HostState{Executable: "fixture", ConfigFiles: []string{config}, ConfigDigest: digest}
	o := integrations.HostOptions{Verification: &verification.Connection{Workspace: workspace, RunID: strings.Repeat("a", 48), Token: "private-token"}}
	var setup integrations.SetupResponse
	setup.Integration.ModelID = "selected-model"
	old := runIntegrationProcess
	t.Cleanup(func() { runIntegrationProcess = old })
	t.Setenv("OPENCLAW_STATE_DIR", filepath.Join(configDir, "saved-state"))
	t.Setenv("OPENCLAW_INCLUDE_ROOTS", configDir)
	t.Setenv("OPENCLAW_AGENT_DIR", filepath.Join(configDir, "saved-agent"))
	launched := false
	runIntegrationProcess = func(cmd *exec.Cmd) error {
		if strings.Join(cmd.Args[1:], " ") == "agents list --json" {
			_, _ = io.WriteString(cmd.Stdout, `[{"id":"work","isDefault":true},{"id":"other"}]`)
			return nil
		}
		launched = true
		want := []string{"agent", "--local", "--agent", "work", "--session-id", o.Verification.RunID, "--model", "offgrid/selected-model", "--message", "hello"}
		if !reflect.DeepEqual(cmd.Args[1:], want) || cmd.Dir != workspace {
			t.Fatal(cmd.Args, cmd.Dir)
		}
		env := map[string]string{}
		for _, value := range cmd.Env {
			if k, v, ok := strings.Cut(value, "="); ok {
				env[k] = v
			}
		}
		root := env["OPENCLAW_STATE_DIR"]
		// This tree is owned solely by this fake invocation; no external process ran.
		t.Cleanup(func() { _ = os.RemoveAll(root) })
		if root == configDir || strings.HasPrefix(root, workspace+string(os.PathSeparator)) || env["OPENCLAW_CONFIG_READONLY"] != "1" {
			t.Fatal("state isolation failed")
		}
		if env["OPENCLAW_AGENT_DIR"] != filepath.Join(root, "agents", "work", "agent") || env["PI_CODING_AGENT_DIR"] != env["OPENCLAW_AGENT_DIR"] {
			t.Fatal("inherited agent state")
		}
		data, err := os.ReadFile(env["OPENCLAW_CONFIG_PATH"])
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(string(data), "do-not-export") || strings.Contains(string(data), "private-token") {
			t.Fatal("secret persisted")
		}
		var overlay map[string]any
		if json.Unmarshal(data, &overlay) != nil || overlay["$include"] != config {
			t.Fatal("lost original config reference")
		}
		if overlay["tools"] != nil || overlay["models"] != nil || overlay["plugins"] != nil {
			t.Fatal("overrode capability settings")
		}
		agents := overlay["agents"].(map[string]any)
		entries := agents["entries"].(map[string]any)
		for _, id := range []string{"work", "other"} {
			entry := entries[id].(map[string]any)
			if entry["workspace"] != workspace || entry["agentDir"] != filepath.Join(root, "agents", id, "agent") {
				t.Fatal("unscoped agent")
			}
		}
		return context.Canceled
	}
	if err := runVerifiedOpenClaw(context.Background(), o, s, setup, []string{"hello"}); !errors.Is(err, context.Canceled) || !launched {
		t.Fatal(err, launched)
	}
	got, _ := os.ReadFile(config)
	if !reflect.DeepEqual(got, original) {
		t.Fatal("saved config changed")
	}
	if os.Getenv("OPENCLAW_STATE_DIR") != filepath.Join(configDir, "saved-state") {
		t.Fatal("parent environment changed")
	}
	// A concurrent config edit must prevent launch, not be overwritten.
	if err := os.WriteFile(config, []byte(`{}`), 0600); err != nil {
		t.Fatal(err)
	}
	launched = false
	if err := runVerifiedOpenClaw(context.Background(), o, s, setup, []string{"hello"}); err == nil || launched {
		t.Fatal("accepted changed configuration")
	}
}
