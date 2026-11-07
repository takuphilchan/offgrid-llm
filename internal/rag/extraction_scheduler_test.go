package rag

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"
)

func TestExtractionSchedulerBoundsCancellationAndRecovery(t *testing.T) {
	var scheduler extractionScheduler
	release, err := scheduler.acquire(context.Background(), time.Second)
	if err != nil {
		t.Fatal(err)
	}
	defer release()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	results := make(chan error, maxQueuedExtractions)
	for i := 0; i < maxQueuedExtractions; i++ {
		go func() {
			done, err := scheduler.acquire(ctx, time.Second)
			if done != nil {
				done()
			}
			results <- err
		}()
	}
	deadline := time.Now().Add(time.Second)
	for len(scheduler.admitted) != maxQueuedExtractions+1 && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	if len(scheduler.admitted) != maxQueuedExtractions+1 {
		t.Fatal("queue did not fill")
	}
	_, err = scheduler.acquire(context.Background(), time.Second)
	var busy *ParseError
	if !errors.As(err, &busy) || busy.Code != "document_busy" {
		t.Fatal("unbounded admission", err)
	}
	cancel()
	for i := 0; i < maxQueuedExtractions; i++ {
		select {
		case err := <-results:
			if !errors.Is(err, context.Canceled) {
				t.Fatal(err)
			}
		case <-time.After(time.Second):
			t.Fatal("queued cancellation blocked")
		}
	}
	if len(scheduler.admitted) != 1 {
		t.Fatal("cancelled work retained queue slots")
	}
	release()
	release() // Disposal is idempotent, not a double release.
	done, err := scheduler.acquire(context.Background(), time.Second)
	if err != nil {
		t.Fatal("queue poisoned after cancellation", err)
	}
	done()
}

func TestExtractionQueueTimeoutIsOperationalNotInvalidDocument(t *testing.T) {
	var scheduler extractionScheduler
	release, _ := scheduler.acquire(context.Background(), time.Second)
	defer release()
	_, err := scheduler.acquire(context.Background(), time.Millisecond)
	var busy *ParseError
	if !errors.As(err, &busy) || busy.Code != "document_busy" {
		t.Fatal(err)
	}
	if len(scheduler.admitted) != 1 {
		t.Fatal("timeout leaked admission")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := scheduler.acquire(ctx, time.Second); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
}

func TestExpiredExtractionWaitCannotAdmitWork(t *testing.T) {
	var scheduler extractionScheduler
	for i := 0; i < 100; i++ {
		release, err := scheduler.acquire(context.Background(), -time.Second)
		if release != nil {
			release()
			t.Fatal("expired queue wait admitted extraction")
		}
		var busy *ParseError
		if !errors.As(err, &busy) || busy.Code != "document_busy" {
			t.Fatalf("expected operational queue timeout, got %v", err)
		}
		if len(scheduler.active) != 0 || len(scheduler.admitted) != 0 {
			t.Fatal("expired queue wait retained a slot")
		}
	}
	release, err := scheduler.acquire(context.Background(), time.Second)
	if err != nil {
		t.Fatal("expired waits prevented subsequent work", err)
	}
	release()
}

func TestParserAndDurableImportShareExtractionAdmission(t *testing.T) {
	release, err := sharedExtractionScheduler.acquire(context.Background(), time.Second)
	if err != nil {
		t.Fatal(err)
	}
	defer release()
	e := NewEngine(nil, t.TempDir())
	defer e.Close()
	store, _ := e.ImportStore()
	record := acceptedImport(t, store, "alice", "shared-reader")
	for _, kind := range []string{"attachment-reader", "library-import"} {
		t.Run(kind, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Millisecond)
			defer cancel()
			if kind == "attachment-reader" {
				_, err = NewDocumentParser().ParseContext(ctx, []byte("readable text"), "source.txt", ".txt")
			} else {
				_, err = e.RunImport(ctx, "alice", record.ID)
			}
			if !errors.Is(err, context.DeadlineExceeded) {
				t.Fatal("reader bypassed shared queue", err)
			}
		})
	}
	saved, err := store.Import("alice", record.ID)
	if err != nil || saved.State != "failed" || saved.ErrorCode != "document_timeout" || saved.ReviewAvailable {
		t.Fatal(saved, err)
	}
	if len(e.ListDocuments()) != 0 {
		t.Fatal("queued work published")
	}
	release()
	if _, err := NewDocumentParser().ParseContext(context.Background(), []byte("readable text"), "source.txt", ".txt"); err != nil {
		t.Fatal(err)
	}
}

func TestImportQueueCapacityPreservesUnstartedOriginal(t *testing.T) {
	e := NewEngine(nil, t.TempDir())
	defer e.Close()
	if err := e.ingestMu.acquire(context.Background()); err != nil {
		t.Fatal(err)
	}
	defer e.ingestMu.release()
	store, _ := e.ImportStore()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	results := make(chan error, maxQueuedExtractions+1)
	for i := 0; i <= maxQueuedExtractions; i++ {
		r := acceptedImport(t, store, "alice", fmt.Sprintf("queued-%d", i))
		go func() { _, err := e.RunImport(ctx, "alice", r.ID); results <- err }()
	}
	deadline := time.Now().Add(time.Second)
	for {
		e.imports.mu.Lock()
		count := len(e.imports.running)
		e.imports.mu.Unlock()
		if count == maxQueuedExtractions+1 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("imports did not queue")
		}
		time.Sleep(time.Millisecond)
	}
	r := acceptedImport(t, store, "alice", "excess")
	_, err := e.RunImport(context.Background(), "alice", r.ID)
	var busy *ParseError
	if !errors.As(err, &busy) || busy.Code != "document_busy" {
		t.Fatal(err)
	}
	saved, err := store.Import("alice", r.ID)
	if err != nil || saved.State != "accepted" || saved.Attempt != 0 || saved.SHA256 != r.SHA256 {
		t.Fatal("rejected admission changed source", saved, err)
	}
	// The index lock is still held: another Chat attachment must nevertheless
	// be able to read its source, without waiting for embedding/publication.
	readerCtx, stopReader := context.WithTimeout(context.Background(), time.Second)
	defer stopReader()
	if _, err := NewDocumentParser().ParseContext(readerCtx, []byte("Independent attachment"), "note.txt", ".txt"); err != nil {
		t.Fatal("indexing monopolized shared extraction", err)
	}
	cancel()
	for i := 0; i <= maxQueuedExtractions; i++ {
		select {
		case err := <-results:
			if !errors.Is(err, context.Canceled) {
				t.Fatal(err)
			}
		case <-time.After(time.Second):
			t.Fatal("import cancellation blocked")
		}
	}
}
