package models

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"io"
	"os"
	"path/filepath"
	"sync"
	"testing"
)

// Storage/header fixture only; deliberately not qualified inference weights.
func importGGUFFixture(t *testing.T, split bool) []byte {
	t.Helper()
	var b bytes.Buffer
	b.WriteString("GGUF")
	write := func(v any) {
		t.Helper()
		if err := binary.Write(&b, binary.LittleEndian, v); err != nil {
			t.Fatal(err)
		}
	}
	text := func(s string) { write(uint64(len(s))); b.WriteString(s) }
	write(uint32(3))
	write(uint64(1))
	count := uint64(1)
	if split {
		count++
	}
	write(count)
	text("general.architecture")
	write(uint32(8))
	text("qwen2")
	if split {
		text("split.count")
		write(uint32(4))
		write(uint32(2))
	}
	return append(b.Bytes(), make([]byte, (1<<20)-b.Len())...)
}

func TestGGUFImportPreservesFilesAndIdentity(t *testing.T) {
	dir := t.TempDir()
	registry := NewRegistry(dir)
	data := importGGUFFixture(t, false)
	receipt, err := registry.ImportGGUF(context.Background(), "fixture.gguf", int64(len(data)), bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	digest := sha256.Sum256(data)
	if receipt.SHA256 != hex.EncodeToString(digest[:]) || receipt.ModelID != "fixture" || !receipt.InventoryRefreshed {
		t.Fatalf("receipt %+v", receipt)
	}
	stored, err := os.ReadFile(filepath.Join(dir, "fixture.gguf"))
	if err != nil || !bytes.Equal(stored, data) {
		t.Fatal("stored bytes differ", err)
	}
	for _, name := range []string{"fixture.gguf", "FIXTURE.gguf"} {
		if _, err := registry.ImportGGUF(context.Background(), name, int64(len(data)), bytes.NewReader(data)); !errors.Is(err, ErrGGUFImportExists) {
			t.Fatalf("replacement: %v", err)
		}
	}
	if registry.CountLoadedModels() != 0 {
		t.Fatal("import loaded a model")
	}
	entries, _ := os.ReadDir(dir)
	if len(entries) != 1 {
		t.Fatalf("staging leaked: %v", entries)
	}
}

func TestGGUFImportRejectsInvalidAndIncompleteInputs(t *testing.T) {
	data := importGGUFFixture(t, false)
	for _, name := range []string{"../escape.gguf", "folder/model.gguf", "CON.gguf", "model.gguf:ads", "mmproj-model.gguf", "model-00001-of-00002.gguf", "model.bin", "bad\x00.gguf"} {
		t.Run(name, func(t *testing.T) {
			dir := t.TempDir()
			_, err := NewRegistry(dir).ImportGGUF(context.Background(), name, int64(len(data)), bytes.NewReader(data))
			if !errors.Is(err, ErrGGUFImportInvalid) {
				t.Fatalf("accepted %q: %v", name, err)
			}
			entries, _ := os.ReadDir(dir)
			if len(entries) != 0 {
				t.Fatal("invalid input stored")
			}
		})
	}
	for _, kind := range []string{"short", "extra", "magic", "version", "metadata", "renamed-shard", "reader-error", "cancelled"} {
		t.Run(kind, func(t *testing.T) {
			dir := t.TempDir()
			copyData := append([]byte(nil), data...)
			size := int64(len(data))
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			var reader io.Reader
			switch kind {
			case "short":
				copyData = copyData[:len(copyData)-1]
			case "extra":
				copyData = append(copyData, 0)
			case "magic":
				copyData[0] = 'X'
			case "version":
				copyData[4] = 99
			case "metadata":
				binary.LittleEndian.PutUint64(copyData[16:24], 1_000_001)
			case "renamed-shard":
				copyData = importGGUFFixture(t, true)
			case "reader-error":
				reader = io.MultiReader(bytes.NewReader(copyData[:100]), failingImportReader{})
			case "cancelled":
				reader = &cancelImportReader{bytes.NewReader(copyData), cancel}
			}
			if reader == nil {
				reader = bytes.NewReader(copyData)
			}
			if _, err := NewRegistry(dir).ImportGGUF(ctx, "fixture.gguf", size, reader); err == nil {
				t.Fatal("accepted invalid input")
			}
			entries, _ := os.ReadDir(dir)
			if len(entries) != 0 {
				t.Fatalf("failed import left files: %v", entries)
			}
		})
	}
}

type failingImportReader struct{}

func (failingImportReader) Read([]byte) (int, error) { return 0, errors.New("simulated read failure") }

type cancelImportReader struct {
	reader *bytes.Reader
	cancel context.CancelFunc
}

func (r *cancelImportReader) Read(p []byte) (int, error) {
	n, err := r.reader.Read(p)
	if r.reader.Len() == 0 {
		r.cancel()
	}
	return n, err
}

func TestGGUFImportConcurrentPublication(t *testing.T) {
	dir := t.TempDir()
	registry := NewRegistry(dir)
	data := importGGUFFixture(t, false)
	var wg sync.WaitGroup
	outcomes := make(chan error, 2)
	for range 2 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := registry.ImportGGUF(context.Background(), "fixture.gguf", int64(len(data)), bytes.NewReader(data))
			outcomes <- err
		}()
	}
	wg.Wait()
	close(outcomes)
	success := 0
	conflict := 0
	for err := range outcomes {
		if err == nil {
			success++
		} else if errors.Is(err, ErrGGUFImportExists) || errors.Is(err, ErrGGUFImportBusy) {
			conflict++
		} else {
			t.Fatal(err)
		}
	}
	if success != 1 || conflict != 1 {
		t.Fatalf("success=%d conflict=%d", success, conflict)
	}
}

func TestGGUFImportDoesNotShadowNestedModel(t *testing.T) {
	dir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, "saved"), 0700); err != nil {
		t.Fatal(err)
	}
	data := importGGUFFixture(t, false)
	if err := os.WriteFile(filepath.Join(dir, "saved", "fixture.gguf"), data, 0600); err != nil {
		t.Fatal(err)
	}
	registry := NewRegistry(dir)
	if _, err := registry.ImportGGUF(context.Background(), "fixture.gguf", int64(len(data)), bytes.NewReader(data)); !errors.Is(err, ErrGGUFImportExists) {
		t.Fatalf("shadowed model: %v", err)
	}
}
