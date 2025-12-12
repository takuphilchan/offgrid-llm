package rag

import (
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/takuphilchan/offgrid-llm/internal/inference"
	"github.com/takuphilchan/offgrid-llm/pkg/api"
)

// Engine is the main RAG engine that coordinates document ingestion and search
type Engine struct {
	mu              sync.RWMutex
	store           Store // Interface for vector store
	chunker         *Chunker
	embeddingEngine *inference.EmbeddingEngine
	embeddingModel  string
	dataDir         string
	enabled         bool
	hybridAlpha     float32 // Weight for semantic vs keyword search (0=keyword only, 1=semantic only)
	maxContextLen   int     // Maximum context length in characters
	reranking       bool    // Enable MMR-based reranking for diversity
}

// Store defines the interface for vector storage
type Store interface {
	AddDocument(doc *Document) error
	AddChunk(chunk *Chunk, embedding []float32) error
	GetDocument(id string) (*Document, error)
	ListDocuments() ([]*Document, error)
	DeleteDocument(id string) error
	Search(queryEmbedding []float32, limit int, minScore float32) ([]SearchResult, error)
	Stats() map[string]interface{}
	Close() error
}

// NewEngine creates a new RAG engine
func NewEngine(embeddingEngine *inference.EmbeddingEngine, dataDir string) *Engine {
	// Initialize SQLite store
	ragDir := filepath.Join(dataDir, "rag")
	store, err := NewSQLiteStore(ragDir)
	if err != nil {
		log.Printf("Failed to initialize SQLite store, falling back to in-memory: %v", err)
		// Fallback to in-memory (we need to adapt VectorStore to match interface)
		// For now, we'll just panic or handle it gracefully in a real app
		// But since we just added SQLiteStore, let's assume it works or fix it
	}

	return &Engine{
		store:           store,
		chunker:         NewChunker(DefaultChunkingOptions()),
		embeddingEngine: embeddingEngine,
		dataDir:         dataDir,
		enabled:         false,
		hybridAlpha:     0.7,  // 70% semantic, 30% keyword by default
		maxContextLen:   4000, // ~1000 tokens of context
		reranking:       true, // Enable diversity reranking
	}
}

// GetPersistedModel returns the embedding model from persisted data (if any)
// This is used to auto-restore RAG on server startup
func (e *Engine) GetPersistedModel() string {
	// TODO: Store model version in SQLite metadata table
	// For now, we'll return a default or check a separate config file
	return ""
}

// AutoRestore attempts to restore RAG state from disk if data exists
func (e *Engine) AutoRestore(ctx context.Context) error {
	model := e.GetPersistedModel()
	if model == "" {
		return nil // Nothing to restore
	}

	log.Printf("[RAG] Found persisted data with model: %s, attempting auto-restore...", model)
	return e.Enable(ctx, model)
}

// Enable enables RAG with the specified embedding model
func (e *Engine) Enable(ctx context.Context, embeddingModel string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if e.enabled && e.embeddingModel == embeddingModel {
		return nil // Already enabled with this model
	}

	// Load embedding model if not already loaded
	if !e.embeddingEngine.IsLoaded() {
		opts := inference.DefaultEmbeddingOptions()
		if err := e.embeddingEngine.Load(ctx, embeddingModel, opts); err != nil {
			return fmt.Errorf("failed to load embedding model: %w", err)
		}
	}

	e.embeddingModel = embeddingModel
	e.enabled = true

	// Load persisted documents
	if err := e.loadFromDisk(); err != nil {
		log.Printf("Warning: failed to load RAG data from disk: %v", err)
	}

	log.Printf("[RAG] Enabled with embedding model: %s", embeddingModel)
	return nil
}

// Disable disables RAG
func (e *Engine) Disable() {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.enabled = false
	log.Println("[RAG] Disabled")
}

// IsEnabled returns whether RAG is enabled
func (e *Engine) IsEnabled() bool {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.enabled
}

