package storage

import (
	"context"
	"errors"
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"runtime"
	"sync/atomic"
	"testing"
	"time"

	"filippo.io/age"
)

// Opt in explicitly: this measures disposable fixture storage and uses no real
// workspace. Process RSS is measured separately by the invoking host tool.
func TestBackupFixtureQualification(t *testing.T) {
	if os.Getenv("OFFGRID_TEST_BACKUP_QUALIFICATION") != "1" {
		t.Skip("set OFFGRID_TEST_BACKUP_QUALIFICATION=1 for isolated resource measurements")
	}
	root, data := t.TempDir(), t.TempDir()
	random := rand.New(rand.NewSource(42)) // Repeatable incompressible fixture, no keys.
	buffer := make([]byte, 1<<20)
	for i := 0; i < 64; i++ {
		if _, err := random.Read(buffer); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(data, fmt.Sprintf("fixture-%02d.bin", i)), buffer, 0600); err != nil {
			t.Fatal(err)
		}
	}
	buffer = nil
	runtime.GC()
	var peak atomic.Uint64
	sample := func() {
		var memory runtime.MemStats
		runtime.ReadMemStats(&memory)
		for previous := peak.Load(); memory.HeapAlloc > previous && !peak.CompareAndSwap(previous, memory.HeapAlloc); previous = peak.Load() {
		}
	}
	sample()
	baseline := peak.Load()
	stop, stopped := make(chan struct{}), make(chan struct{})
	go func() {
		defer close(stopped)
		ticker := time.NewTicker(5 * time.Millisecond)
		defer ticker.Stop()
		for {
			select {
			case <-stop:
				return
			case <-ticker.C:
				sample()
			}
		}
	}()
	defer func() { close(stop); <-stopped }()
	key := backupTestKey(t)
	archive := filepath.Join(root, "fixture.age")
	start := time.Now()
	manifest, err := BackupWorkspaceWithOptions(context.Background(), data, archive, "qualification-v1", BackupWriteOptions{Recipients: []age.Recipient{key.Recipient()}})
	if err != nil {
		t.Fatal(err)
	}
	backupElapsed := time.Since(start)
	read := BackupReadOptions{Identities: []age.Identity{key}, RequireEncryption: true}
	start = time.Now()
	verified, err := VerifyBackupWithOptions(context.Background(), archive, read)
	if err != nil || verified.Bytes != manifest.Bytes || len(verified.Files) != len(manifest.Files) {
		t.Fatal("fixture verification changed inventory", err)
	}
	verifyElapsed := time.Since(start)
	start = time.Now()
	if _, err := RestoreWorkspaceWithOptions(context.Background(), archive, filepath.Join(root, "restored"), "qualification-v1", read); err != nil {
		t.Fatal(err)
	}
	restoreElapsed := time.Since(start)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() { _, err := VerifyBackupWithOptions(ctx, archive, read); done <- err }()
	// This is a measured asynchronous cancellation during real archive IO, not a
	// unit-test timing guarantee. If the fixture finishes too fast, report that.
	time.Sleep(5 * time.Millisecond)
	start = time.Now()
	cancel()
	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Fatal("fixture completed before cancellation or lost cancellation", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("fixture verification did not acknowledge cancellation")
	}
	cancelElapsed := time.Since(start)
	sample()
	info, err := os.Stat(archive)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("files=%d source_bytes=%d archive_bytes=%d backup=%s verify=%s restore=%s cancel_ack=%s heap_baseline_bytes=%d sampled_peak_heap_bytes=%d", len(manifest.Files), manifest.Bytes, info.Size(), backupElapsed, verifyElapsed, restoreElapsed, cancelElapsed, baseline, peak.Load())
}
