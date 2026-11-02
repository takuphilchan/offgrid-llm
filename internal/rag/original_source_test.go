package rag

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"
)

func originalFixture(data []byte) *Document {
	h := sha256.Sum256(data)
	return &Document{ID: GenerateDocumentID(data), Name: "source.pdf", RawContent: "identical extracted text", IndexStatus: "ready", CreatedAt: time.Now(), UpdatedAt: time.Now(), original: &OriginalSource{Name: "source.pdf", MediaType: "application/pdf", SHA256: hex.EncodeToString(h[:]), Bytes: data}}
}

func TestOriginalSourceAtomicPersistenceAndDeletion(t *testing.T) {
	dir := t.TempDir()
	s, err := NewSQLiteStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	a, b := originalFixture([]byte("first original binary")), originalFixture([]byte("second original binary"))
	for _, d := range []*Document{a, b} {
		if err = s.AddDocumentWithChunks(d, nil, nil); err != nil {
			t.Fatal(err)
		}
	}
	if a.ID == b.ID {
		t.Fatal("distinct originals collapsed on extracted text")
	}
	if err = s.AddDocument(a); err == nil {
		t.Fatal("legacy replacement may erase original")
	}
	s.Close()
	s, err = NewSQLiteStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	for _, d := range []*Document{a, b} {
		got, err := s.original(d.ID)
		if err != nil || !bytes.Equal(got.Bytes, d.original.Bytes) {
			t.Fatal("original lost", err)
		}
		saved, err := s.GetDocument(d.ID)
		if err != nil || !saved.OriginalRetained || !saved.SourceRetained {
			t.Fatal("source states lost", err)
		}
		encoded, _ := json.Marshal(saved)
		if bytes.Contains(encoded, d.original.Bytes) {
			t.Fatal("original leaked in metadata")
		}
		if err = s.ReplaceDocumentWithChunks(saved, nil, nil); err != nil {
			t.Fatal(err)
		}
		if _, err = s.original(d.ID); err != nil {
			t.Fatal("reindex deleted original", err)
		}
	}
	if err = s.AddDocumentWithChunks(a, nil, nil); err == nil {
		t.Fatal("collision replaced original")
	}
	if err = s.DeleteDocument(a.ID); err != nil {
		t.Fatal(err)
	}
	if _, err = s.original(a.ID); !errors.Is(err, ErrOriginalUnavailable) {
		t.Fatal("deleted original survived", err)
	}
	if _, err = s.original(b.ID); err != nil {
		t.Fatal("unrelated source deleted", err)
	}
}

func TestOriginalSourceRollbackAndLegacy(t *testing.T) {
	s, err := NewSQLiteStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	legacy := originalFixture([]byte("legacy"))
	legacy.original = nil
	if err = s.AddDocumentWithChunks(legacy, nil, nil); err != nil {
		t.Fatal(err)
	}
	if _, err = s.original(legacy.ID); !errors.Is(err, ErrOriginalUnavailable) {
		t.Fatal("fabricated legacy original", err)
	}
	broken := originalFixture([]byte("must rollback"))
	broken.original.SHA256 = "incorrect"
	if err = s.AddDocumentWithChunks(broken, nil, nil); err == nil {
		t.Fatal("digest mismatch accepted")
	}
	if d, err := s.GetDocument(broken.ID); err != nil || d != nil {
		t.Fatal("failed source created document", err)
	}
	// A genuine SQLite page ceiling exercises SQLITE_FULL without filling disk.
	s.db.SetMaxOpenConns(1)
	var pages int
	if err = s.db.QueryRow("PRAGMA page_count").Scan(&pages); err != nil {
		t.Fatal(err)
	}
	if _, err = s.db.Exec(fmt.Sprintf("PRAGMA max_page_count=%d", pages)); err != nil {
		t.Fatal(err)
	}
	large := originalFixture(bytes.Repeat([]byte("private"), 100000))
	if err = s.AddDocumentWithChunks(large, nil, nil); err == nil {
		t.Fatal("full database accepted source")
	}
	if d, err := s.GetDocument(large.ID); err != nil || d != nil {
		t.Fatal("full disk left ghost document", err)
	}
	if _, err = s.original(large.ID); !errors.Is(err, ErrOriginalUnavailable) {
		t.Fatal("full disk left source", err)
	}
	if d, err := s.GetDocument(legacy.ID); err != nil || d.RawContent != legacy.RawContent {
		t.Fatal("legacy data lost", err)
	}
}

