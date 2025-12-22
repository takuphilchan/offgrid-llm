package rag

import (
	"container/heap"
	"database/sql"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sync"

	_ "modernc.org/sqlite"
)

// SQLiteStore implements a persistent vector store using SQLite
type SQLiteStore struct {
	db     *sql.DB
	dbPath string
	mu     sync.RWMutex
}

// NewSQLiteStore creates a new SQLite-based vector store
func NewSQLiteStore(dataDir string) (*SQLiteStore, error) {
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create data directory: %w", err)
	}

	dbPath := filepath.Join(dataDir, "rag.db")
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	store := &SQLiteStore{
		db:     db,
		dbPath: dbPath,
	}

	if err := store.initSchema(); err != nil {
		db.Close()
		return nil, err
	}

	return store, nil
}

// initSchema initializes the database schema
func (s *SQLiteStore) initSchema() error {
	// Enable WAL mode for better concurrency
	if _, err := s.db.Exec("PRAGMA journal_mode=WAL;"); err != nil {
		return fmt.Errorf("failed to set WAL mode: %w", err)
	}
	// Enable foreign keys for cascade delete
	if _, err := s.db.Exec("PRAGMA foreign_keys = ON;"); err != nil {
		return fmt.Errorf("failed to enable foreign keys: %w", err)
	}

	// Create tables
	queries := []string{
		`CREATE TABLE IF NOT EXISTS documents (
			id TEXT PRIMARY KEY,
			name TEXT NOT NULL,
			content_type TEXT,
			size INTEGER,
			chunk_count INTEGER,
			metadata TEXT,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
		);`,
		`CREATE TABLE IF NOT EXISTS chunks (
			id TEXT PRIMARY KEY,
			document_id TEXT NOT NULL,
			content TEXT NOT NULL,
			chunk_index INTEGER,
			start_char INTEGER,
			end_char INTEGER,
			embedding BLOB, -- Stored as JSON array of floats for now (simple)
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY(document_id) REFERENCES documents(id) ON DELETE CASCADE
		);`,
		`CREATE INDEX IF NOT EXISTS idx_chunks_doc_id ON chunks(document_id);`,
	}

	for _, query := range queries {
		if _, err := s.db.Exec(query); err != nil {
			return fmt.Errorf("failed to execute init query: %w", err)
		}
	}

	return nil
}

// Close closes the database connection
func (s *SQLiteStore) Close() error {
	return s.db.Close()
}

// AddDocument adds a document to the store
func (s *SQLiteStore) AddDocument(doc *Document) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	metadataJSON, _ := json.Marshal(doc.Metadata)

	_, err := s.db.Exec(`
		INSERT OR REPLACE INTO documents (id, name, content_type, size, chunk_count, metadata, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`, doc.ID, doc.Name, doc.ContentType, doc.Size, doc.ChunkCount, string(metadataJSON), doc.CreatedAt, doc.UpdatedAt)

	return err
}

// AddChunk adds a chunk with its embedding to the store
func (s *SQLiteStore) AddChunk(chunk *Chunk, embedding []float32) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	embeddingJSON, _ := json.Marshal(embedding)

	_, err := s.db.Exec(`
		INSERT OR REPLACE INTO chunks (id, document_id, content, chunk_index, start_char, end_char, embedding, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`, chunk.ID, chunk.DocumentID, chunk.Content, chunk.Index, chunk.StartChar, chunk.EndChar, embeddingJSON, chunk.CreatedAt)

	return err
}

// GetDocument retrieves a document by ID
func (s *SQLiteStore) GetDocument(id string) (*Document, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var doc Document
	var metadataJSON string

	err := s.db.QueryRow(`
		SELECT id, name, content_type, size, chunk_count, metadata, created_at, updated_at
		FROM documents WHERE id = ?
	`, id).Scan(&doc.ID, &doc.Name, &doc.ContentType, &doc.Size, &doc.ChunkCount, &metadataJSON, &doc.CreatedAt, &doc.UpdatedAt)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}

	if metadataJSON != "" {
		json.Unmarshal([]byte(metadataJSON), &doc.Metadata)
	}

	return &doc, nil
}