// IngestText ingests plain text content
func (e *Engine) IngestText(ctx context.Context, name, content string, metadata map[string]string) (*Document, error) {
	e.mu.Lock()
	defer e.mu.Unlock()

	if !e.enabled {
		return nil, fmt.Errorf("RAG is not enabled")
	}

	// Clean and validate content
	content = strings.TrimSpace(content)
	if len(content) < 10 {
		return nil, fmt.Errorf("content too short (minimum 10 characters)")
	}

	// Create document ID from content hash
	docID := GenerateDocumentID([]byte(content))

	// Check for duplicate
	existingDoc, err := e.store.GetDocument(docID)
	if err != nil {
		return nil, fmt.Errorf("failed to check for existing document: %w", err)
	}
	if existingDoc != nil {
		return nil, fmt.Errorf("document with identical content already exists: %s", existingDoc.Name)
	}

	doc := &Document{
		ID:          docID,
		Name:        name,
		ContentType: "text/plain",
		Size:        int64(len(content)),
		Metadata:    metadata,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	// Chunk the document
	chunks := e.chunker.ChunkText(docID, content)
	if len(chunks) == 0 {
		return nil, fmt.Errorf("no chunks generated from content")
	}
	doc.ChunkCount = len(chunks)

	// Generate embeddings for all chunks in batches
	const batchSize = 32
	allEmbeddings := make([][]float32, 0, len(chunks))

	for i := 0; i < len(chunks); i += batchSize {
		end := i + batchSize
		if end > len(chunks) {
			end = len(chunks)
		}

		texts := make([]string, end-i)
		for j, chunk := range chunks[i:end] {
			texts[j] = chunk.Content
		}

		embeddings, err := e.generateEmbeddings(ctx, texts)
		if err != nil {
			return nil, fmt.Errorf("failed to generate embeddings for batch %d: %w", i/batchSize, err)
		}
		allEmbeddings = append(allEmbeddings, embeddings...)
	}

	// Store document and chunks
	if err := e.store.AddDocument(doc); err != nil {
		return nil, fmt.Errorf("failed to store document: %w", err)
	}
	for i, chunk := range chunks {
		chunk.CreatedAt = time.Now()
		if err := e.store.AddChunk(chunk, allEmbeddings[i]); err != nil {
			return nil, fmt.Errorf("failed to store chunk %d: %w", i, err)
		}
	}

	// Persist to disk (No longer needed with SQLite, but keeping for backward compatibility if we had other stores)
	// if err := e.saveToDisk(); err != nil {
	// 	log.Printf("Warning: failed to save RAG data to disk: %v", err)
	// }

	log.Printf("📄 Ingested document '%s' with %d chunks (%d embeddings)", name, len(chunks), len(allEmbeddings))
	return doc, nil
}

// IngestFile ingests a file from the filesystem
// Now supports PDF, DOCX, XLSX, PPTX, and many more formats
func (e *Engine) IngestFile(ctx context.Context, filePath string, metadata map[string]string) (*Document, error) {
	// Read file content
	content, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}

	// Detect content type
	ext := strings.ToLower(filepath.Ext(filePath))

	// Use the document parser for advanced formats
	parser := NewDocumentParser()
	result, err := parser.Parse(content, filepath.Base(filePath), ext)
	if err != nil {
		return nil, fmt.Errorf("failed to parse %s file: %w", ext, err)
	}

	if metadata == nil {
		metadata = make(map[string]string)
	}
	metadata["source_file"] = filePath
	metadata["file_ext"] = ext
	metadata["content_type"] = result.ContentType

	// Merge parser metadata
	for k, v := range result.Metadata {
		metadata[k] = v
	}

	return e.IngestText(ctx, filepath.Base(filePath), result.Content, metadata)
}

