package rag

import (
	"os"
	"testing"
	"time"
)

func TestSQLiteStore(t *testing.T) {
	// Setup temp dir
	tmpDir, err := os.MkdirTemp("", "rag_test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	// Initialize store
	store, err := NewSQLiteStore(tmpDir)
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer store.Close()

	// Create test document
	doc := &Document{
		ID:          "doc1",
		Name:        "test.txt",
		ContentType: "text/plain",
		Size:        100,
		ChunkCount:  1,
		Metadata:    map[string]string{"author": "test"},
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	// Add document
	if err := store.AddDocument(doc); err != nil {
		t.Fatalf("Failed to add document: %v", err)
	}

	// Verify document retrieval
	retrievedDoc, err := store.GetDocument("doc1")
	if err != nil {
		t.Fatalf("Failed to get document: %v", err)
	}
	if retrievedDoc == nil {
		t.Fatal("Document not found")
	}
	if retrievedDoc.Name != "test.txt" {
		t.Errorf("Expected name test.txt, got %s", retrievedDoc.Name)
	}
	if retrievedDoc.Metadata["author"] != "test" {
		t.Errorf("Expected metadata author=test, got %v", retrievedDoc.Metadata)
	}

	// Create test chunk
	chunk := &Chunk{
		ID:         "chunk1",
		DocumentID: "doc1",
		Content:    "This is a test chunk content.",
		Index:      0,
		StartChar:  0,
		EndChar:    25,
		CreatedAt:  time.Now(),
	}
	embedding := []float32{0.1, 0.2, 0.3}

	// Add chunk
	if err := store.AddChunk(chunk, embedding); err != nil {
		t.Fatalf("Failed to add chunk: %v", err)
	}

	// Test Search
	// Exact match embedding
	results, err := store.Search(embedding, 5, 0.5)
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}

	if len(results) != 1 {
		t.Errorf("Expected 1 result, got %d", len(results))
	} else {
		if results[0].Chunk.ID != "chunk1" {
			t.Errorf("Expected chunk1, got %s", results[0].Chunk.ID)
		}
		if results[0].Score < 0.99 {
			t.Errorf("Expected score ~1.0, got %f", results[0].Score)
		}
	}

	// Test ListDocuments
	docs, err := store.ListDocuments()
	if err != nil {
		t.Fatalf("ListDocuments failed: %v", err)
	}
	if len(docs) != 1 {
		t.Errorf("Expected 1 document, got %d", len(docs))
	}

	// Test DeleteDocument
	if err := store.DeleteDocument("doc1"); err != nil {
		t.Fatalf("DeleteDocument failed: %v", err)
	}

	// Verify deletion
	retrievedDoc, err = store.GetDocument("doc1")
	if err != nil {
		t.Fatalf("GetDocument after delete failed: %v", err)
	}
	if retrievedDoc != nil {
		t.Error("Document should have been deleted")
	}

	// Verify chunks deleted (cascade)
	var count int
	store.db.QueryRow("SELECT COUNT(*) FROM chunks WHERE document_id = 'doc1'").Scan(&count)
	if count != 0 {
		t.Errorf("Expected 0 chunks after delete, got %d", count)
	}
}
