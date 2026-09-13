package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseOpenClawOptions(t *testing.T) {
	options, err := parseOpenClawOptions([]string{
		"--server", "http://127.0.0.1:11611/", "--base-url", "https://inference.example", "--model", "chat-model", "--yes", "--force",
	})
	if err != nil {
		t.Fatal(err)
	}
	if options.ServerURL != "http://127.0.0.1:11611" || options.BaseURL != "https://inference.example" || options.ModelID != "chat-model" || !options.Yes || !options.Force {
		t.Fatalf("unexpected parsed options: %+v", options)
	}
	if _, err := parseOpenClawOptions([]string{"--model"}); err == nil {
		t.Fatal("missing model value was accepted")
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
