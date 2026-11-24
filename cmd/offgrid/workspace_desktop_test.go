package main

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/json"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"filippo.io/age"
	"github.com/takuphilchan/offgrid-llm/internal/storage"
)

func TestDesktopMaintenanceProcessHelper(t *testing.T) {
	if os.Getenv("OFFGRID_MAINTENANCE_PROCESS_TEST") != "1" {
		return
	}
	Version = "desktop-test-v1"
	if os.Getenv("OFFGRID_MAINTENANCE_PUBLISHED_TEST") == "1" {
		os.Exit(runPublishedMaintenanceFixture())
	}
	os.Exit(runRoot(context.Background(), []string{"offgrid", "workspace", "--desktop-stdio"}, osCLIEnvironment()))
}

func startDesktopMaintenanceTest(t *testing.T, environment ...string) (*exec.Cmd, io.WriteCloser, io.ReadCloser, *bytes.Buffer) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	command := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestDesktopMaintenanceProcessHelper$")
	command.Env = append(os.Environ(), "OFFGRID_MAINTENANCE_PROCESS_TEST=1")
	command.Env = append(command.Env, environment...)
	input, err := command.StdinPipe()
	if err != nil {
		t.Fatal(err)
	}
	output, err := command.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	diagnostics := &bytes.Buffer{}
	command.Stderr = diagnostics
	if err := command.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		input.Close()
		if command.ProcessState == nil {
			command.Process.Kill()
			command.Wait()
		}
		cancel()
	})
	return command, input, output, diagnostics
}

// This branch exists only in the test executable. The normal production binary
// has no environment-controlled fault switch. Its real storage callback returns
// cancellation after verified publication while the shared projector emits the
// same bounded private result used by runDesktopMaintenance.
func runPublishedMaintenanceFixture() int {
	ctx := context.Background()
	var request desktopMaintenanceRequest
	if readDesktopMaintenanceStart(ctx, os.Stdin, &request) != nil || request.Operation != "backup" {
		return 2
	}
	options, _, err := request.options(ctx)
	if err != nil {
		return 2
	}
	if desktopWriteFrame(os.Stdout, desktopMaintenanceState{Protocol: 1, ID: request.ID, Phase: "working"}) != nil {
		return 1
	}
	manifest, err := storage.BackupWorkspaceThen(ctx, request.DataDir, request.Destination, Version, options, func(*storage.BackupManifest) error {
		return context.Canceled
	})
	state, exitCode := desktopMaintenanceOutcome(request, manifest, err, nil)
	if desktopWriteFrame(os.Stdout, state) != nil {
		return 1
	}
	return exitCode
}

func TestDesktopMaintenanceActualPostPublicationCancellationRetainsArchive(t *testing.T) {
	key, err := age.GenerateX25519Identity()
	if err != nil {
		t.Fatal(err)
	}
	root, data := t.TempDir(), t.TempDir()
	if err := os.WriteFile(filepath.Join(data, "private.txt"), []byte("retained verified contents"), 0600); err != nil {
		t.Fatal(err)
	}
	archive := filepath.Join(root, "published.age")
	command, input, output, diagnostics := startDesktopMaintenanceTest(t, "OFFGRID_MAINTENANCE_PUBLISHED_TEST=1")
	request := desktopMaintenanceRequest{Protocol: 1, Kind: "start", ID: strings.Repeat("e", 64), Operation: "backup", Encryption: "recipient", DataDir: data, Destination: archive, KeyMaterial: key.Recipient().String()}
	writeMaintenanceTestFrame(t, input, request)
	if state := readMaintenanceTestState(t, output); state.Phase != "working" {
		t.Fatal(state)
	}
	state := readMaintenanceTestState(t, output)
	if state.ID != request.ID || state.Outcome != "cancelled" || state.Code != "cancelled" || state.Summary == nil || !state.Summary.Published || !state.Summary.Authenticated || state.Summary.DatabaseChecked || state.Summary.Files != 1 {
		t.Fatalf("published evidence lost on cancellation: %+v", state)
	}
	if err := command.Wait(); err == nil || command.ProcessState.ExitCode() != 130 {
		t.Fatal("wrong post-publication exit", err, command.ProcessState.ExitCode())
	}
	manifest, err := storage.VerifyBackupWithOptions(context.Background(), archive, storage.BackupReadOptions{Identities: []age.Identity{key}, RequireEncryption: true})
	if err != nil || manifest.Bytes != state.Summary.Bytes || manifest.ApplicationVersion != state.Summary.ApplicationVersion {
		t.Fatal("retained archive does not match compact result", err)
	}
	encoded, _ := json.Marshal(state)
	if bytes.Contains(encoded, []byte("private.txt")) || bytes.Contains(encoded, []byte("retained verified contents")) || strings.Contains(diagnostics.String(), key.String()) {
		t.Fatal("private inventory/content/key escaped post-publication result")
	}
}

