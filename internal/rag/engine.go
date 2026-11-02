package rag

import (
	"context"
	"fmt"
	"log"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/takuphilchan/offgrid-llm/internal/inference"
	"github.com/takuphilchan/offgrid-llm/pkg/api"
)

// Engine is the main RAG engine that coordinates document ingestion and search
type Engine struct {
	retention         retentionRunner
	imports           importExecution
	mu                sync.RWMutex
	ingestMu          ingestionGate
	store             Store // Interface for vector store
	storageErr        error // Startup failure; never substitute volatile storage.
	chunker           *Chunker
	embeddingEngine   *inference.EmbeddingEngine
	embeddingModel    string
	embeddingIdentity string
	indexErr          error
	resolveModel      func(string) (string, error)
	dataDir           string
	enabled           bool
	hybridAlpha       float32 // Weight for semantic vs keyword search (0=keyword only, 1=semantic only)
	maxContextLen     int     // Maximum context length in characters
	reranking         bool    // Enable MMR-based reranking for diversity
	autoTuneChunks    bool    // Enable automatic chunking parameter tuning
}

// SetModelResolver configures how a stable model ID is resolved to the local
// model file used by the embedding runtime. RAG metadata intentionally stores
// the ID rather than an absolute path so indexes remain portable across native
// and container installations.
func (e *Engine) SetModelResolver(resolve func(string) (string, error)) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.resolveModel = resolve
}

// Store defines the interface for vector storage
type Store interface {
	AddDocument(doc *Document) error
	AddChunk(chunk *Chunk, embedding []float32) error
	GetDocument(id string) (*Document, error)
	ListDocuments() ([]*Document, error)
	DeleteDocument(id string) error
	Search(queryEmbedding []float32, limit int, minScore float32) ([]SearchResult, error)
	HybridSearch(queryEmbedding []float32, query string, limit int, minScore float32, alpha float32) ([]SearchResult, error)
	Stats() map[string]interface{}
	Close() error
}

type batchStore interface {
	AddDocumentWithChunks(doc *Document, chunks []*Chunk, embeddings [][]float32) error
}

type replaceBatchStore interface {
	ReplaceDocumentWithChunks(doc *Document, chunks []*Chunk, embeddings [][]float32) error
}

type filteredHybridStore interface {
	HybridSearchWithOptions(queryEmbedding []float32, query string, opts SearchOptions, alpha float32) ([]SearchResult, error)
}

type indexMetadataStore interface {
	GetIndexMetadata() (IndexMetadata, bool, error)
	SetIndexMetadata(IndexMetadata) error
}

const (
	chunkerVersion = "adaptive-v2"
	parserVersion  = "document-parser-v1"
)

// NewEngine creates a new RAG engine
func NewEngine(embeddingEngine *inference.EmbeddingEngine, dataDir string) *Engine {
	// Initialize SQLite store
	ragDir := filepath.Join(dataDir, "rag")

	var store Store
	sqliteStore, err := NewSQLiteStore(ragDir)
	if err != nil {
		log.Printf("Knowledge storage unavailable; ingestion is disabled: %v", err)
	} else {
		store = sqliteStore
		if recoveryErr := sqliteStore.RecoverImports(); recoveryErr != nil {
			err = recoveryErr
		}
	}

	return &Engine{
		store:           store,
		storageErr:      err,
		chunker:         NewChunker(DefaultChunkingOptions()),
		embeddingEngine: embeddingEngine,
		dataDir:         dataDir,
		enabled:         false,
		hybridAlpha:     0.7,  // 70% semantic, 30% keyword by default
		maxContextLen:   4000, // ~1000 tokens of context
		reranking:       true, // Enable diversity reranking
		autoTuneChunks:  true, // Enable automatic chunking tuning by default
	}
}

// StorageError reports a startup failure without preventing ordinary chat.
// Fix the underlying storage issue and restart to reopen the database.
func (e *Engine) StorageError() error { return e.storageErr }

// Close is called after the service has drained requests and ingestion.
func (e *Engine) Close() error {
	e.stopRetention()
	e.closeImports()
	_ = e.ingestMu.acquire(context.Background()) // Requests have drained before close.
	defer e.ingestMu.release()
	if e.store != nil {
		return e.store.Close()
	}
	return nil
}

