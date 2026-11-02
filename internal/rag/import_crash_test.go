package rag

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"testing"
)

// A child exits without closing the SQLite connection at each durable boundary.
// Only disposable test directories are used; no actual model is involved.
func TestImportCrashChild(t *testing.T) {
	if os.Getenv("OFFGRID_IMPORT_CRASH_FIXTURE") != "1" {
		return
	}
	s, err := NewSQLiteStore(os.Getenv("OFFGRID_IMPORT_CRASH_DIR"))
	if err != nil {
		t.Fatal(err)
	}
	r := acceptedImport(t, s, "alice", "crash-key")
	phase := os.Getenv("OFFGRID_IMPORT_CRASH_PHASE")
	if phase == "accepted" {
		os.Exit(23)
	}
	active, err := s.beginImport("alice", r.ID)
	if err != nil {
		t.Fatal(err)
	}
	lease := importLease{id: r.ID, owner: "alice", attempt: active.Attempt}
	if phase == "queued" {
		os.Exit(23)
	}
	if err = s.advanceImport(lease, "queued", "extracting", ""); err != nil {
		t.Fatal(err)
	}
	if phase == "extracting" {
		os.Exit(23)
	}
	data, err := s.importBytes(lease, r)
	if err != nil {
		t.Fatal(err)
	}
	if err = s.advanceImport(lease, "extracting", "indexing", ""); err != nil {
		t.Fatal(err)
	}
	if phase == "indexing" {
		os.Exit(23)
	}
	doc := originalFixture(data)
	doc.original.importRef = &lease
	chunk, index := publicationFixture(doc)
	if err = s.PublishDocument(doc, []*Chunk{chunk}, [][]float32{{1, 0}}, index, false); err != nil {
		t.Fatal(err)
	}
	os.Exit(23)
}

func TestImportCrashRecoveryAtEveryDurableBoundary(t *testing.T) {
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	for _, phase := range []string{"accepted", "queued", "extracting", "indexing", "published"} {
		t.Run(phase, func(t *testing.T) {
			dir := t.TempDir()
			command := exec.Command(executable, "-test.run=^TestImportCrashChild$")
			command.Env = append(os.Environ(), "OFFGRID_IMPORT_CRASH_FIXTURE=1", "OFFGRID_IMPORT_CRASH_DIR="+dir, "OFFGRID_IMPORT_CRASH_PHASE="+phase)
			output, err := command.CombinedOutput()
			var exited *exec.ExitError
			if !errors.As(err, &exited) || exited.ExitCode() != 23 {
				t.Fatalf("crash fixture failed: %s %v", output, err)
			}
			s, err := NewSQLiteStore(dir)
			if err != nil {
				t.Fatal(err)
			}
			defer s.Close()
			if err = s.RecoverImports(); err != nil {
				t.Fatal(err)
			}
			records, err := s.Imports("alice")
			if err != nil || len(records) != 1 {
				t.Fatal("lost import", err)
			}
			r := records[0]
			if phase == "published" {
				if r.State != "ready" {
					t.Fatal("committed receipt lost")
				}
				if _, err = s.original(r.DocumentID); err != nil {
					t.Fatal(err)
				}
			} else {
				want := "interrupted"
				if phase == "accepted" {
					want = "accepted"
				}
				if r.State != want {
					t.Fatalf("got %s want %s", r.State, want)
				}
				if docs, _ := s.ListDocuments(); len(docs) != 0 {
					t.Fatal("ghost-ready document")
				}
				doc, _ := indexingImport(t, s, r, "alice")
				chunk, index := publicationFixture(doc)
				if err = s.PublishDocument(doc, []*Chunk{chunk}, [][]float32{{1, 0}}, index, false); err != nil {
					t.Fatal(err)
				}
			}
			again := acceptedImport(t, s, "alice", "crash-key")
			if again.ID != r.ID || again.State != "ready" {
				t.Fatal("recovered import duplicated")
			}
			if docs, _ := s.ListDocuments(); len(docs) != 1 {
				t.Fatal("publication count differs")
			}
		})
	}
}

func TestImportAcceptanceFullDiskAndMissingSource(t *testing.T) {
	s, err := NewSQLiteStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	r := acceptedImport(t, s, "alice", "retained")
	var pages int
	if err = s.db.QueryRow(`PRAGMA page_count`).Scan(&pages); err != nil {
		t.Fatal(err)
	}
	if _, err = s.db.Exec(fmt.Sprintf("PRAGMA max_page_count=%d", pages)); err != nil {
		t.Fatal(err)
	}
	if _, err = s.AcceptImport("alice", "full", "large.txt", make([]byte, 1<<20)); err == nil {
		t.Fatal("full disk accepted an import")
	}
	items, err := s.Imports("alice")
	if err != nil || len(items) != 1 {
		t.Fatal("failed acceptance left a record", err)
	}
	if _, err = s.db.Exec(`UPDATE document_imports SET content=NULL WHERE id=?`, r.ID); err != nil {
		t.Fatal(err)
	}
	active, err := s.beginImport("alice", r.ID)
	if err != nil {
		t.Fatal(err)
	}
	lease := importLease{id: r.ID, owner: "alice", attempt: active.Attempt}
	if err = s.advanceImport(lease, "queued", "extracting", ""); err != nil {
		t.Fatal(err)
	}
	if _, err = s.importBytes(lease, r); err == nil {
		t.Fatal("missing original fabricated")
	}
}
