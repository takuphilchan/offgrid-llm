package rag

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/takuphilchan/offgrid-llm/internal/documentocr"
	"github.com/takuphilchan/offgrid-llm/internal/sessions"
	"github.com/takuphilchan/offgrid-llm/internal/testsupport/tifffixture"
)

func TestOCRCheckpointRealWorkerResumesBothSourceBoundaries(t *testing.T) {
	testOCRCheckpointRealWorker(t, "fra")
}

func TestMixedOCRCheckpointRealWorkerResumesBothSourceBoundaries(t *testing.T) {
	testOCRCheckpointRealWorker(t, "fra+chi_sim")
}

func testOCRCheckpointRealWorker(t *testing.T, selection string) {
	worker, packs := os.Getenv("OFFGRID_TEST_OCR_WORKER"), os.Getenv("OFFGRID_TEST_OCR_PACKS")
	if worker == "" || packs == "" {
		t.Skip("requires explicit worker and pinned packs")
	}
	models := [][]byte{}
	for _, id := range strings.Split(selection, "+") {
		data, err := os.ReadFile(filepath.Join(packs, id+".traineddata"))
		if err != nil {
			t.Fatal(err)
		}
		models = append(models, data)
	}
	model, err := documentocr.Models(selection, models)
	if err != nil {
		t.Fatal(err)
	}
	input, err := os.ReadFile("testdata/documents/ocr/languages/fra.png")
	if err != nil {
		t.Fatal(err)
	}
	pixels, err := png.Decode(bytes.NewReader(input))
	if err != nil {
		t.Fatal(err)
	}
	source := tifffixture.Encode([]image.Image{pixels, pixels}, []uint16{1, 1}, binary.LittleEndian)
	for _, boundary := range []string{"knowledge", "private"} {
		t.Run(boundary, func(t *testing.T) {
			directory := t.TempDir()
			var store documentocr.Checkpoints
			var reopen func() documentocr.Checkpoints
			if boundary == "knowledge" {
				s, err := NewSQLiteStore(directory)
				if err != nil {
					t.Fatal(err)
				}
				defer func() { s.Close() }()
				r, err := s.AcceptImportWithOCRLanguage("owner", "key", "scan.tiff", source, WorkspaceCollection, selection)
				if err != nil {
					t.Fatal(err)
				}
				start := func() documentocr.Checkpoints {
					r, err := s.beginImport("owner", r.ID)
					if err != nil {
						t.Fatal(err)
					}
					lease := importLease{id: r.ID, owner: "owner", collection: r.CollectionID, attempt: r.Attempt}
					if err := s.advanceImport(lease, "queued", "extracting", ""); err != nil {
						t.Fatal(err)
					}
					return s.importOCRCheckpoints(lease)
				}
				store = start()
				reopen = func() documentocr.Checkpoints {
					s.Close()
					s, err = NewSQLiteStore(directory)
					if err != nil {
						t.Fatal(err)
					}
					if err := s.RecoverImports(); err != nil {
						t.Fatal(err)
					}
					progress, err := s.Import("owner", r.ID)
					if err != nil || progress.OCRCompletedPages != 1 {
						t.Fatal("lost progress", progress, err)
					}
					return start()
				}
			} else {
				s := sessions.NewSessionManager(directory).WithAccess(sessions.Access{UserID: "owner"})
				if _, err := s.Create("scan", "model"); err != nil {
					t.Fatal(err)
				}
				a, err := s.CreateAttachmentWithOCRLanguage("scan", "scan.tiff", "image/tiff", source, selection)
				if err != nil {
					t.Fatal(err)
				}
				store = s.AttachmentOCRCheckpoints("scan", a.ID, a.SHA256)
				reopen = func() documentocr.Checkpoints {
					s = sessions.NewSessionManager(directory).WithAccess(sessions.Access{UserID: "owner"})
					return s.AttachmentOCRCheckpoints("scan", a.ID, a.SHA256)
				}
			}
			parser := NewDocumentParser()
			parser.ocrWorker = worker
			if err := parser.UseOCRLanguage(selection, model); err != nil {
				t.Fatal(err)
			}
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			first := store
			parser.UseOCRCheckpoints(documentocr.Checkpoints{Read: first.Read, Write: func(page int, key string, raw []byte) error {
				if err := first.Write(page, key, raw); err != nil {
					return err
				}
				cancel()
				return nil
			}})
			if got, err := parser.ParseContext(ctx, source, "scan.tiff", ".tiff"); got != nil || !errors.Is(err, context.Canceled) {
				t.Fatal("cancelled evidence published", got, err)
			}
			store = reopen()
			hits, writes := 0, 0
			parser.UseOCRCheckpoints(documentocr.Checkpoints{Read: func(page int, key string) ([]byte, error) {
				raw, err := store.Read(page, key)
				if len(raw) > 0 {
					hits++
				}
				return raw, err
			}, Write: func(page int, key string, raw []byte) error { writes++; return store.Write(page, key, raw) }})
			got, err := parser.ParseContext(context.Background(), source, "scan.tiff", ".tiff")
			if err != nil || got == nil || got.Structure == nil || got.Structure.Verify() != nil || len(got.SourceUnits) != 2 || hits != 1 || writes != 1 {
				t.Fatal("resume failed", got, err, hits, writes)
			}
			for _, unit := range got.SourceUnits {
				if !strings.Contains(unit.Text, "42 euros") {
					t.Fatal("source text lost", unit)
				}
			}
			// A subsequent pass must validate retained receipts without executing a
			// worker, even when its configured path cannot be launched.
			parser.ocrWorker = filepath.Join(t.TempDir(), "must-not-launch.exe")
			again, err := parser.ParseContext(context.Background(), source, "scan.tiff", ".tiff")
			if err != nil || again.Structure.ID != got.Structure.ID || hits != 3 || writes != 1 {
				t.Fatal("completed OCR repeated or identity changed", err, hits, writes)
			}
			t.Logf("%s: first page survived cancellation/restart; one remaining page processed; all saved pages then reused", boundary)
		})
	}
}
