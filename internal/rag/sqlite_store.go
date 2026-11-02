package rag

import (
	"container/heap"
	"database/sql"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"

	"github.com/takuphilchan/offgrid-llm/internal/storage"
)

// SQLiteStore implements a persistent vector store using SQLite
type SQLiteStore struct {
	originalQuota int64
	db            *sql.DB
	dbPath        string
	mu            sync.RWMutex
}

// NewSQLiteStore creates a new SQLite-based vector store
func NewSQLiteStore(dataDir string) (*SQLiteStore, error) {
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create data directory: %w", err)
	}

	dbPath := filepath.Join(dataDir, "rag.db")
	db, err := storage.OpenSQLite(dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	store := &SQLiteStore{
		originalQuota: maxLibraryOriginalBytes,
		db:            db,
		dbPath:        dbPath,
	}

	if err := store.initSchema(); err != nil {
		db.Close()
		return nil, err
	}

	return store, nil
}

// Close closes the database connection
func (s *SQLiteStore) Close() error {
	return s.db.Close()
}

// AddDocument adds a document to the store
func (s *SQLiteStore) AddDocument(doc *Document) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if doc.original != nil {
		return fmt.Errorf("original sources require atomic document publication")
	}
	var retained bool
	if err := s.db.QueryRow(`SELECT EXISTS(SELECT 1 FROM document_originals WHERE document_id=?)`, doc.ID).Scan(&retained); err != nil {
		return err
	}
	if retained {
		return fmt.Errorf("use atomic reindexing for a document with an original source")
	}
	if err := s.checkLegacyReplacementPolicy(doc.ID); err != nil {
		return err
	}

	metadataJSON, _ := json.Marshal(doc.Metadata)

	_, err := s.db.Exec(`
		INSERT OR REPLACE INTO documents
			(id, name, content_type, size, chunk_count, content_hash, index_status, last_error, indexed_at, raw_content, metadata, created_at, updated_at, source_ranges)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, doc.ID, doc.Name, doc.ContentType, doc.Size, doc.ChunkCount, doc.ContentHash, doc.IndexStatus,
		doc.LastError, doc.IndexedAt, doc.RawContent, string(metadataJSON), doc.CreatedAt, doc.UpdatedAt, sourceRangesJSON(doc.SourceRanges))

	return err
}

// AddChunk adds a chunk with its embedding to the store
func (s *SQLiteStore) AddChunk(chunk *Chunk, embedding []float32) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	embeddingJSON, _ := json.Marshal(embedding)

	_, err := s.db.Exec(`
		INSERT OR REPLACE INTO chunks (id, document_id, content, chunk_index, start_char, end_char, page, section, content_hash, embedding, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, chunk.ID, chunk.DocumentID, chunk.Content, chunk.Index, chunk.StartChar, chunk.EndChar,
		chunk.Page, chunk.Section, chunk.ContentHash, embeddingJSON, chunk.CreatedAt)

	return err
}

// AddDocumentWithChunks stores a document and all of its embeddings atomically.
func (s *SQLiteStore) AddDocumentWithChunks(doc *Document, chunks []*Chunk, embeddings [][]float32) error {
	return s.addDocumentWithChunks(doc, chunks, embeddings, nil)
}

