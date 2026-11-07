package rag

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/takuphilchan/offgrid-llm/internal/inference"
)

// Opt-in real reader + embedding + durable publication coverage. Each sample
// uses a fresh index, so a warm measurement cannot pass through deduplication.
// No installed workspace is opened and no model/runtime is downloaded.
func TestRealDocumentImportSchedulingAndRecovery(t *testing.T) {
	model, runtime := os.Getenv("OFFGRID_TEST_EMBEDDING_MODEL"), os.Getenv("OFFGRID_LLAMA_SERVER_PATH")
	if model == "" || runtime == "" || os.Getenv("OFFGRID_TEST_PACKAGED_DOCUMENTS") != "1" {
		t.Skip("requires explicit local embedding model/runtime and an app-local PDF worker")
	}
	if NewDocumentParser().pdfWorker == "" {
		t.Fatal("compile this test beside the packaged PDF worker")
	}
	cache := t.TempDir()
	for _, key := range []string{"LOCALAPPDATA", "XDG_CACHE_HOME"} {
		t.Setenv(key, cache)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	worker := inference.NewEmbeddingEngine(t.TempDir())
	defer worker.Unload()
	data, err := os.ReadFile(filepath.Join("testdata", "documents", "corpus", "compressed.pdf"))
	if err != nil {
		t.Fatal(err)
	}
	var identity string
	for _, phase := range []string{"cold", "warm", "cancel-and-retry"} {
		t.Run(phase, func(t *testing.T) {
			e := NewEngine(worker, t.TempDir())
			defer e.Close()
			e.SetModelResolver(func(string) (string, error) { return model, nil })
			started := time.Now()
			if err := e.Enable(ctx, "qualification-embedding"); err != nil {
				t.Fatal(err)
			}
			loadTime := time.Since(started)
			store, err := e.ImportStore()
			if err != nil {
				t.Fatal(err)
			}
			before, found, err := store.GetIndexMetadata()
			if err != nil || !found || before.EmbeddingIdentity == "" {
				t.Fatal("missing real embedding identity", err)
			}
			if identity != "" && before.EmbeddingIdentity != identity {
				t.Fatal("warm import changed the embedding identity")
			}
			identity = before.EmbeddingIdentity
			r, err := store.AcceptImport("alice", phase, "compressed.pdf", data)
			if err != nil {
				t.Fatal(err)
			}
			if phase == "cancel-and-retry" {
				cancelImportBeforePublication(t, ctx, e, store, r)
			}
			started = time.Now()
			doc, err := e.RunImport(ctx, "alice", r.ID)
			importTime := time.Since(started)
			if err != nil {
				t.Fatal(err)
			}
			saved, err := store.Import("alice", r.ID)
			if err != nil || saved.State != "ready" || saved.DocumentID != doc.ID || saved.SHA256 != r.SHA256 {
				t.Fatalf("durable publication mismatch: %+v %v", saved, err)
			}
			original, err := e.ForActor("alice").Original(doc.ID)
			if err != nil || !bytes.Equal(original.Bytes, data) || original.SHA256 != r.SHA256 {
				t.Fatal("import changed the retained original", err)
			}
			if doc.IndexStatus != "ready" || doc.ChunkCount < 1 || !strings.Contains(doc.RawContent, "The library closes at 17:00.") {
				t.Fatal("missing indexed PDF text")
			}
			result, err := e.ForActor("alice").Search(ctx, "When does the library close?", DefaultSearchOptions())
			if err != nil || result == nil || len(result.Results) == 0 || result.Results[0].DocumentID != doc.ID {
				t.Fatalf("real indexed retrieval failed: %+v %v", result, err)
			}
			after, _, err := store.GetIndexMetadata()
			if err != nil || after.EmbeddingIdentity != identity || after.EmbeddingModel != before.EmbeddingModel || after.EmbeddingDim != before.EmbeddingDim {
				t.Fatal("import changed model/index identity", err)
			}
			if len(e.ListDocuments()) != 1 {
				t.Fatal("retry produced duplicate publications")
			}
			t.Logf("%s load=%s import=%s chunks=%d dimensions=%d", phase, loadTime, importTime, doc.ChunkCount, after.EmbeddingDim)
		})
	}
}

func cancelImportBeforePublication(t *testing.T, ctx context.Context, e *Engine, store *SQLiteStore, r *ImportRecord) {
	t.Helper()
	if err := e.ingestMu.acquire(ctx); err != nil {
		t.Fatal(err)
	}
	var once sync.Once
	release := func() { once.Do(e.ingestMu.release) }
	defer release()
	runCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	done := make(chan error, 1)
	go func() { _, err := e.RunImport(runCtx, "alice", r.ID); done <- err }()
	deadline := time.NewTimer(30 * time.Second)
	defer deadline.Stop()
	tick := time.NewTicker(5 * time.Millisecond)
	defer tick.Stop()
	for {
		saved, err := store.Import("alice", r.ID)
		if err != nil {
			t.Fatal(err)
		}
		if saved.State == "indexing" {
			break
		}
		select {
		case err := <-done:
			t.Fatalf("import ended before indexing: %v", err)
		case <-deadline.C:
			t.Fatal("import did not reach indexing")
		case <-tick.C:
		}
	}
	// An import awaiting index publication must not monopolize the reader.
	readCtx, stopRead := context.WithTimeout(ctx, time.Second)
	defer stopRead()
	if _, err := NewDocumentParser().ParseContext(readCtx, []byte("Independent Chat attachment"), "note.txt", ".txt"); err != nil {
		t.Fatal("indexing blocked a separate source reader", err)
	}
	started := time.Now()
	if err := e.StopImport("alice", r.ID, false); err != nil {
		t.Fatal(err)
	}
	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Fatal("cancellation was not preserved", err)
		}
	case <-time.After(time.Second):
		t.Fatal("cancellation waited for the index lock")
	}
	saved, err := store.Import("alice", r.ID)
	if err != nil || saved.State != "cancelled" || saved.SHA256 != r.SHA256 || len(e.ListDocuments()) != 0 {
		t.Fatal("cancelled import published or lost its source", saved, err)
	}
	t.Logf("index-wait cancellation=%s; separate Chat source remained readable", time.Since(started))
}