// ListDocuments returns all documents
func (s *SQLiteStore) ListDocuments() ([]*Document, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	rows, err := s.db.Query(`SELECT id, name, content_type, size, chunk_count, metadata, created_at, updated_at FROM documents ORDER BY created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var docs []*Document
	for rows.Next() {
		var doc Document
		var metadataJSON string
		if err := rows.Scan(&doc.ID, &doc.Name, &doc.ContentType, &doc.Size, &doc.ChunkCount, &metadataJSON, &doc.CreatedAt, &doc.UpdatedAt); err != nil {
			return nil, err
		}
		if metadataJSON != "" {
			json.Unmarshal([]byte(metadataJSON), &doc.Metadata)
		}
		docs = append(docs, &doc)
	}
	return docs, nil
}

// DeleteDocument deletes a document and its chunks
func (s *SQLiteStore) DeleteDocument(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Cascade delete handles chunks
	_, err := s.db.Exec("DELETE FROM documents WHERE id = ?", id)
	return err
}

// Search performs a semantic search using cosine similarity
// Uses a min-heap for efficient top-k selection (O(n log k) vs O(n log n) for full sort)
func (s *SQLiteStore) Search(queryEmbedding []float32, limit int, minScore float32) ([]SearchResult, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	// Fetch all chunks and their embeddings
	// In a real vector DB, this would be an index scan
	rows, err := s.db.Query(`
		SELECT c.id, c.document_id, c.content, c.chunk_index, c.start_char, c.end_char, c.embedding, d.name
		FROM chunks c
		JOIN documents d ON c.document_id = d.id
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	// Use a min-heap to maintain top-k results efficiently
	h := &searchResultHeap{}
	heap.Init(h)

	for rows.Next() {
		var chunk Chunk
		var embeddingJSON []byte
		var docName string

		if err := rows.Scan(&chunk.ID, &chunk.DocumentID, &chunk.Content, &chunk.Index, &chunk.StartChar, &chunk.EndChar, &embeddingJSON, &docName); err != nil {
			continue
		}

		var embedding []float32
		if err := json.Unmarshal(embeddingJSON, &embedding); err != nil {
			continue
		}

		score := cosineSimilarity(queryEmbedding, embedding)
		if score >= minScore {
			result := SearchResult{
				Chunk:      &Chunk{ID: chunk.ID, DocumentID: chunk.DocumentID, Content: chunk.Content, Index: chunk.Index, StartChar: chunk.StartChar, EndChar: chunk.EndChar},
				Score:      score,
				DocumentID: chunk.DocumentID,
				DocName:    docName,
			}

			// Maintain a heap of size limit (min-heap by score)
			if h.Len() < limit {
				heap.Push(h, result)
			} else if score > (*h)[0].Score {
				// Replace the minimum element if current score is higher
				heap.Pop(h)
				heap.Push(h, result)
			}
		}
	}

	// Extract results from heap in descending order
	results := make([]SearchResult, h.Len())
	for i := len(results) - 1; i >= 0; i-- {
		results[i] = heap.Pop(h).(SearchResult)
	}

	return results, nil
}

// searchResultHeap implements a min-heap for SearchResult based on Score
type searchResultHeap []SearchResult

func (h searchResultHeap) Len() int           { return len(h) }
func (h searchResultHeap) Less(i, j int) bool { return h[i].Score < h[j].Score } // Min-heap
func (h searchResultHeap) Swap(i, j int)      { h[i], h[j] = h[j], h[i] }

func (h *searchResultHeap) Push(x interface{}) {
	*h = append(*h, x.(SearchResult))
}

func (h *searchResultHeap) Pop() interface{} {
	old := *h
	n := len(old)
	x := old[n-1]
	*h = old[0 : n-1]
	return x
}

// Stats returns statistics about the store
func (s *SQLiteStore) Stats() map[string]interface{} {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var docCount, chunkCount int
	s.db.QueryRow("SELECT COUNT(*) FROM documents").Scan(&docCount)
	s.db.QueryRow("SELECT COUNT(*) FROM chunks").Scan(&chunkCount)

	return map[string]interface{}{
		"document_count": docCount,
		"chunk_count":    chunkCount,
		"backend":        "sqlite",
	}
}

// cosineSimilarity calculates cosine similarity between two embedding vectors
// Optimized with 4x loop unrolling for better CPU pipeline utilization
func cosineSimilarity(a, b []float32) float32 {
	n := len(a)
	if n != len(b) || n == 0 {
		return 0
	}

	// Use float64 for accumulation to avoid precision issues with high-dimensional vectors
	var dot, normA, normB float64

	// Process 4 elements at a time (loop unrolling)
	i := 0
	for ; i <= n-4; i += 4 {
		a0, a1, a2, a3 := float64(a[i]), float64(a[i+1]), float64(a[i+2]), float64(a[i+3])
		b0, b1, b2, b3 := float64(b[i]), float64(b[i+1]), float64(b[i+2]), float64(b[i+3])

		dot += a0*b0 + a1*b1 + a2*b2 + a3*b3
		normA += a0*a0 + a1*a1 + a2*a2 + a3*a3
		normB += b0*b0 + b1*b1 + b2*b2 + b3*b3
	}

	// Handle remaining elements
	for ; i < n; i++ {
		ai, bi := float64(a[i]), float64(b[i])
		dot += ai * bi
		normA += ai * ai
		normB += bi * bi
	}

	if normA == 0 || normB == 0 {
		return 0
	}
	return float32(dot / (math.Sqrt(normA) * math.Sqrt(normB)))
}