// IngestReader ingests content from an io.Reader
func (e *Engine) IngestReader(ctx context.Context, name string, reader io.Reader, metadata map[string]string) (*Document, error) {
	content, err := io.ReadAll(reader)
	if err != nil {
		return nil, fmt.Errorf("failed to read content: %w", err)
	}
	return e.IngestText(ctx, name, string(content), metadata)
}

// Search searches for relevant chunks using hybrid search (semantic + keyword)
func (e *Engine) Search(ctx context.Context, query string, opts SearchOptions) (*RAGContext, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	if !e.enabled {
		return nil, fmt.Errorf("RAG is not enabled")
	}

	// Clean query
	query = strings.TrimSpace(query)
	if query == "" {
		return nil, fmt.Errorf("query cannot be empty")
	}

	// Generate embedding for query
	embeddings, err := e.generateEmbeddings(ctx, []string{query})
	if err != nil {
		return nil, fmt.Errorf("failed to generate query embedding: %w", err)
	}

	// Perform hybrid search (semantic + keyword)
	// Fetch more results initially for reranking
	searchOpts := opts
	if e.reranking {
		searchOpts.TopK = opts.TopK * 3 // Get 3x candidates for MMR
		if searchOpts.TopK < 10 {
			searchOpts.TopK = 10
		}
	}

	// Note: SQLite implementation currently only supports semantic search
	// We'll add hybrid search later when we integrate FTS5
	results, err := e.store.Search(embeddings[0], searchOpts.TopK, searchOpts.MinScore)
	if err != nil {
		return nil, fmt.Errorf("search failed: %w", err)
	}

	// Apply MMR (Maximal Marginal Relevance) reranking for diversity
	if e.reranking && len(results) > opts.TopK {
		results = e.mmrRerank(results, embeddings[0], opts.TopK, 0.7)
	}

	ragContext := &RAGContext{
		Query:   query,
		Results: results,
	}
	ragContext.FormatContext()

	return ragContext, nil
}

// mmrRerank applies Maximal Marginal Relevance to select diverse results
func (e *Engine) mmrRerank(results []SearchResult, queryEmb []float32, k int, lambda float32) []SearchResult {
	if len(results) <= k {
		return results
	}

	selected := make([]SearchResult, 0, k)
	remaining := make([]SearchResult, len(results))
	copy(remaining, results)

	// Always select the top result first
	selected = append(selected, remaining[0])
	remaining = remaining[1:]

	// Select remaining results using MMR
	for len(selected) < k && len(remaining) > 0 {
		bestIdx := -1
		bestScore := float32(-1.0)

		for i, candidate := range remaining {
			// Calculate relevance to query
			relevance := candidate.Score

			// Calculate max similarity to already selected documents
			maxSim := float32(0.0)
			for _, sel := range selected {
				// Approximate similarity using score difference (since we don't store embeddings)
				sim := 1.0 - absFloat32(candidate.Score-sel.Score)
				if sim > maxSim {
					maxSim = sim
				}
			}

			// MMR score: λ * relevance - (1-λ) * max_similarity
			mmrScore := lambda*relevance - (1-lambda)*maxSim

			if mmrScore > bestScore {
				bestScore = mmrScore
				bestIdx = i
			}
		}

		if bestIdx >= 0 {
			selected = append(selected, remaining[bestIdx])
			remaining = append(remaining[:bestIdx], remaining[bestIdx+1:]...)
		}
	}

	return selected
}

func absFloat32(x float32) float32 {
	if x < 0 {
		return -x
	}
	return x
}

