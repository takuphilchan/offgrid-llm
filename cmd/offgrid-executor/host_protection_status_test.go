package main

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/takuphilchan/offgrid-llm/internal/execution"
	"github.com/takuphilchan/offgrid-llm/internal/testhost"
)

func TestHostProtectionStatusNativeCLI(t *testing.T) {
	testhost.Account(t)
	t.Setenv("OFFGRID_EXECUTOR_TOKEN", "")
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	inspect := func(args ...string) execution.HostProtectionInspection {
		t.Helper()
		command := exec.CommandContext(ctx, os.Args[0], append([]string{"-test.run=^TestExecutorSubprocess$", "--", "--host-protection-status"}, args...)...)
		command.Env = append(os.Environ(), "OFFGRID_EXECUTOR_TEST_CHILD=1")
		output, err := command.Output()
		if err != nil {
			t.Fatalf("native inspection: %v, output=%q", err, output)
		}
		var report execution.HostProtectionInspection
		if err := json.Unmarshal(output, &report); err != nil || report.Version != 1 {
			t.Fatalf("invalid diagnostic: %+v: %v", report, err)
		}
		for _, secret := range []string{"capability", "identity_hash", "capability_hash"} {
			if strings.Contains(string(output), secret) {
				t.Fatalf("diagnostic exposed private authority: %q", output)
			}
		}
		return report
	}
	config, err := os.UserConfigDir()
	if err != nil {
		t.Fatal(err)
	}
	if report := inspect(); report.Status != "absent" || len(report.Records) != 0 {
		t.Fatalf("empty account diagnostic: %+v", report)
	}
	if _, err := os.Stat(filepath.Join(config, "offgrid")); !os.IsNotExist(err) {
		t.Fatalf("inspection initialized account state: %v", err)
	}
	registry, err := execution.AccountHostProtectionRegistry()
	if err != nil {
		t.Fatal(err)
	}
	for range 2 {
		if _, err := registry.RegisterBrowserRoot(ctx, testhost.PrivateDirectory(t)); err != nil {
			t.Fatal(err)
		}
	}
	first := inspect("--host-protection-limit", "1")
	if first.Status != "ready" || len(first.Records) != 1 || first.Next == "" {
		t.Fatalf("first diagnostic page: %+v", first)
	}
	second := inspect("--host-protection-limit", "1", "--host-protection-after", first.Next)
	if len(second.Records) != 1 || second.Next != "" || second.Records[0].ID == first.Records[0].ID {
		t.Fatalf("second diagnostic page: %+v", second)
	}
}

func TestHostProtectionStatusRejectsMixedAuthority(t *testing.T) {
	testhost.Account(t)
	for _, args := range [][]string{
		{"--host-protection-status", "--stdio"},
		{"--host-protection-status", "--connect"},
		{"--host-protection-status", "--grant-file", "foreign"},
		{"--host-protection-status", "--state-dir", "foreign"},
		{"--host-protection-status", "unexpected"},
		{"--host-protection-limit", "1"},
		{"--host-protection-after", strings.Repeat("a", 64)},
	} {
		if err := run(args); err == nil {
			t.Fatalf("mixed diagnostic authority accepted: %v", args)
		}
	}
}