func writeMaintenanceTestFrame(t *testing.T, writer io.Writer, value any) {
	t.Helper()
	data, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	var header [4]byte
	binary.BigEndian.PutUint32(header[:], uint32(len(data)))
	if _, err := writer.Write(append(header[:], data...)); err != nil {
		t.Fatal(err)
	}
}

func readMaintenanceTestState(t *testing.T, reader io.Reader) desktopMaintenanceState {
	t.Helper()
	var state desktopMaintenanceState
	if err := desktopReadFrame(reader, 32768, &state); err != nil {
		t.Fatal(err)
	}
	return state
}

func TestDesktopMaintenanceActualEncryptedProcessJourney(t *testing.T) {
	key, err := age.GenerateX25519Identity()
	if err != nil {
		t.Fatal(err)
	}
	root, data := t.TempDir(), t.TempDir()
	if err := os.WriteFile(filepath.Join(data, "private.txt"), []byte("isolated private contents"), 0600); err != nil {
		t.Fatal(err)
	}
	archive, restored := filepath.Join(root, "misleading.zip"), filepath.Join(root, "restored")
	for _, operation := range []string{"backup", "verify", "restore"} {
		t.Run(operation, func(t *testing.T) {
			command, input, output, diagnostics := startDesktopMaintenanceTest(t)
			request := desktopMaintenanceRequest{Protocol: 1, Kind: "start", ID: strings.Repeat("a", 64), Operation: operation, Encryption: "recipient", KeyMaterial: key.String()}
			if operation == "backup" {
				request.DataDir, request.Destination, request.KeyMaterial = data, archive, key.Recipient().String()
			} else {
				request.Source = archive
				if operation == "restore" {
					request.Destination = restored
				}
			}
			writeMaintenanceTestFrame(t, input, request)
			if state := readMaintenanceTestState(t, output); state.Phase != "working" {
				t.Fatal(state)
			}
			state := readMaintenanceTestState(t, output)
			if state.Phase != "finished" || state.Outcome != "succeeded" || state.Summary == nil || state.Summary.Files != 1 || !state.Summary.Authenticated || state.Summary.Published != (operation != "verify") || state.Summary.DatabaseChecked != (operation == "restore") {
				t.Fatal(state)
			}
			if err := command.Wait(); err != nil {
				t.Fatal(err, diagnostics.String())
			}
			encoded, _ := json.Marshal(state)
			if bytes.Contains(encoded, []byte("private.txt")) || bytes.Contains(encoded, []byte("manifest")) || strings.Contains(diagnostics.String(), key.String()) {
				t.Fatal("full inventory or key escaped compact protocol")
			}
		})
	}
	content, err := os.ReadFile(filepath.Join(restored, "private.txt"))
	if err != nil || string(content) != "isolated private contents" {
		t.Fatal("restored content changed", err)
	}
}

func TestDesktopMaintenanceActualCancellationAndInvalidFrames(t *testing.T) {
	for _, mode := range []string{"cancel", "eof", "mismatched", "duplicate", "oversized", "unknown-field"} {
		t.Run(mode, func(t *testing.T) {
			root, data := t.TempDir(), t.TempDir()
			if err := os.WriteFile(filepath.Join(data, "payload"), make([]byte, 16<<20), 0600); err != nil {
				t.Fatal(err)
			}
			command, input, output, diagnostics := startDesktopMaintenanceTest(t)
			request := desktopMaintenanceRequest{Protocol: 1, Kind: "start", ID: strings.Repeat("b", 64), Operation: "backup", Encryption: "legacy", DataDir: data, Destination: filepath.Join(root, "archive")}
			if mode == "oversized" {
				var header [4]byte
				binary.BigEndian.PutUint32(header[:], 2<<20+1)
				input.Write(header[:])
			} else if mode == "unknown-field" {
				writeMaintenanceTestFrame(t, input, map[string]any{"protocol": 1, "id": request.ID, "secret": "untrusted-sentinel"})
			} else {
				writeMaintenanceTestFrame(t, input, request)
				if mode == "eof" {
					input.Close()
				} else {
					control := map[string]any{"protocol": 1, "kind": "cancel", "id": request.ID}
					if mode == "mismatched" {
						control["id"] = strings.Repeat("c", 64)
					}
					if mode == "duplicate" {
						writeMaintenanceTestFrame(t, input, request)
					} else {
						writeMaintenanceTestFrame(t, input, control)
					}
				}
				readMaintenanceTestState(t, output)
				state := readMaintenanceTestState(t, output)
				if (mode == "cancel" || mode == "eof") && state.Outcome != "cancelled" || (mode == "mismatched" || mode == "duplicate") && state.Code != "invalid_control" {
					t.Fatal(state)
				}
			}
			if err := command.Wait(); err == nil {
				t.Fatal("invalid or cancelled request succeeded")
			}
			if strings.Contains(diagnostics.String(), "untrusted-sentinel") {
				t.Fatal("raw request diagnostics escaped")
			}
			entries, err := os.ReadDir(root)
			if err != nil || len(entries) != 0 {
				t.Fatal("cancelled/invalid operation published", entries, err)
			}
		})
	}
}