// GetPersistedModel returns the embedding model from persisted data (if any)
// This is used to auto-restore RAG on server startup
func (e *Engine) GetPersistedModel() string {
	metadataStore, ok := e.store.(indexMetadataStore)
	if !ok {
		return ""
	}
	metadata, found, err := metadataStore.GetIndexMetadata()
	if err != nil {
		log.Printf("[RAG] Failed to read index metadata: %v", err)
		return ""
	}
	if found {
		return metadata.EmbeddingModel
	}
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

// AutoEnableWithModel auto-enables RAG if an embedding model is available
// It looks for models with names containing "embed", "bge", "minilm", or "nomic"
func (e *Engine) AutoEnableWithModel(ctx context.Context, availableModels []string) error {
	// Already enabled, nothing to do
	if e.IsEnabled() {
		return nil
	}

	// Priority order for embedding models
	prefixes := []string{"bge", "nomic", "embed", "minilm"}

	var embeddingModel string

	// First pass: look for models with known embedding prefixes
	for _, prefix := range prefixes {
		for _, model := range availableModels {
			lower := strings.ToLower(model)
			if strings.Contains(lower, prefix) {
				embeddingModel = model
				break
			}
		}
		if embeddingModel != "" {
			break
		}
	}

	if embeddingModel == "" {
		log.Printf("[RAG] No embedding model found, RAG will remain disabled until manually enabled")
		return nil
	}

	log.Printf("[RAG] Auto-enabling with embedding model: %s", embeddingModel)
	return e.Enable(ctx, embeddingModel)
}

// Enable enables RAG with the specified embedding model
func (e *Engine) Enable(ctx context.Context, embeddingModel string) (enableErr error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	defer func() { e.indexErr = enableErr }()
	if e.storageErr != nil {
		return fmt.Errorf("knowledge storage unavailable: %w", e.storageErr)
	}
	embeddingModel = strings.TrimSpace(embeddingModel)
	if embeddingModel == "" {
		return fmt.Errorf("embedding model is required")
	}

	documents, err := e.store.ListDocuments()
	if err != nil {
		return fmt.Errorf("inspect knowledge documents: %w", err)
	}
	var previous IndexMetadata
	if metadataStore, ok := e.store.(indexMetadataStore); ok {
		metadata, found, err := metadataStore.GetIndexMetadata()
		if err != nil {
			return fmt.Errorf("read RAG index metadata: %w", err)
		}
		previous = metadata
		if len(documents) > 0 && (!found || metadata.SchemaVersion != IndexSchemaVersion || metadata.EmbeddingIdentity == "") {
			e.enabled = false
			return fmt.Errorf("knowledge index requires rebuilding with real embeddings; retained documents are preserved, but unverified vectors cannot be searched")
		}
		if found && len(documents) > 0 {
			if metadata.SchemaVersion != IndexSchemaVersion {
				return fmt.Errorf("RAG index schema %d is incompatible with schema %d; rebuild the index", metadata.SchemaVersion, IndexSchemaVersion)
			}
			if metadata.EmbeddingModel != embeddingModel {
				documents, listErr := e.store.ListDocuments()
				if listErr != nil {
					return fmt.Errorf("inspect RAG index before switching models: %w", listErr)
				}
				if len(documents) > 0 {
					return fmt.Errorf("RAG index was built with embedding model %q, not %q; rebuild the index before switching models", metadata.EmbeddingModel, embeddingModel)
				}
			}
		}
	}

	modelPath := embeddingModel
	if e.resolveModel != nil {
		resolved, err := e.resolveModel(embeddingModel)
		if err != nil {
			return fmt.Errorf("resolve embedding model %q: %w", embeddingModel, err)
		}
		modelPath = resolved
	}

	// Load the requested file when no embedding model is active or a different
	// one is active. EmbeddingEngine.Load safely unloads the previous runtime.
	loadedPath, _ := e.embeddingEngine.GetModelInfo()["model_path"].(string)
	if !e.embeddingEngine.IsLoaded() || loadedPath != modelPath {
		opts := inference.DefaultEmbeddingOptions()
		if err := e.embeddingEngine.Load(ctx, modelPath, opts); err != nil {
			return fmt.Errorf("failed to load embedding model: %w", err)
		}
	}
	identity, _ := e.embeddingEngine.GetModelInfo()["identity"].(string)
	if identity == "" {
		e.enabled = false
		return fmt.Errorf("embedding runtime has no verified identity; knowledge activation refused")
	}
	if len(documents) > 0 && previous.EmbeddingIdentity != identity {
		e.enabled = false
		return fmt.Errorf("embedding model or runtime differs from the saved index; rebuild knowledge before searching")
	}

	e.embeddingModel = embeddingModel
	e.embeddingIdentity = identity
	e.enabled = true
	if metadataStore, ok := e.store.(indexMetadataStore); ok {
		if err := metadataStore.SetIndexMetadata(IndexMetadata{
			SchemaVersion: IndexSchemaVersion, EmbeddingModel: embeddingModel,
			EmbeddingIdentity: identity,
			EmbeddingDim:      e.embeddingEngine.GetDimensions(), ChunkerVersion: chunkerVersion,
			ParserVersion: parserVersion, UpdatedAt: time.Now().UTC(),
		}); err != nil {
			e.enabled = false
			return fmt.Errorf("persist RAG activation: %w", err)
		}
	}

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

// Keep deletion from racing knowledge setup, retrieval, or ingestion.
func (e *Engine) WithModelRemoval(model string, remove func() error) error {
	if !e.mu.TryLock() {
		return inference.ErrRuntimeBusy
	}
	defer e.mu.Unlock()
	if model == e.embeddingModel {
		if e.enabled {
			return fmt.Errorf("disable knowledge before deleting its embedding model")
		}
		if e.embeddingEngine != nil {
			if err := e.embeddingEngine.Unload(); err != nil {
				return err
			}
		}
	}
	return remove()
}

// IsEnabled returns whether RAG is enabled
func (e *Engine) IsEnabled() bool {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.enabled
}

// SetAutoTuning enables or disables automatic chunking tuning
func (e *Engine) SetAutoTuning(enabled bool) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.autoTuneChunks = enabled
	log.Printf("[RAG] Automatic chunking tuning: %v", enabled)
}

// IsAutoTuningEnabled returns whether auto-tuning is enabled
func (e *Engine) IsAutoTuningEnabled() bool {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.autoTuneChunks
}

// AnalyzeDocument analyzes a document and returns chunking recommendations
// This is useful for previewing what settings would be used before ingestion
func (e *Engine) AnalyzeDocument(content string) DocumentAnalysis {
	_, analysis := AutoTuneChunkingOptions(content)
	return analysis
}

// ReindexDocument rebuilds all derived chunks and embeddings from the retained
// source text, then swaps the index atomically. Documents created by older
// versions must be ingested once more because their source was not retained.
func (e *Engine) ReindexDocument(ctx context.Context, documentID string) (*Document, error) {
	if err := e.ingestMu.acquire(ctx); err != nil {
		return nil, err
	}
	defer e.ingestMu.release()
	e.mu.RLock()
	defer e.mu.RUnlock()
	if !e.enabled {
		return nil, fmt.Errorf("RAG is not enabled")
	}
	principal := principalFromContext(ctx)
	if err := e.documentAccess(principal, strings.TrimSpace(documentID), true); err != nil {
		return nil, err
	}
	doc, err := e.store.GetDocument(strings.TrimSpace(documentID))
	if err != nil {
		return nil, fmt.Errorf("load document: %w", err)
	}
	if doc == nil {
		return nil, fmt.Errorf("document %q not found", documentID)
	}
	if strings.TrimSpace(doc.RawContent) == "" {
		return nil, fmt.Errorf("source content is unavailable for document %q; re-ingest it once to enable future reindexing", documentID)
	}
	store, ok := e.store.(replaceBatchStore)
	if !ok {
		return nil, fmt.Errorf("RAG store does not support atomic reindexing")
	}
	chunks, embeddings, embeddingDim, err := e.buildDocumentIndex(ctx, doc, doc.RawContent)
	if err != nil {
		return nil, err
	}
	doc.ChunkCount = len(chunks)
	doc.ContentHash = GenerateContentHash(doc.RawContent)
	doc.Size = int64(len(doc.RawContent))
	doc.IndexStatus = "ready"
	doc.LastError = ""
	doc.IndexedAt = time.Now().UTC()
	doc.UpdatedAt = doc.IndexedAt
	if sqlite, ok := e.store.(*SQLiteStore); ok {
		identity := IndexMetadata{EmbeddingIdentity: e.embeddingIdentity, SchemaVersion: IndexSchemaVersion,
			EmbeddingModel: e.embeddingModel, EmbeddingDim: embeddingDim, ChunkerVersion: chunkerVersion,
			ParserVersion: parserVersion, UpdatedAt: doc.IndexedAt}
		if err := sqlite.replaceDocumentWithChunks(doc, chunks, embeddings, &identity, principal); err != nil {
			return nil, fmt.Errorf("replace document index: %w", err)
		}
		return doc, nil
	}
	if err := store.ReplaceDocumentWithChunks(doc, chunks, embeddings); err != nil {
		return nil, fmt.Errorf("replace document index: %w", err)
	}
	if metadataStore, ok := e.store.(indexMetadataStore); ok {
		if err := metadataStore.SetIndexMetadata(IndexMetadata{
			EmbeddingIdentity: e.embeddingIdentity,
			SchemaVersion:     IndexSchemaVersion, EmbeddingModel: e.embeddingModel, EmbeddingDim: embeddingDim,
			ChunkerVersion: chunkerVersion, ParserVersion: parserVersion, UpdatedAt: doc.IndexedAt,
		}); err != nil {
			return nil, fmt.Errorf("persist index metadata: %w", err)
		}
	}
	return doc, nil
}

func (e *Engine) buildDocumentIndex(ctx context.Context, doc *Document, content string) ([]*Chunk, [][]float32, int, error) {
	chunker := e.chunker
	if e.autoTuneChunks {
		opts, analysis := AutoTuneChunkingOptions(content)
		chunker = NewChunker(opts)
		if doc.Metadata == nil {
			doc.Metadata = make(map[string]string)
		}
		doc.Metadata["doc_type"] = analysis.DocumentType
		doc.Metadata["chunk_size"] = fmt.Sprintf("%d", opts.ChunkSize)
		doc.Metadata["auto_tuned"] = "true"
	}
	chunks, err := chunker.chunkDocument(doc)
	if err != nil {
		return nil, nil, 0, err
	}
	if len(chunks) == 0 {
		return nil, nil, 0, fmt.Errorf("no chunks generated from source content")
	}
	const batchSize = 32
	embeddings := make([][]float32, 0, len(chunks))
	for i := 0; i < len(chunks); i += batchSize {
		end := i + batchSize
		if end > len(chunks) {
			end = len(chunks)
		}
		texts := make([]string, end-i)
		for j, chunk := range chunks[i:end] {
			texts[j] = chunk.Content
		}
		batch, err := e.generateEmbeddings(ctx, texts)
		if err != nil {
			return nil, nil, 0, fmt.Errorf("generate embeddings for batch %d: %w", i/batchSize, err)
		}
		embeddings = append(embeddings, batch...)
	}
	if len(embeddings) != len(chunks) || len(embeddings) == 0 {
		return nil, nil, 0, fmt.Errorf("embedding response count mismatch: got %d for %d chunks", len(embeddings), len(chunks))
	}
	dimension := len(embeddings[0])
	if dimension == 0 {
		return nil, nil, 0, fmt.Errorf("embedding model returned an empty vector")
	}
	for i := range chunks {
		if len(embeddings[i]) != dimension {
			return nil, nil, 0, fmt.Errorf("embedding %d has dimension %d, expected %d", i, len(embeddings[i]), dimension)
		}
		chunks[i].Embedding = append([]float32(nil), embeddings[i]...)
		chunks[i].CreatedAt = time.Now().UTC()
	}
	return chunks, embeddings, dimension, nil
}

// Search searches for relevant chunks using hybrid search (semantic + keyword)
func (e *Engine) Search(ctx context.Context, query string, opts SearchOptions) (*RAGContext, error) {
	return e.search(ctx, query, opts, e.generateEmbeddings)
}

func (e *Engine) search(ctx context.Context, query string, opts SearchOptions, embed func(context.Context, []string) ([][]float32, error)) (*RAGContext, error) {
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
	principal := principalFromContext(ctx)
	if !principal.valid() {
		return nil, ErrCollectionUnavailable
	}
	if store, ok := e.store.(*SQLiteStore); ok {
		requested := opts.DocumentFilter
		if len(requested) == 0 {
			requested = nil
		}
		ids, err := store.authorizedDocumentIDs(principal, requested)
		if err != nil {
			return nil, err
		}
		if len(ids) == 0 {
			return &RAGContext{Query: query, Results: []SearchResult{}}, nil
		}
		opts.DocumentFilter = ids
	}

	// Generate embedding for query
	embeddings, err := embed(ctx, []string{query})
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

	// Use hybrid search combining semantic similarity with keyword matching
	var results []SearchResult
	if store, ok := e.store.(filteredHybridStore); ok {
		results, err = store.HybridSearchWithOptions(embeddings[0], query, searchOpts, e.hybridAlpha)
	} else {
		if len(searchOpts.DocumentFilter) > 0 {
			return nil, fmt.Errorf("knowledge store cannot enforce document filters")
		}
		results, err = e.store.HybridSearch(embeddings[0], query, searchOpts.TopK, searchOpts.MinScore, e.hybridAlpha)
	}
	if err != nil {
		// Never drop document filters to disguise a retrieval failure.
		return nil, fmt.Errorf("search failed: %w", err)
	}
	for _, result := range results {
		if err := e.documentAccess(principal, result.DocumentID, false); err != nil {
			return nil, err
		}
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
	_ = queryEmb
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

			// Calculate max similarity to already selected chunks. Search stores
			// return embeddings specifically for this reranking step.
			maxSim := float32(0.0)
			for _, sel := range selected {
				sim := chunkDiversitySimilarity(candidate.Chunk, sel.Chunk)
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

func chunkDiversitySimilarity(a, b *Chunk) float32 {
	if a == nil || b == nil {
		return 0
	}
	if len(a.Embedding) > 0 && len(a.Embedding) == len(b.Embedding) {
		return cosineSimilarity(a.Embedding, b.Embedding)
	}
	if a.DocumentID == b.DocumentID {
		distance := a.Index - b.Index
		if distance < 0 {
			distance = -distance
		}
		return 1 / float32(distance+1)
	}
	return 0
}

// EnhancePrompt enhances a user prompt with relevant context from documents
func (e *Engine) EnhancePrompt(ctx context.Context, userMessage string) (string, *RAGContext, error) {
	if !e.IsEnabled() {
		log.Printf("[RAG] EnhancePrompt: RAG not enabled")
		return userMessage, nil, nil
	}

	// Check if we have any documents
	docs := e.viewForContext(ctx).ListDocuments()
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
	return e.viewForContext(context.Background()).ListDocuments()
}

// GetDocument returns a document by ID
func (e *Engine) GetDocument(id string) *Document {
	return e.viewForContext(context.Background()).GetDocument(id)
}

// DeleteDocument removes a document and its chunks
func (e *Engine) DeleteDocument(id string) bool {
	return e.viewForContext(context.Background()).DeleteDocument(id)
}

// Stats returns statistics about the RAG engine
func (e *Engine) Stats() map[string]interface{} {
	e.mu.RLock()
	defer e.mu.RUnlock()
	if e.storageErr != nil {
		return map[string]interface{}{
			"enabled": false, "storage_available": false, "persistent": false,
			"error": "Knowledge storage is unavailable. Check disk space and data-directory access, then restart OffGrid.",
		}
	}

	stats := e.store.Stats()
	stats["retention"] = e.RetentionStatus()
	stats["storage_available"] = true
	stats["enabled"] = e.enabled
	stats["embedding_model"] = e.embeddingModel
	if e.indexErr != nil {
		stats["index_error"] = e.indexErr.Error()
		stats["rebuild_required"] = true
	}
	return stats
}

// generateEmbeddings generates embeddings for texts using the embedding engine
func (e *Engine) generateEmbeddings(ctx context.Context, texts []string) ([][]float32, error) {
	req := &api.EmbeddingRequest{
		Model: e.embeddingModel,
		Input: texts,
	}

	resp, err := e.embeddingEngine.GenerateEmbeddingsForIdentity(ctx, req, e.embeddingIdentity)
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
