package p2p

import (
	"encoding/binary"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestSecureTransferKeyPersists(t *testing.T) {
	dir := t.TempDir()
	first, err := NewSecureTransferManager(DefaultSecureTransferConfig(dir))
	if err != nil {
		t.Fatal(err)
	}
	second, err := NewSecureTransferManager(DefaultSecureTransferConfig(dir))
	if err != nil {
		t.Fatal(err)
	}
	if first.GetEncryptionKeyHex() != second.GetEncryptionKeyHex() {
		t.Fatal("generated encryption key did not persist")
	}
}

func TestSetEncryptionKeyPersists(t *testing.T) {
	dir := t.TempDir()
	manager, err := NewSecureTransferManager(DefaultSecureTransferConfig(dir))
	if err != nil {
		t.Fatal(err)
	}
	key := "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
	if err := manager.SetEncryptionKey(key); err != nil {
		t.Fatal(err)
	}
	reloaded, err := NewSecureTransferManager(DefaultSecureTransferConfig(dir))
	if err != nil {
		t.Fatal(err)
	}
	if reloaded.GetEncryptionKeyHex() != key {
		t.Fatal("SetEncryptionKey was not persisted")
	}
}

func TestPauseTransferDoesNotDeadlock(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "model.gguf")
	if err := os.WriteFile(source, []byte("model"), 0600); err != nil {
		t.Fatal(err)
	}
	manager, err := NewSecureTransferManager(DefaultSecureTransferConfig(dir))
	if err != nil {
		t.Fatal(err)
	}
	transfer, err := manager.CreateResumableTransfer(source)
	if err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() { done <- manager.PauseTransfer(transfer.ID) }()
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("PauseTransfer deadlocked")
	}
}

func TestWriteChunkCompletesWithoutDeadlock(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "source.gguf")
	content := []byte("model")
	if err := os.WriteFile(source, content, 0600); err != nil {
		t.Fatal(err)
	}
	manager, err := NewSecureTransferManager(DefaultSecureTransferConfig(dir))
	if err != nil {
		t.Fatal(err)
	}
	transfer, err := manager.CreateResumableTransfer(source)
	if err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() { done <- manager.WriteChunk(transfer.ID, 0, content, true) }()
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("WriteChunk deadlocked")
	}
	got, err := os.ReadFile(filepath.Join(dir, "models", "source.gguf"))
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(content) {
		t.Fatalf("completed content = %q, want %q", got, content)
	}
}

func TestDecryptFileRejectsInvalidChunkLength(t *testing.T) {
	dir := t.TempDir()
	manager, err := NewSecureTransferManager(DefaultSecureTransferConfig(dir))
	if err != nil {
		t.Fatal(err)
	}
	for _, length := range []int32{-1, 64 * 1024 * 1024} {
		input := filepath.Join(dir, "bad.enc")
		file, err := os.Create(input)
		if err != nil {
			t.Fatal(err)
		}
		if err := binary.Write(file, binary.BigEndian, int32(1024)); err != nil {
			t.Fatal(err)
		}
		if err := binary.Write(file, binary.BigEndian, length); err != nil {
			t.Fatal(err)
		}
		if err := file.Close(); err != nil {
			t.Fatal(err)
		}
		if err := manager.DecryptFile(input, filepath.Join(dir, "out")); err == nil {
			t.Fatalf("expected invalid length %d to be rejected", length)
		}
	}
}
