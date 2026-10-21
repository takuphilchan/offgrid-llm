package models

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"os"
	"path/filepath"
	"testing"
)

func TestCapabilityEvidenceChangedFileWithoutStatChange(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	path := filepath.Join(dir, "fixture.gguf")
	if err := os.WriteFile(path, []byte("model-A"), 0600); err != nil {
		t.Fatal(err)
	}
	measure := func() string {
		t.Helper()
		file, err := os.Open(path)
		if err != nil {
			t.Fatal(err)
		}
		defer file.Close()
		if _, err = file.Seek(2, io.SeekStart); err != nil {
			t.Fatal(err)
		}
		digest, err := MeasureCapabilityFile(ctx, file)
		if err != nil {
			t.Fatal(err)
		}
		if offset, err := file.Seek(0, io.SeekCurrent); err != nil || offset != 2 {
			t.Fatal("measurement changed caller offset", offset, err)
		}
		return digest
	}
	e := capabilityEvidenceFixture()
	e.Fingerprint.ModelSHA256 = measure()
	expected := sha256.Sum256([]byte("model-A"))
	if e.Fingerprint.ModelSHA256 != hex.EncodeToString(expected[:]) {
		t.Fatal("content digest does not match actual file")
	}
	store := NewCapabilityEvidenceStore(dir)
	if _, err := store.Record(ctx, e); err != nil {
		t.Fatal(err)
	}
	before, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(path, []byte("model-B"), 0600); err != nil {
		t.Fatal(err)
	}
	if err = os.Chtimes(path, before.ModTime(), before.ModTime()); err != nil {
		t.Fatal(err)
	}
	current := e.Fingerprint
	current.ModelSHA256 = measure()
	view, err := NewCapabilityEvidenceStore(dir).Latest(ctx, e.ModelID, e.CheckID, CapabilityEvidenceTarget{Fingerprint: current})
	if err != nil || view.State != "stale" {
		t.Fatalf("changed bytes reused evidence after restart: %+v %v", view, err)
	}
}

func TestCapabilityMeasurementCancellationAndInvalidHandles(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := MeasureCapabilityFile(ctx, nil); !errors.Is(err, context.Canceled) {
		t.Fatal("cancelled measurement did not stop", err)
	}
	if _, err := MeasureCapabilityFile(context.Background(), nil); err == nil {
		t.Fatal("missing handle accepted")
	}
	file, err := os.CreateTemp(t.TempDir(), "empty")
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	if _, err = MeasureCapabilityFile(context.Background(), file); err == nil {
		t.Fatal("empty artifact accepted")
	}
	if err = file.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err = MeasureCapabilityFile(context.Background(), file); err == nil {
		t.Fatal("closed artifact accepted")
	}
	store := NewCapabilityEvidenceStore("")
	if _, err = store.Record(context.Background(), capabilityEvidenceFixture()); err == nil {
		t.Fatal("implicit working-directory storage accepted")
	}
	if _, err = store.Latest(context.Background(), "fixture", "native-tool-v1", CapabilityEvidenceTarget{}); err == nil {
		t.Fatal("implicit working-directory lookup accepted")
	}
}