func TestOriginalSourceRejectsTamperedAndFutureRecords(t *testing.T) {
	for _, sql := range []string{"UPDATE document_originals SET schema_version=2", "UPDATE document_originals SET byte_size=999", "UPDATE document_originals SET source_sha256='wrong'", "UPDATE document_originals SET content=X'00'"} {
		t.Run(sql, func(t *testing.T) {
			s, err := NewSQLiteStore(t.TempDir())
			if err != nil {
				t.Fatal(err)
			}
			defer s.Close()
			d := originalFixture([]byte("private original"))
			if err = s.AddDocumentWithChunks(d, nil, nil); err != nil {
				t.Fatal(err)
			}
			if _, err = s.db.Exec(sql); err != nil {
				t.Fatal(err)
			}
			if _, err = s.original(d.ID); !errors.Is(err, ErrOriginalUnavailable) {
				t.Fatal("invalid source delivered", err)
			}
		})
	}
}

func TestOriginalExtractionMustMatchBytes(t *testing.T) {
	e := NewEngine(nil, t.TempDir())
	defer e.Close()
	parsed, err := NewDocumentParser().Parse([]byte("This is the original source."), "test.txt", ".txt")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = e.IngestOriginal(context.Background(), "test.txt", []byte("Different original bytes."), parsed, nil); err == nil {
		t.Fatal("wrong original accepted")
	}
}

func TestOriginalQuotaSerializesConcurrentPublication(t *testing.T) {
	s, err := NewSQLiteStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	s.originalQuota = 30
	var wg sync.WaitGroup
	results := make(chan error, 2)
	for _, data := range []string{"first-original-bytes", "second-original-bytes"} {
		wg.Add(1)
		go func(data string) {
			defer wg.Done()
			results <- s.AddDocumentWithChunks(originalFixture([]byte(data)), nil, nil)
		}(data)
	}
	wg.Wait()
	close(results)
	accepted, rejected := 0, 0
	for err := range results {
		if err == nil {
			accepted++
			continue
		}
		var problem *ParseError
		if !errors.As(err, &problem) || problem.Code != "document_storage_limit" {
			t.Fatal(err)
		}
		rejected++
	}
	if accepted != 1 || rejected != 1 {
		t.Fatal("quota oversubscribed", accepted, rejected)
	}
	docs, err := s.ListDocuments()
	if err != nil || len(docs) != 1 {
		t.Fatal("rejected source published", err)
	}
}

func TestOriginalSchemaMigrationPreservesLegacyText(t *testing.T) {
	dir := t.TempDir()
	s, err := NewSQLiteStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	doc := originalFixture([]byte("legacy original not retained"))
	doc.original = nil
	if err = s.AddDocumentWithChunks(doc, nil, nil); err != nil {
		t.Fatal(err)
	}
	if _, err = s.db.Exec("DROP TABLE document_originals"); err != nil {
		t.Fatal(err)
	}
	s.Close()
	s, err = NewSQLiteStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	saved, err := s.GetDocument(doc.ID)
	if err != nil || saved.RawContent != doc.RawContent || saved.OriginalRetained {
		t.Fatal("migration changed legacy source", err)
	}
	if _, err = s.original(doc.ID); !errors.Is(err, ErrOriginalUnavailable) {
		t.Fatal("migration fabricated original")
	}
}
