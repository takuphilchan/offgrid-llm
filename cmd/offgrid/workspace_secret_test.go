package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"filippo.io/age"
	"github.com/charmbracelet/x/term"
)

func TestWorkspaceSecretTerminalHelper(t *testing.T) {
	mode := os.Getenv("OFFGRID_WORKSPACE_SECRET_PTY")
	if mode == "" {
		return
	}
	before, err := term.GetState(os.Stdin.Fd())
	if err != nil {
		t.Fatal("fixture terminal unavailable", err)
	}
	ctx := context.Background()
	if mode == "cancel" {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, 250*time.Millisecond)
		defer cancel()
	}
	secret, err := readWorkspaceTerminalSecret(ctx, os.Stdin, os.Stdout, true)
	defer clear(secret)
	if mode == "confirm" {
		if err != nil || string(secret) != "isolated café 日本語 😀" {
			t.Fatal("protected confirmation failed", err)
		}
	} else if err == nil {
		t.Fatal("invalid/cancelled input accepted")
	}
	after, stateErr := term.GetState(os.Stdin.Fd())
	if stateErr != nil || !reflect.DeepEqual(before, after) {
		t.Fatal("terminal mode was not restored", stateErr)
	}
	fmt.Print("PROTECTED_INPUT_DONE\n")
}

func TestWorkspaceSecretBoundsAndWhitespace(t *testing.T) {
	for _, secret := range []string{" phrase 日本語 \n", strings.Repeat("x", 4096) + "\r\n", "no newline"} {
		got, err := readWorkspaceStdinSecret(context.Background(), strings.NewReader(secret))
		want := strings.TrimSuffix(strings.TrimSuffix(secret, "\n"), "\r")
		if err != nil || string(got) != want {
			t.Fatal("secret whitespace changed", err)
		}
		clear(got)
	}
	for _, secret := range []string{"", "\n", "a\nb", "a\r", "a\x00b", "\xff", strings.Repeat("x", 4097), strings.Repeat("x", 4096) + "\n\n"} {
		got, err := readWorkspaceStdinSecret(context.Background(), strings.NewReader(secret))
		if err == nil || got != nil {
			t.Fatal("invalid secret accepted")
		}
	}
	for _, test := range []struct {
		input string
		want  string
		valid bool
	}{
		{"café\x7f日\n", "caf日", true}, {"abc\r", "abc", true}, {"abc\x03", "", false}, {strings.Repeat("x", 4097), "", false},
	} {
		reader := strings.NewReader(test.input)
		next := func() ([]byte, error) { b, err := reader.ReadByte(); return []byte{b}, err }
		got, err := readWorkspaceSecretLine(context.Background(), next)
		if (err == nil) != test.valid || string(got) != test.want {
			t.Fatal("bounded terminal editing failed", err)
		}
		clear(got)
	}
}

func TestWorkspaceRedirectedSecretCancellation(t *testing.T) {
	read, write, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer read.Close()
	defer write.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 150*time.Millisecond)
	defer cancel()
	started := time.Now()
	secret, err := readWorkspaceStdinSecret(ctx, read)
	if !errors.Is(err, context.DeadlineExceeded) || secret != nil || time.Since(started) > 2*time.Second {
		t.Fatal("redirected secret did not cancel promptly", err)
	}
	// Cancellation did not close or steal later input from the caller's stream.
	go func() { _, _ = write.Write([]byte("next\n")); _ = write.Close() }()
	got, err := io.ReadAll(read)
	if err != nil || string(got) != "next\n" {
		t.Fatal("input ownership lost", err)
	}
}

func TestWorkspaceKeyFilesAreBoundedAndRedacted(t *testing.T) {
	key, _ := age.GenerateX25519Identity()
	root := t.TempDir()
	file := filepath.Join(root, "key")
	for _, content := range []string{"", "# no selected key\n", "AGE-SECRET-KEY-1private-sentinel", strings.Repeat("x", maxWorkspaceKeyFile+1), strings.Repeat(key.String()+"\n", 33)} {
		if err := os.WriteFile(file, []byte(content), 0600); err != nil {
			t.Fatal(err)
		}
		_, _, err := (workspaceKeyFlags{identities: []string{file}}).options(context.Background(), workspaceTestStreams(), false)
		if err == nil || strings.Contains(err.Error(), "private-sentinel") || strings.Contains(err.Error(), key.String()) {
			t.Fatalf("unsafe identity handling: %v", err)
		}
	}
	if _, err := readWorkspaceKeyFile(root); err == nil {
		t.Fatal("directory accepted as key")
	}
	// Native keys may be grouped with comments; SSH/plugins never execute.
	if err := os.WriteFile(file, []byte("# fixture\n"+key.String()+"\n"), 0600); err != nil {
		t.Fatal(err)
	}
	_, read, err := (workspaceKeyFlags{identities: []string{file}}).options(context.Background(), workspaceTestStreams(), false)
	if err != nil || len(read.Identities) != 1 || !read.RequireEncryption {
		t.Fatal("valid identity file rejected", err)
	}
	for _, content := range []string{"ssh-ed25519 private-sentinel", "AGE-PLUGIN-EXAMPLE-1private-sentinel"} {
		if err := os.WriteFile(file, []byte(content), 0600); err != nil {
			t.Fatal(err)
		}
		_, _, err := (workspaceKeyFlags{identities: []string{file}}).options(context.Background(), workspaceTestStreams(), false)
		if err == nil || strings.Contains(err.Error(), "private-sentinel") {
			t.Fatal("unsupported identity accepted or echoed", err)
		}
	}
}

func TestWorkspaceEmptyRecipientFileCannotCreatePlaintext(t *testing.T) {
	parent := t.TempDir()
	source := filepath.Join(parent, "source")
	if err := os.Mkdir(source, 0700); err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(parent, "recipients")
	if err := os.WriteFile(file, []byte("# no keys\n"), 0600); err != nil {
		t.Fatal(err)
	}
	archive := filepath.Join(parent, "archive.age")
	err := runWorkspaceCommandWithIO(context.Background(), []string{"backup", "--data-dir", source, "--output", archive, "--recipients-file", file}, workspaceTestStreams())
	if err == nil {
		t.Fatal("empty recipient file accepted")
	}
	if _, err = os.Stat(archive); !os.IsNotExist(err) {
		t.Fatal("archive was published", err)
	}
	entries, _ := os.ReadDir(source)
	if len(entries) != 0 {
		t.Fatal("invalid encryption input touched source")
	}
}

func TestWorkspaceTerminalConfirmationRequestedOnlyForWrites(t *testing.T) {
	for _, writing := range []bool{false, true} {
		streams := workspaceTestStreams()
		called := false
		secret := []byte("private confirmation fixture")
		streams.terminalSecret = func(_ context.Context, _ io.Writer, confirm bool) ([]byte, error) {
			called = true
			if confirm != writing {
				t.Fatal("incorrect confirmation policy")
			}
			return secret, nil
		}
		_, _, err := (workspaceKeyFlags{passphrase: true}).options(context.Background(), streams, writing)
		if err != nil || !called || !bytes.Equal(secret, make([]byte, len(secret))) {
			t.Fatal("terminal acquisition or retained input cleanup failed", err)
		}
	}
}
