//go:build linux

package models

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"testing"
)

func TestCapabilityMeasurementCacheDetectsRestoredMetadataAndReplacement(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "model")
	if err := os.WriteFile(path, []byte("model-A"), 0600); err != nil {
		t.Fatal(err)
	}
	m := &CapabilityMeasurements{}
	measure := func() string {
		t.Helper()
		f, err := os.Open(path)
		if err != nil {
			t.Fatal(err)
		}
		defer f.Close()
		got, err := m.File(ctx, f)
		if err != nil {
			t.Fatal(err)
		}
		return got
	}
	a := measure()
	if again := measure(); again != a {
		t.Fatal("unchanged bytes changed identity")
	}
	before, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("model-B"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(path, before.ModTime(), before.ModTime()); err != nil {
		t.Fatal(err)
	}
	b := measure()
	digest := sha256.Sum256([]byte("model-B"))
	if a == b || b != hex.EncodeToString(digest[:]) {
		t.Fatal("equal-size, restored-mtime write reused stale bytes")
	}
	replacement := path + ".replacement"
	if err := os.WriteFile(replacement, []byte("model-C"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(replacement, before.ModTime(), before.ModTime()); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(replacement, path); err != nil {
		t.Fatal(err)
	}
	if c := measure(); c == a || c == b {
		t.Fatal("replacement inherited cached identity")
	}
}

func TestCapabilityMeasurementCacheConcurrentCancellationAndBundleParity(t *testing.T) {
	path := filepath.Join(t.TempDir(), "runtime")
	if err := os.WriteFile(path, []byte("runtime-bytes"), 0600); err != nil {
		t.Fatal(err)
	}
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	m := &CapabilityMeasurements{}
	ctx := context.Background()
	want, err := MeasureCapabilityBundle(ctx, map[string]*os.File{"executable": f})
	if err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	for i := 0; i < 16; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			got, err := m.Bundle(ctx, map[string]*os.File{"executable": f})
			if err != nil || got != want {
				t.Errorf("cache changed bundle identity: %s %v", got, err)
			}
		}()
	}
	wg.Wait()
	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	if _, err := m.File(cancelled, f); !errors.Is(err, context.Canceled) {
		t.Fatal("cache ignored cancellation", err)
	}
	var uncached *CapabilityMeasurements
	if got, err := uncached.Bundle(ctx, map[string]*os.File{"executable": f}); err != nil || got != want {
		t.Fatal(got, err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := m.File(ctx, f); err == nil {
		t.Fatal("closed handle reused cache")
	}
}
