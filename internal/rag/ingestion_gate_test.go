package rag

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestCancelledIngestionDoesNotWaitForAnotherIndexWriter(t *testing.T) {
	for _, kind := range []string{"import", "text", "reindex"} {
		t.Run(kind, func(t *testing.T) {
			e := NewEngine(nil, t.TempDir())
			defer e.Close()
			if err := e.ingestMu.acquire(context.Background()); err != nil {
				t.Fatal(err)
			}
			defer e.ingestMu.release()
			s, _ := e.ImportStore()
			r := acceptedImport(t, s, "alice", kind)
			ctx, cancel := context.WithTimeout(context.Background(), 40*time.Millisecond)
			defer cancel()
			done := make(chan error, 1)
			go func() {
				var err error
				switch kind {
				case "import":
					_, err = e.RunImport(ctx, "alice", r.ID)
				case "text":
					_, err = e.IngestText(ctx, "file", "source text with enough content", nil)
				case "reindex":
					_, err = e.ReindexDocument(ctx, "existing-document")
				}
				done <- err
			}()
			select {
			case err := <-done:
				if !errors.Is(err, context.DeadlineExceeded) {
					t.Fatal(err)
				}
			case <-time.After(2 * time.Second):
				t.Fatal("cancelled request is still waiting for another index writer")
			}
			if kind == "import" {
				saved, err := s.Import("alice", r.ID)
				if err != nil || saved.State != "failed" || saved.ErrorCode != "document_timeout" {
					t.Fatalf("missing durable timeout: %+v %v", saved, err)
				}
			}
			if len(e.ListDocuments()) != 0 {
				t.Fatal("cancelled work published")
			}
		})
	}
}
