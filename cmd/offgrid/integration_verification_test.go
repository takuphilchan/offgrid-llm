package main

import (
	"context"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/takuphilchan/offgrid-llm/internal/integrations"
	"github.com/takuphilchan/offgrid-llm/internal/integrations/verification"
)

func TestVerificationEnvironmentAndWorkingDirectoryAreInvocationLocal(t *testing.T) {
	t.Setenv("OFFGRID_VERIFY_TOKEN", "stale")
	t.Setenv("OFFGRID_VERIFY_URL", "http://untrusted.invalid")
	ordinary := integrations.HostOptions{ServerURL: "http://localhost:11611"}
	for _, e := range integrationEnvironment(ordinary) {
		if strings.HasPrefix(e, "OFFGRID_VERIFY_") {
			t.Fatal("inherited stale verifier")
		}
	}
	c := verification.Connection{URL: "http://127.0.0.1:12345", Token: "private-token", RunID: "run", Adapter: verification.HermesAdapter, Workspace: t.TempDir()}
	o := ordinary
	o.Verification = &c
	env := strings.Join(integrationEnvironment(o), "\n")
	if !strings.Contains(env, "OFFGRID_VERIFY_TOKEN=private-token") || strings.Contains(env, "stale") {
		t.Fatal("invalid environment")
	}
	factory := func(path string, args []string) (*exec.Cmd, error) { return exec.Command(path, args...), nil }
	cmd, err := verificationCommand(factory, o)("fixture", []string{"task"})
	if err != nil || cmd.Dir != c.Workspace || strings.Contains(strings.Join(cmd.Args, " "), c.Token) {
		t.Fatal("unscoped process")
	}
	cmd, err = verificationCommand(factory, ordinary)("fixture", nil)
	if err != nil || cmd.Dir != "" {
		t.Fatal("changed ordinary cwd")
	}
}

func TestVerificationPreflightRequiresPinnedEnabledTrustedHooks(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HERMES_HOME", filepath.Join(home, "hermes"))
	t.Setenv("HERMES_SAFE_MODE", "")
	h := newHermesHost(home)
	s := integrations.HostState{Executable: "fixture", Version: "Hermes Agent v0.21.2 (2026.9.11) · upstream f364c197"}
	if _, err := h.CheckVerification(context.Background(), integrations.HostOptions{}, s, []string{"hello"}); err == nil {
		t.Fatal("accepted missing plugin")
	}
	if _, err := integrations.InstallPluginBundle("hermes-verifier", home, false); err != nil {
		t.Fatal(err)
	}
	previous := runIntegrationProcess
	t.Cleanup(func() { runIntegrationProcess = previous })
	enabled := false
	runIntegrationProcess = func(cmd *exec.Cmd) error {
		if strings.Join(cmd.Args[1:], " ") != "config get plugins.enabled" {
			t.Fatal(cmd.Args)
		}
		if enabled {
			_, _ = io.WriteString(cmd.Stdout, "- offgrid-verifier\n")
		} else {
			_, _ = io.WriteString(cmd.Stdout, "[]")
		}
		return nil
	}
	if _, err := h.CheckVerification(context.Background(), integrations.HostOptions{}, s, []string{"hello"}); err == nil {
		t.Fatal("enabled plugin silently")
	}
	enabled = true
	if revision, err := h.CheckVerification(context.Background(), integrations.HostOptions{}, s, []string{"hello"}); err != nil || revision != verification.HermesAdapter {
		t.Fatal(revision, err)
	}
	for _, args := range [][]string{nil, {"--resume", "old"}, {"hello", "--toolsets", "all"}} {
		if _, err := h.CheckVerification(context.Background(), integrations.HostOptions{}, s, args); err == nil {
			t.Fatal("accepted unsupported invocation")
		}
	}
	s.Version = "Hermes Agent v0.22"
	if _, err := h.CheckVerification(context.Background(), integrations.HostOptions{}, s, []string{"hello"}); err == nil {
		t.Fatal("accepted unreviewed runtime")
	}
	bundle, err := integrations.InstallPluginBundle("openclaw", home, false)
	if err != nil {
		t.Fatal(err)
	}
	oc := newOpenClawHost(home)
	state := integrations.HostState{Executable: "fixture", Version: "OpenClaw 2026.10.4 (383c124)", PluginPath: bundle.Path}
	permission := false
	runIntegrationProcess = func(cmd *exec.Cmd) error {
		if strings.Join(cmd.Args[1:], " ") != "config get plugins.entries.offgrid --json" {
			t.Fatal(cmd.Args)
		}
		if permission {
			_, _ = io.WriteString(cmd.Stdout, `{"enabled":true,"hooks":{"allowConversationAccess":true}}`)
		} else {
			_, _ = io.WriteString(cmd.Stdout, `{"enabled":true}`)
		}
		return nil
	}
	if _, err := oc.CheckVerification(context.Background(), integrations.HostOptions{}, state, []string{"hello"}); err == nil {
		t.Fatal("accepted missing lifecycle permission")
	}
	permission = true
	if _, err := oc.CheckVerification(context.Background(), integrations.HostOptions{}, state, []string{"hello"}); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(bundle.Path, "verification.ts"), []byte("changed"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := oc.CheckVerification(context.Background(), integrations.HostOptions{}, state, []string{"hello"}); err == nil {
		t.Fatal("accepted modified hooks")
	}
}
