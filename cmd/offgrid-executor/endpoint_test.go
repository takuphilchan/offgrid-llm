package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/takuphilchan/offgrid-llm/internal/execution"
)

func TestPinnedStdioEndpointRunsRealExecutor(t *testing.T) {
	ext := ""
	if runtime.GOOS == "windows" {
		ext = ".exe"
	}
	binary := filepath.Join(t.TempDir(), "offgrid-executor"+ext)
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	build := exec.CommandContext(ctx, filepath.Join(runtime.GOROOT(), "bin", "go"+ext), "build", "-o", binary, ".")
	if output, err := build.CombinedOutput(); err != nil {
		t.Fatal(string(output), err)
	}
	data, err := os.ReadFile(binary)
	if err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(data)
	digest := hex.EncodeToString(sum[:])
	root := t.TempDir()
	grant := filepath.Join(t.TempDir(), "grant.json")
	state := filepath.Join(t.TempDir(), "journal")
	b := execution.Binding{Actor: "fixture", Workspace: "workspace", Host: "host", Session: "session", Grant: "grant", Revision: 1, Job: "supervisor"}
	config := execution.Context{Version: 1, Binding: b, Backend: "local", Root: root, Directory: ".", Permissions: execution.Permissions{Read: true}}
	encoded, _ := json.Marshal(config)
	if err = os.WriteFile(grant, encoded, 0600); err != nil {
		t.Fatal(err)
	}
	if err = execution.ProtectPrivatePath(grant); err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(filepath.Join(root, "exact.txt"), []byte("café 日本語"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err = execution.StartStdio(ctx, binary, strings.Repeat("0", 64), grant, state); err == nil {
		t.Fatal("wrong pack digest accepted")
	}
	endpoint, err := execution.StartStdio(ctx, binary, digest, grant, state)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { endpoint.Close() })
	current, err := endpoint.Check(ctx, b)
	if err != nil || current != config {
		t.Fatal(current, err)
	}
	hosts := execution.NewHosts()
	if err = hosts.RegisterApproved(ctx, config, endpoint); err != nil {
		t.Fatal(err)
	}
	ref := execution.Reference{Host: b.Host, Session: b.Session, Grant: b.Grant, Revision: b.Revision}
	if err = hosts.Admit(ctx, b.Actor, b.Workspace, "canonical-job", ref); err != nil {
		t.Fatal(err)
	}
	response, err := hosts.Call(ctx, b.Actor, b.Workspace, "canonical-job", "read-once", ref, "file_read", []byte(`{"path":"exact.txt"}`))
	if err != nil || response.Error != "" || response.Receipt == nil {
		t.Fatal(response, err)
	}
	var page execution.FilePage
	if err = json.Unmarshal(response.Receipt.Result, &page); err != nil || string(page.Data) != "café 日本語" {
		t.Fatal(page, err)
	}
	if err = hosts.Cancel(ctx, b.Actor, b.Workspace, "canonical-job", ref); err != nil {
		t.Fatal(err)
	}
	late, err := hosts.Call(ctx, b.Actor, b.Workspace, "canonical-job", "late-read", ref, "file_read", []byte(`{"path":"exact.txt"}`))
	if err == nil && late.Error == "" {
		t.Fatal("cancelled job dispatched", late)
	}
	t.Run("shutdown settles owned command", func(t *testing.T) {
		testEndpointCommandShutdown(t, binary, digest)
	})
}
