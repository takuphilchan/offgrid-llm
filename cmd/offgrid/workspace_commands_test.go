package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"filippo.io/age"
	"github.com/takuphilchan/offgrid-llm/internal/rag"
	"github.com/takuphilchan/offgrid-llm/internal/storage"
)

func TestWorkspaceCommandProcess(t *testing.T) {
	if os.Getenv("OFFGRID_WORKSPACE_TEST_HELPER") != "1" {
		return
	}
	Version = "9.9.9"
	var args []string
	if json.Unmarshal([]byte(os.Getenv("OFFGRID_WORKSPACE_TEST_ARGS")), &args) != nil {
		os.Exit(99)
	}
	os.Exit(runRoot(context.Background(), append([]string{"offgrid", "--json", "workspace"}, args...), osCLIEnvironment()))
}

func workspaceProcess(t *testing.T, input string, args ...string) (map[string]any, string, int) {
	t.Helper()
	self, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	command := exec.CommandContext(ctx, self, "-test.run=^TestWorkspaceCommandProcess$")
	encoded, _ := json.Marshal(args)
	command.Env = append(os.Environ(), "OFFGRID_WORKSPACE_TEST_HELPER=1", "OFFGRID_WORKSPACE_TEST_ARGS="+string(encoded))
	command.Stdin = strings.NewReader(input)
	var out, diagnostic bytes.Buffer
	command.Stdout = &out
	command.Stderr = &diagnostic
	err = command.Run()
	code := 0
	if err != nil {
		var exit *exec.ExitError
		if !errors.As(err, &exit) {
			t.Fatal(err)
		}
		code = exit.ExitCode()
	}
	var result map[string]any
	if json.Unmarshal(out.Bytes(), &result) != nil {
		t.Fatalf("invalid command JSON: %q (stderr %q, code %d)", out.String(), diagnostic.String(), code)
	}
	return result, out.String() + diagnostic.String(), code
}

func TestWorkspaceEncryptedCLIContentAndLegacyJourneys(t *testing.T) {
	key, err := age.GenerateX25519Identity()
	if err != nil {
		t.Fatal(err)
	}
	parent := t.TempDir()
	source := filepath.Join(parent, "source")
	if err = os.Mkdir(source, 0700); err != nil {
		t.Fatal(err)
	}
	content := []byte("private fixture — café 日本語\n")
	if err = os.WriteFile(filepath.Join(source, "history.txt"), content, 0600); err != nil {
		t.Fatal(err)
	}
	identity := filepath.Join(parent, "identity.txt")
	if err = os.WriteFile(identity, []byte(key.String()+"\n"), 0600); err != nil {
		t.Fatal(err)
	}
	for _, mode := range []string{"recipient", "passphrase", "legacy"} {
		t.Run(mode, func(t *testing.T) {
			archive := filepath.Join(parent, mode+".misleading")
			backupArgs := []string{"backup", "--data-dir", source, "--output", archive}
			readArgs := []string{}
			secret := ""
			encryption := "none"
			if mode == "recipient" {
				backupArgs = append(backupArgs, "--recipient", key.Recipient().String())
				readArgs = []string{"--identity", identity}
				encryption = "age-v1"
			}
			if mode == "passphrase" {
				backupArgs = append(backupArgs, "--passphrase-stdin")
				readArgs = []string{"--passphrase-stdin"}
				secret = "  isolated passphrase 日本語  \r\n"
				encryption = "age-v1"
			}
			result, transcript, code := workspaceProcess(t, secret, backupArgs...)
			if code != 0 || result["encryption"] != encryption || result["authenticated"] != (encryption == "age-v1") {
				t.Fatalf("backup: %d %s", code, transcript)
			}
			result, transcript, code = workspaceProcess(t, secret, append([]string{"verify", archive}, readArgs...)...)
			if code != 0 || result["encryption"] != encryption {
				t.Fatalf("verify: %d %s", code, transcript)
			}
			target := filepath.Join(parent, "restored-"+mode)
			result, transcript, code = workspaceProcess(t, secret, append([]string{"restore", archive, "--data-dir", target, "--yes"}, readArgs...)...)
			if code != 0 || result["encryption"] != encryption {
				t.Fatalf("restore: %d %s", code, transcript)
			}
			actual, err := os.ReadFile(filepath.Join(target, "history.txt"))
			if err != nil || !bytes.Equal(actual, content) {
				t.Fatal("restored inventory mismatch", err)
			}
			if strings.Contains(transcript, key.String()) || strings.Contains(transcript, "isolated passphrase") {
				t.Fatal("secret leaked in command output")
			}
			if mode == "legacy" {
				_, _, code = workspaceProcess(t, "", "verify", archive, "--identity", identity)
				if code == 0 {
					t.Fatal("encryption-only verify silently accepted plaintext")
				}
			}
		})
	}
}