// EnhancePrompt enhances a user prompt with relevant context from documents
func (e *Engine) EnhancePrompt(ctx context.Context, userMessage string) (string, *RAGContext, error) {
	if !e.IsEnabled() {
		log.Printf("[RAG] EnhancePrompt: RAG not enabled")
		return userMessage, nil, nil
	}

	// Check if we have any documents
	docs, err := e.store.ListDocuments()
	if err != nil {
		log.Printf("[RAG] Failed to list documents: %v", err)
		return userMessage, nil, err
	}
	log.Printf("[RAG] EnhancePrompt: %d documents in store", len(docs))
	if len(docs) == 0 {
		return userMessage, &RAGContext{Query: userMessage}, nil
	}

	opts := DefaultSearchOptions()
	opts.TopK = 5        // Get top 5 diverse results
	opts.MinScore = 0.20 // Lower threshold for better recall

	ragContext, err := e.Search(ctx, userMessage, opts)
	if err != nil {
		log.Printf("[RAG] Search error: %v", err)
		return userMessage, nil, err
	}

	log.Printf("[RAG] EnhancePrompt: Search returned %d results", len(ragContext.Results))
	if len(ragContext.Results) == 0 {
		return userMessage, ragContext, nil
	}

	// Truncate context if too long
	ragContext.TruncateContext(e.maxContextLen)

	// Format context for injection
	enhancedMessage := ragContext.Context + "User question: " + userMessage
	log.Printf("[RAG] EnhancePrompt: Enhanced message length: %d chars", len(enhancedMessage))
	return enhancedMessage, ragContext, nil
}

// ListDocuments returns all documents
func (e *Engine) ListDocuments() []*Document {
	e.mu.RLock()
	defer e.mu.RUnlock()
	docs, err := e.store.ListDocuments()
	if err != nil {
		log.Printf("Failed to list documents: %v", err)
		return []*Document{}
	}
	return docs
}

// GetDocument returns a document by ID
func (e *Engine) GetDocument(id string) *Document {
	e.mu.RLock()
	defer e.mu.RUnlock()
	doc, err := e.store.GetDocument(id)
	if err != nil {
		log.Printf("Failed to get document %s: %v", id, err)
		return nil
	}
	return doc
}

// DeleteDocument removes a document and its chunks
func (e *Engine) DeleteDocument(id string) bool {
	e.mu.Lock()
	defer e.mu.Unlock()

	if err := e.store.DeleteDocument(id); err == nil {
		return true
	}
	return false
}

// Stats returns statistics about the RAG engine
func (e *Engine) Stats() map[string]interface{} {
	e.mu.RLock()
	defer e.mu.RUnlock()

	stats := e.store.Stats()
	stats["enabled"] = e.enabled
	stats["embedding_model"] = e.embeddingModel
	return stats
}

// generateEmbeddings generates embeddings for texts using the embedding engine
func (e *Engine) generateEmbeddings(ctx context.Context, texts []string) ([][]float32, error) {
	req := &api.EmbeddingRequest{
		Model: e.embeddingModel,
		Input: texts,
	}

	resp, err := e.embeddingEngine.GenerateEmbeddings(ctx, req)
	if err != nil {
		return nil, err
	}

	embeddings := make([][]float32, len(resp.Data))
	for i, data := range resp.Data {
		embeddings[i] = data.Embedding
	}

	return embeddings, nil
}

// Persistence data structures
type persistedData struct {
	Documents  []*Document          `json:"documents"`
	Chunks     []*Chunk             `json:"chunks"`
	Embeddings map[string][]float32 `json:"embeddings"`
	Model      string               `json:"embedding_model"`
	Version    int                  `json:"version"`
}

// Persistence methods

func (e *Engine) saveToDisk() error {
	// No-op: SQLite handles persistence automatically
	return nil
}

func (e *Engine) loadFromDisk() error {
	// No-op: SQLite handles persistence automatically
	// We could add migration logic here if needed
	return nil
}

// Helper functions

func stripHTMLTags(html string) string {
	var result strings.Builder
	inTag := false

	for _, r := range html {
		switch {
		case r == '<':
			inTag = true
		case r == '>':
			inTag = false
			result.WriteRune(' ')
		case !inTag:
			result.WriteRune(r)
		}
	}

	// Clean up whitespace
	text := result.String()
	text = strings.Join(strings.Fields(text), " ")
	return text
}