func (s *SQLiteStore) addDocumentWithChunks(doc *Document, chunks []*Chunk, embeddings [][]float32, index *IndexMetadata) error {
	if len(chunks) != len(embeddings) {
		return fmt.Errorf("chunk and embedding counts differ: %d != %d", len(chunks), len(embeddings))
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	metadataJSON, err := json.Marshal(doc.Metadata)
	if err != nil {
		return err
	}
	if _, err := tx.Exec(`
		INSERT INTO documents
			(id, name, content_type, size, chunk_count, content_hash, index_status, last_error, indexed_at, raw_content, metadata, created_at, updated_at, source_ranges)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, doc.ID, doc.Name, doc.ContentType, doc.Size, doc.ChunkCount, doc.ContentHash, doc.IndexStatus,
		doc.LastError, doc.IndexedAt, doc.RawContent, string(metadataJSON), doc.CreatedAt, doc.UpdatedAt, sourceRangesJSON(doc.SourceRanges)); err != nil {
		return err
	}
	if err := bindPublication(tx, doc); err != nil {
		return err
	}
	if err := completeImport(tx, doc); err != nil {
		return err
	}
	if err := s.storeOriginal(tx, doc); err != nil {
		return err
	}

	stmt, err := tx.Prepare(`
		INSERT INTO chunks (id, document_id, content, chunk_index, start_char, end_char, page, section, content_hash, embedding, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`)
	if err != nil {
		return err
	}
	defer stmt.Close()
	for i, chunk := range chunks {
		embeddingJSON, err := json.Marshal(embeddings[i])
		if err != nil {
			return err
		}
		if _, err := stmt.Exec(chunk.ID, chunk.DocumentID, chunk.Content, chunk.Index, chunk.StartChar, chunk.EndChar,
			chunk.Page, chunk.Section, chunk.ContentHash, embeddingJSON, chunk.CreatedAt); err != nil {
			return err
		}
	}
	if index != nil {
		if err := storePublicationIdentity(tx, *index); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// ReplaceDocumentWithChunks swaps all derived chunks in one transaction while
// retaining the stable document ID and creation time.
func (s *SQLiteStore) ReplaceDocumentWithChunks(doc *Document, chunks []*Chunk, embeddings [][]float32) error {
	return s.replaceDocumentWithChunks(doc, chunks, embeddings, nil)
}

func (s *SQLiteStore) replaceDocumentWithChunks(doc *Document, chunks []*Chunk, embeddings [][]float32, index *IndexMetadata, principals ...knowledgePrincipal) error {
	if len(chunks) != len(embeddings) {
		return fmt.Errorf("chunk and embedding counts differ: %d != %d", len(chunks), len(embeddings))
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if len(principals) > 0 {
		policy, _, err := documentPolicy(tx, doc.ID)
		if err != nil || !principals[0].allows(policy, true) {
			return ErrCollectionUnavailable
		}
	}
	metadataJSON, err := json.Marshal(doc.Metadata)
	if err != nil {
		return err
	}
	result, err := tx.Exec(`UPDATE documents SET
		name=?, content_type=?, size=?, chunk_count=?, content_hash=?, index_status=?,
		last_error=?, indexed_at=?, raw_content=?, metadata=?, updated_at=?, source_ranges=? WHERE id=?`,
		doc.Name, doc.ContentType, doc.Size, doc.ChunkCount, doc.ContentHash, doc.IndexStatus,
		doc.LastError, doc.IndexedAt, doc.RawContent, string(metadataJSON), doc.UpdatedAt, sourceRangesJSON(doc.SourceRanges), doc.ID)
	if err != nil {
		return err
	}
	if affected, err := result.RowsAffected(); err != nil || affected != 1 {
		return fmt.Errorf("document %q not found", doc.ID)
	}
	if _, err := tx.Exec(`DELETE FROM chunks WHERE document_id = ?`, doc.ID); err != nil {
		return err
	}
	stmt, err := tx.Prepare(`INSERT INTO chunks
		(id, document_id, content, chunk_index, start_char, end_char, page, section, content_hash, embedding, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`)
	if err != nil {
		return err
	}
	defer stmt.Close()
	for i, chunk := range chunks {
		embeddingJSON, err := json.Marshal(embeddings[i])
		if err != nil {
			return err
		}
		if _, err := stmt.Exec(chunk.ID, chunk.DocumentID, chunk.Content, chunk.Index, chunk.StartChar, chunk.EndChar,
			chunk.Page, chunk.Section, chunk.ContentHash, embeddingJSON, chunk.CreatedAt); err != nil {
			return err
		}
	}
	if index != nil {
		if err := storePublicationIdentity(tx, *index); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// GetIndexMetadata returns the persisted index build identity.
func (s *SQLiteStore) GetIndexMetadata() (IndexMetadata, bool, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var metadata IndexMetadata
	err := s.db.QueryRow(`
		SELECT schema_version, embedding_model, embedding_dimension, chunker_version, parser_version, updated_at, embedding_identity
		FROM index_metadata WHERE id = 1
	`).Scan(&metadata.SchemaVersion, &metadata.EmbeddingModel, &metadata.EmbeddingDim,
		&metadata.ChunkerVersion, &metadata.ParserVersion, &metadata.UpdatedAt, &metadata.EmbeddingIdentity)
	if err == sql.ErrNoRows {
		return IndexMetadata{}, false, nil
	}
	if err != nil {
		return IndexMetadata{}, false, err
	}
	return metadata, true, nil
}

// SetIndexMetadata persists the index build identity.
func (s *SQLiteStore) SetIndexMetadata(metadata IndexMetadata) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if metadata.UpdatedAt.IsZero() {
		metadata.UpdatedAt = time.Now().UTC()
	}
	_, err := s.db.Exec(`
		INSERT INTO index_metadata (id, schema_version, embedding_model, embedding_dimension, chunker_version, parser_version, updated_at, embedding_identity)
		VALUES (1, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			schema_version=excluded.schema_version,
			embedding_model=excluded.embedding_model,
			embedding_identity=excluded.embedding_identity,
			embedding_dimension=excluded.embedding_dimension,
			chunker_version=excluded.chunker_version,
			parser_version=excluded.parser_version,
			updated_at=excluded.updated_at
	`, metadata.SchemaVersion, metadata.EmbeddingModel, metadata.EmbeddingDim,
		metadata.ChunkerVersion, metadata.ParserVersion, metadata.UpdatedAt, metadata.EmbeddingIdentity)
	return err
}

// GetDocument retrieves a document by ID
func (s *SQLiteStore) GetDocument(id string) (*Document, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var doc Document
	var metadataJSON, rangesJSON string
	var indexedAt sql.NullTime

	err := s.db.QueryRow(`
		SELECT id, name, content_type, size, chunk_count, content_hash, index_status,
		       last_error, indexed_at, raw_content, metadata, created_at, updated_at, source_ranges,
		       EXISTS(SELECT 1 FROM document_originals WHERE document_id=documents.id AND schema_version=1)
		FROM documents WHERE id = ?
	`, id).Scan(&doc.ID, &doc.Name, &doc.ContentType, &doc.Size, &doc.ChunkCount, &doc.ContentHash,
		&doc.IndexStatus, &doc.LastError, &indexedAt, &doc.RawContent, &metadataJSON, &doc.CreatedAt, &doc.UpdatedAt, &rangesJSON, &doc.OriginalRetained)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}

	if metadataJSON != "" {
		json.Unmarshal([]byte(metadataJSON), &doc.Metadata)
	}
	if indexedAt.Valid {
		doc.IndexedAt = indexedAt.Time
	} else {
		doc.IndexedAt = doc.UpdatedAt
	}
	doc.SourceRetained = doc.RawContent != ""
	if err := json.Unmarshal([]byte(rangesJSON), &doc.SourceRanges); err != nil {
		return nil, fmt.Errorf("invalid retained source ranges: %w", err)
	}
	if err := validateSourceRanges(doc.RawContent, doc.SourceRanges); err != nil {
		return nil, err
	}

	return &doc, nil
}

// ListDocuments returns all documents
func (s *SQLiteStore) ListDocuments() ([]*Document, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	rows, err := s.db.Query(`SELECT id, name, content_type, size, chunk_count, content_hash, index_status,
		last_error, indexed_at, LENGTH(raw_content) > 0, metadata, created_at, updated_at,
		EXISTS(SELECT 1 FROM document_originals WHERE document_id=documents.id AND schema_version=1)
		FROM documents ORDER BY created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	docs := make([]*Document, 0)
	for rows.Next() {
		var doc Document
		var metadataJSON string
		var indexedAt sql.NullTime
		if err := rows.Scan(&doc.ID, &doc.Name, &doc.ContentType, &doc.Size, &doc.ChunkCount, &doc.ContentHash,
			&doc.IndexStatus, &doc.LastError, &indexedAt, &doc.SourceRetained, &metadataJSON, &doc.CreatedAt, &doc.UpdatedAt, &doc.OriginalRetained); err != nil {
			return nil, err
		}
		if indexedAt.Valid {
			doc.IndexedAt = indexedAt.Time
		} else {
			doc.IndexedAt = doc.UpdatedAt
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
	return s.deleteDocument(id)
}

func (s *SQLiteStore) deleteDocument(id string) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if err := checkDeletionRetention(tx, id); err != nil {
		return err
	}
	// Invalidate pending duplicates before deletion, so old extraction cannot
	// recreate the source after deletion commits.
	_, err = tx.Exec(`UPDATE document_imports SET state='deleted',content=NULL,extraction_json='',attempt=attempt+1,updated_at=? WHERE document_id=? OR (source_sha256 IN (SELECT source_sha256 FROM document_originals WHERE document_id=?) AND id IN (SELECT i.import_id FROM import_collections i JOIN document_collections d ON i.collection_id=d.source_namespace WHERE d.document_id=?))`, time.Now().UTC(), id, id, id)
	if err != nil {
		return err
	}
	if _, err = tx.Exec("DELETE FROM documents WHERE id = ?", id); err != nil {
		return err
	}
	if _, err = tx.Exec("DELETE FROM document_retention WHERE document_id = ?", id); err != nil {
		return err
	}
	return tx.Commit()
}

// Search performs a semantic search using cosine similarity
// Uses a min-heap for efficient top-k selection (O(n log k) vs O(n log n) for full sort)
func (s *SQLiteStore) Search(queryEmbedding []float32, limit int, minScore float32) ([]SearchResult, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	// Fetch all chunks and their embeddings with document metadata
	// In a real vector DB, this would be an index scan
	rows, err := s.db.Query(`
		SELECT c.id, c.document_id, c.content, c.chunk_index, c.start_char, c.end_char,
		       c.page, c.section, c.content_hash, c.embedding, d.name, d.metadata, d.source_ranges
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
		var metadataJSON sql.NullString
		var rangesJSON string

		if err := rows.Scan(&chunk.ID, &chunk.DocumentID, &chunk.Content, &chunk.Index, &chunk.StartChar, &chunk.EndChar,
			&chunk.Page, &chunk.Section, &chunk.ContentHash, &embeddingJSON, &docName, &metadataJSON, &rangesJSON); err != nil {
			continue
		}

		var embedding []float32
		if err := json.Unmarshal(embeddingJSON, &embedding); err != nil {
			continue
		}
		chunk.Embedding = embedding
		if chunk.ContentHash == "" {
			chunk.ContentHash = GenerateContentHash(chunk.Content)
		}

		var ranges []SourceRange
		if err := json.Unmarshal([]byte(rangesJSON), &ranges); err != nil {
			return nil, fmt.Errorf("invalid retained source ranges: %w", err)
		}
		bindChunkSource(&chunk, ranges)

		// Parse document metadata
		var metadata map[string]string
		if metadataJSON.Valid && metadataJSON.String != "" {
			json.Unmarshal([]byte(metadataJSON.String), &metadata)
		}

		score := cosineSimilarity(queryEmbedding, embedding)
		if score >= minScore {
			result := SearchResult{
				Chunk:      &chunk,
				Score:      score,
				DocumentID: chunk.DocumentID,
				DocName:    docName,
				Metadata:   metadata,
			}
			result.Locator = locatorForResult(result)

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

// HybridSearch performs a hybrid search combining semantic similarity with FTS5 keyword matching
func (s *SQLiteStore) HybridSearch(queryEmbedding []float32, query string, limit int, minScore float32, alpha float32) ([]SearchResult, error) {
	return s.HybridSearchWithOptions(queryEmbedding, query, SearchOptions{
		TopK: limit, MinScore: minScore, IncludeContent: true,
	}, alpha)
}

// HybridSearchWithOptions performs hybrid retrieval while honoring document
// scope. Filtering before ranking prevents unrelated documents from consuming
// the requested top-k slots.
func (s *SQLiteStore) HybridSearchWithOptions(queryEmbedding []float32, query string, opts SearchOptions, alpha float32) ([]SearchResult, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	limit := opts.TopK
	minScore := opts.MinScore
	if limit <= 0 {
		return []SearchResult{}, nil
	}
	if alpha < 0 {
		alpha = 0
	} else if alpha > 1 {
		alpha = 1
	}

	// Rank keyword matches. We fuse ranks instead of mixing raw BM25 and cosine
	// values, whose scales are unrelated and vary by corpus.
	keywordRanks := make(map[string]int)
	// Apply selection to both keyword candidates and semantic rows before
	// either ranking or limiting. A global FTS top-k can otherwise crowd out
	// every matching passage from the selected source.
	filterSQL, filterArgs := sqliteDocumentSelection(opts.DocumentFilter)
	keywordArgs := append([]any{query}, filterArgs...)
	keywordArgs = append(keywordArgs, limit*10)
	ftsRows, err := s.db.Query(`
		SELECT c.id
		FROM chunks_fts
		JOIN chunks c ON c.rowid = chunks_fts.rowid
		WHERE chunks_fts MATCH ?`+filterSQL+`
		ORDER BY bm25(chunks_fts)
		LIMIT ?
	`, keywordArgs...)

	if err == nil {
		defer ftsRows.Close()
		rank := 1
		for ftsRows.Next() {
			var chunkID string
			if err := ftsRows.Scan(&chunkID); err == nil {
				keywordRanks[chunkID] = rank
				rank++
			}
		}
	}

	// Fetch all chunks for semantic search with document metadata
	rows, err := s.db.Query(`
		SELECT c.id, c.document_id, c.content, c.chunk_index, c.start_char, c.end_char,
		       c.page, c.section, c.content_hash, c.embedding, d.name, d.metadata, d.source_ranges
		FROM chunks c
		JOIN documents d ON c.document_id = d.id
		WHERE 1=1`+filterSQL, filterArgs...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	type candidate struct {
		result   SearchResult
		semantic float32
	}
	candidates := make([]candidate, 0)

	for rows.Next() {
		var chunk Chunk
		var embeddingJSON []byte
		var docName string
		var metadataJSON sql.NullString
		var rangesJSON string

		if err := rows.Scan(&chunk.ID, &chunk.DocumentID, &chunk.Content, &chunk.Index, &chunk.StartChar, &chunk.EndChar,
			&chunk.Page, &chunk.Section, &chunk.ContentHash, &embeddingJSON, &docName, &metadataJSON, &rangesJSON); err != nil {
			continue
		}
		var embedding []float32
		if err := json.Unmarshal(embeddingJSON, &embedding); err != nil {
			continue
		}
		chunk.Embedding = embedding
		if chunk.ContentHash == "" {
			chunk.ContentHash = GenerateContentHash(chunk.Content)
		}

		var ranges []SourceRange
		if err := json.Unmarshal([]byte(rangesJSON), &ranges); err != nil {
			return nil, fmt.Errorf("invalid retained source ranges: %w", err)
		}
		bindChunkSource(&chunk, ranges)

		// Parse document metadata
		var metadata map[string]string
		if metadataJSON.Valid && metadataJSON.String != "" {
			json.Unmarshal([]byte(metadataJSON.String), &metadata)
		}

		semanticScore := cosineSimilarity(queryEmbedding, embedding)
		result := SearchResult{Chunk: &chunk, DocumentID: chunk.DocumentID, DocName: docName, Metadata: metadata}
		result.Locator = locatorForResult(result)
		candidates = append(candidates, candidate{result: result, semantic: semanticScore})
	}

	if len(keywordRanks) == 0 {
		sort.SliceStable(candidates, func(i, j int) bool { return candidates[i].semantic > candidates[j].semantic })
	} else {
		sort.SliceStable(candidates, func(i, j int) bool { return candidates[i].semantic > candidates[j].semantic })
		const rrfK = float32(60)
		for i := range candidates {
			semanticRRF := (rrfK + 1) / (rrfK + float32(i+1))
			keywordRRF := float32(0)
			if rank, ok := keywordRanks[candidates[i].result.Chunk.ID]; ok {
				keywordRRF = (rrfK + 1) / (rrfK + float32(rank))
			}
			candidates[i].result.Score = alpha*semanticRRF + (1-alpha)*keywordRRF
		}
		sort.SliceStable(candidates, func(i, j int) bool { return candidates[i].result.Score > candidates[j].result.Score })
	}
	results := make([]SearchResult, 0, limit)
	for _, candidate := range candidates {
		score := candidate.result.Score
		if len(keywordRanks) == 0 {
			score = candidate.semantic
			candidate.result.Score = score
		}
		if score >= minScore {
			results = append(results, candidate.result)
			if len(results) == limit {
				break
			}
		}
	}
	if !opts.IncludeContent {
		for i := range results {
			chunk := results[i].Chunk
			results[i].Chunk = &Chunk{ID: chunk.ID, DocumentID: chunk.DocumentID, Index: chunk.Index,
				StartChar: chunk.StartChar, EndChar: chunk.EndChar, Page: chunk.Page, Section: chunk.Section,
				ContentHash: chunk.ContentHash, Source: chunk.Source}
		}
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

	var docCount, chunkCount, retainedCount int
	s.db.QueryRow("SELECT COUNT(*) FROM documents").Scan(&docCount)
	s.db.QueryRow("SELECT COUNT(*) FROM chunks").Scan(&chunkCount)
	s.db.QueryRow("SELECT COUNT(*) FROM documents WHERE LENGTH(raw_content) > 0").Scan(&retainedCount)

	return map[string]interface{}{
		"document_count":             docCount,
		"chunk_count":                chunkCount,
		"reindexable_document_count": retainedCount,
		"backend":                    "sqlite",
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