func workspaceTestStreams() workspaceStreams {
	return workspaceStreams{in: strings.NewReader(""), out: io.Discard, err: io.Discard, json: true, version: "9.9.9", rebuild: rag.RebuildKnowledgeOfflineWithBackup, terminalSecret: func(context.Context, io.Writer, bool) ([]byte, error) {
		return nil, errors.New("unexpected terminal prompt")
	}}
}

func TestWorkspaceInvalidOptionsDoNotLeakOrMutate(t *testing.T) {
	for _, extra := range [][]string{{"--passphrase=private-sentinel"}, {"--passphrase=false"}, {"--passphrase-stdin=false"}, {"--identity", "private-sentinel"}, {"--passphrase", "--passphrase-stdin"}, {"--recipient", "private-sentinel"}, {"--passphrase", "--recipient", "age1invalid"}, {"--unknown=private-sentinel"}} {
		root := t.TempDir()
		target := filepath.Join(root, "out")
		err := runWorkspaceCommandWithIO(context.Background(), append([]string{"backup", "--data-dir", root, "--output", target}, extra...), workspaceTestStreams())
		if err == nil || strings.Contains(err.Error(), "private-sentinel") {
			t.Fatalf("unsafe option diagnostic: %v", err)
		}
		entries, _ := os.ReadDir(root)
		if len(entries) != 0 {
			t.Fatal("invalid options mutated source", entries)
		}
	}
	_, transcript, code := workspaceProcess(t, "", "verify", "missing", "--passphrase=private-sentinel")
	if code != 2 || strings.Contains(transcript, "private-sentinel") {
		t.Fatalf("usage exit/redaction: %d %s", code, transcript)
	}
	for _, command := range []string{"verify", "restore"} {
		_, transcript, code = workspaceProcess(t, "", command, "--passphrase=private-sentinel")
		if code != 2 || strings.Contains(transcript, "private-sentinel") {
			t.Fatalf("missing archive leaked secret: %d %s", code, transcript)
		}
	}
}

func TestWorkspaceRebuildRequiresPublishedEncryptedRecovery(t *testing.T) {
	key, _ := age.GenerateX25519Identity()
	parent := t.TempDir()
	source := filepath.Join(parent, "source")
	if err := os.Mkdir(source, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(source, "retained.txt"), []byte("original"), 0600); err != nil {
		t.Fatal(err)
	}
	archive := filepath.Join(parent, "recovery")
	args := []string{"rebuild-knowledge", "--data-dir", source, "--model-id", "fixture", "--model-file", "fixture.gguf", "--runtime", "fixture-runtime", "--output", archive, "--yes", "--recipient", key.Recipient().String()}
	streams := workspaceTestStreams()
	err := runWorkspaceCommandWithIO(context.Background(), args, streams)
	if err == nil || !strings.Contains(err.Error(), "recovery backup retained") {
		t.Fatal(err)
	}
	manifest, verifyErr := storage.VerifyBackupWithOptions(context.Background(), archive, storage.BackupReadOptions{Identities: []age.Identity{key}, RequireEncryption: true})
	if verifyErr != nil || manifest.Encryption != "age-v1" {
		t.Fatal("rebuild failure lost verified encrypted recovery", verifyErr)
	}
	// A colliding backup path must prevent the subsequent mutation boundary.
	err = runWorkspaceCommandWithIO(context.Background(), args, streams)
	if err == nil || !strings.Contains(err.Error(), "backup output already exists") {
		t.Fatal("rebuild passed backup failure", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err = runWorkspaceCommandWithIO(ctx, args, streams); !errors.Is(err, context.Canceled) {
		t.Fatal("cancelled rebuild dispatched", err)
	}
	actual, _ := os.ReadFile(filepath.Join(source, "retained.txt"))
	if string(actual) != "original" {
		t.Fatal("source changed")
	}
}
