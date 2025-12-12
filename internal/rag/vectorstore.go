package rag

import (
	"container/heap"
	"fmt"
	"math"
	"strings"
	"sync"
)

// VectorStore is an in-memory vector database for semantic search
type VectorStore struct {
	mu         sync.RWMutex
	embeddings map[string][]float32 // chunkID -> embedding
	chunks     map[string]*Chunk    // chunkID -> chunk
	documents  map[string]*Document // documentID -> document
	docChunks  map[string][]string  // documentID -> []chunkID
}

// NewVectorStore creates a new in-memory vector store
func NewVectorStore() *VectorStore {
	return &VectorStore{
		embeddings: make(map[string][]float32),
		chunks:     make(map[string]*Chunk),
		documents:  make(map[string]*Document),
		docChunks:  make(map[string][]string),
	}
}

// AddDocument adds a document to the store
func (vs *VectorStore) AddDocument(doc *Document) error {
	vs.mu.Lock()
	defer vs.mu.Unlock()
	vs.documents[doc.ID] = doc
	return nil
}

// AddChunk adds a chunk with its embedding to the store
func (vs *VectorStore) AddChunk(chunk *Chunk, embedding []float32) error {
	vs.mu.Lock()
	defer vs.mu.Unlock()

	vs.chunks[chunk.ID] = chunk
	vs.embeddings[chunk.ID] = embedding

	// Track which chunks belong to which document
	vs.docChunks[chunk.DocumentID] = append(vs.docChunks[chunk.DocumentID], chunk.ID)
	return nil
}

// GetDocument retrieves a document by ID
func (vs *VectorStore) GetDocument(id string) (*Document, error) {
	vs.mu.RLock()
	defer vs.mu.RUnlock()
	doc, ok := vs.documents[id]
	if !ok {
		return nil, fmt.Errorf("document not found")
	}
	return doc, nil
}

// GetChunk retrieves a chunk by ID
func (vs *VectorStore) GetChunk(id string) (*Chunk, error) {
	vs.mu.RLock()
	defer vs.mu.RUnlock()
	chunk, ok := vs.chunks[id]
	if !ok {
		return nil, fmt.Errorf("chunk not found")
	}
	return chunk, nil
}

// ListDocuments returns all documents
func (vs *VectorStore) ListDocuments() ([]*Document, error) {
	vs.mu.RLock()
	defer vs.mu.RUnlock()

	docs := make([]*Document, 0, len(vs.documents))
	for _, doc := range vs.documents {
		docs = append(docs, doc)
	}
	return docs, nil
}

// DeleteDocument removes a document and all its chunks
func (vs *VectorStore) DeleteDocument(docID string) error {
	vs.mu.Lock()
	defer vs.mu.Unlock()

	if _, exists := vs.documents[docID]; !exists {
		return fmt.Errorf("document not found")
	}

	// Remove all chunks for this document
	chunkIDs := vs.docChunks[docID]
	for _, chunkID := range chunkIDs {
		delete(vs.chunks, chunkID)
		delete(vs.embeddings, chunkID)
	}

	delete(vs.docChunks, docID)
	delete(vs.documents, docID)
	return nil
}

// Search finds the top-k most similar chunks to the query embedding
func (vs *VectorStore) Search(queryEmbedding []float32, limit int, minScore float32) ([]SearchResult, error) {
	vs.mu.RLock()
	defer vs.mu.RUnlock()

	if len(vs.embeddings) == 0 {
		return []SearchResult{}, nil
	}

	// Use a max-heap to find top-k results efficiently
	h := &resultHeap{}
	heap.Init(h)

	for chunkID, embedding := range vs.embeddings {
		chunk := vs.chunks[chunkID]
		if chunk == nil {
			continue
		}

		// Calculate cosine similarity
		score := cosineSimilarityInMemory(queryEmbedding, embedding)

		// Skip low scores
		if score < minScore {
			continue
		}

		// Get document name
		docName := ""
		if doc := vs.documents[chunk.DocumentID]; doc != nil {
			docName = doc.Name
		}

		result := SearchResult{
			Chunk:      chunk,
			Score:      score,
			DocumentID: chunk.DocumentID,
			DocName:    docName,
		}

		// Maintain top-k using min-heap
		if h.Len() < limit {
			heap.Push(h, result)
		} else if h.Len() > 0 && score > (*h)[0].Score {
			heap.Pop(h)
			heap.Push(h, result)
		}
	}

	// Extract results in descending order of score
	results := make([]SearchResult, h.Len())
	for i := h.Len() - 1; i >= 0; i-- {
		results[i] = heap.Pop(h).(SearchResult)
	}

	return results, nil
}

// HybridSearch combines semantic search with keyword matching
func (vs *VectorStore) HybridSearch(queryEmbedding []float32, query string, opts SearchOptions, alpha float32) []SearchResult {
	vs.mu.RLock()
	defer vs.mu.RUnlock()

	if len(vs.embeddings) == 0 {
		return nil
	}

	// Build a filter set if document filter is provided
	docFilter := make(map[string]bool)
	if len(opts.DocumentFilter) > 0 {
		for _, docID := range opts.DocumentFilter {
			docFilter[docID] = true
		}
	}

	// Tokenize query for keyword matching
	queryTerms := tokenize(query)

	// Use a max-heap to find top-k results efficiently
	h := &resultHeap{}
	heap.Init(h)

	for chunkID, embedding := range vs.embeddings {
		chunk := vs.chunks[chunkID]
		if chunk == nil {
			continue
		}

		// Apply document filter
		if len(docFilter) > 0 && !docFilter[chunk.DocumentID] {
			continue
		}

		// Calculate semantic similarity (cosine)
		semanticScore := cosineSimilarity(queryEmbedding, embedding)

		// Calculate keyword score (BM25-like)
		keywordScore := calculateKeywordScore(chunk.Content, queryTerms)

		// Combine scores with weighted average
		// alpha controls the balance: 1.0 = pure semantic, 0.0 = pure keyword
		combinedScore := alpha*semanticScore + (1-alpha)*keywordScore

		// Skip low scores
		if combinedScore < opts.MinScore {
			continue
		}

		// Get document name
		docName := ""
		if doc := vs.documents[chunk.DocumentID]; doc != nil {
			docName = doc.Name
		}

		result := SearchResult{
			Chunk:      chunk,
			Score:      combinedScore,
			DocumentID: chunk.DocumentID,
			DocName:    docName,
		}

		// Maintain top-k using min-heap
		if h.Len() < opts.TopK {
			heap.Push(h, result)
		} else if combinedScore > (*h)[0].Score {
			heap.Pop(h)
			heap.Push(h, result)
		}
	}

	// Extract results in descending order of score
	results := make([]SearchResult, h.Len())
	for i := h.Len() - 1; i >= 0; i-- {
		results[i] = heap.Pop(h).(SearchResult)
	}

	// Clear content if not requested
	if !opts.IncludeContent {
		for i := range results {
			results[i].Chunk = &Chunk{
				ID:         results[i].Chunk.ID,
				DocumentID: results[i].Chunk.DocumentID,
				Index:      results[i].Chunk.Index,
			}
		}
	}

	return results
}

// ListChunks returns all chunks
func (vs *VectorStore) ListChunks() []*Chunk {
	vs.mu.RLock()
	defer vs.mu.RUnlock()

	chunks := make([]*Chunk, 0, len(vs.chunks))
	for _, chunk := range vs.chunks {
		chunks = append(chunks, chunk)
	}
	return chunks
}

// GetAllEmbeddings returns all embeddings
func (vs *VectorStore) GetAllEmbeddings() map[string][]float32 {
	vs.mu.RLock()
	defer vs.mu.RUnlock()

	// Return a copy to prevent external modification
	result := make(map[string][]float32, len(vs.embeddings))
	for id, embedding := range vs.embeddings {
		embCopy := make([]float32, len(embedding))
		copy(embCopy, embedding)
		result[id] = embCopy
	}
	return result
}

// Stats returns statistics about the store
func (vs *VectorStore) Stats() map[string]interface{} {
	vs.mu.RLock()
	defer vs.mu.RUnlock()

	return map[string]interface{}{
		"document_count":  len(vs.documents),
		"chunk_count":     len(vs.chunks),
		"embedding_count": len(vs.embeddings),
		"backend":         "memory",
	}
}

// Close closes the store (no-op for in-memory)
func (vs *VectorStore) Close() error {
	return nil
}

// cosineSimilarity calculates the cosine similarity between two vectors
// Renamed to avoid conflict with sqlite_store.go
func cosineSimilarityInMemory(a, b []float32) float32 {
	if len(a) != len(b) || len(a) == 0 {
		return 0
	}

	var dotProduct, normA, normB float64
	for i := range a {
		dotProduct += float64(a[i]) * float64(b[i])
		normA += float64(a[i]) * float64(a[i])
		normB += float64(b[i]) * float64(b[i])
	}

	if normA == 0 || normB == 0 {
		return 0
	}

	return float32(dotProduct / (math.Sqrt(normA) * math.Sqrt(normB)))
}

// tokenize splits text into lowercase tokens for keyword matching
func tokenize(text string) []string {
	// Convert to lowercase
	text = strings.ToLower(text)

	// Split on non-alphanumeric characters
	var tokens []string
	var current strings.Builder

	for _, r := range text {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			current.WriteRune(r)
		} else if current.Len() > 0 {
			token := current.String()
			// Skip very short tokens and common stop words
			if len(token) > 2 && !isStopWord(token) {
				tokens = append(tokens, token)
			}
			current.Reset()
		}
	}

	// Don't forget the last token
	if current.Len() > 0 {
		token := current.String()
		if len(token) > 2 && !isStopWord(token) {
			tokens = append(tokens, token)
		}
	}

	return tokens
}

// isStopWord checks if a word is a common stop word
func isStopWord(word string) bool {
	stopWords := map[string]bool{
		"the": true, "and": true, "for": true, "are": true, "but": true,
		"not": true, "you": true, "all": true, "can": true, "had": true,
		"her": true, "was": true, "one": true, "our": true, "out": true,
		"has": true, "have": true, "been": true, "were": true, "will": true,
		"would": true, "there": true, "their": true, "what": true, "about": true,
		"which": true, "when": true, "make": true, "like": true, "time": true,
		"just": true, "know": true, "take": true, "into": true, "year": true,
		"your": true, "some": true, "them": true, "than": true, "then": true,
		"now": true, "look": true, "only": true, "come": true, "its": true,
		"over": true, "think": true, "also": true, "back": true, "after": true,
		"use": true, "two": true, "how": true, "first": true, "way": true,
		"could": true, "these": true, "from": true, "with": true, "that": true,
		"this": true, "where": true, "does": true, "don": true, "didn": true,
	}
	return stopWords[word]
}

// calculateKeywordScore calculates a BM25-like keyword relevance score
func calculateKeywordScore(content string, queryTerms []string) float32 {
	if len(queryTerms) == 0 {
		return 0
	}

	// Tokenize content
	contentTokens := tokenize(content)
	if len(contentTokens) == 0 {
		return 0
	}

	// Count term frequencies in content
	termFreq := make(map[string]int)
	for _, token := range contentTokens {
		termFreq[token]++
	}

	// Calculate score based on term matches
	var score float32
	matchedTerms := 0

	for _, queryTerm := range queryTerms {
		if freq, exists := termFreq[queryTerm]; exists {
			matchedTerms++
			// BM25-like scoring: diminishing returns for repeated terms
			// tf = freq / (freq + 1.2)
			tf := float32(freq) / (float32(freq) + 1.2)
			score += tf
		}
	}

	if matchedTerms == 0 {
		return 0
	}

	// Normalize by query length and boost for more matches
	matchRatio := float32(matchedTerms) / float32(len(queryTerms))
	score = (score / float32(len(queryTerms))) * (0.5 + 0.5*matchRatio)

	// Clamp to 0-1 range
	if score > 1.0 {
		score = 1.0
	}

	return score
}

// resultHeap is a min-heap for finding top-k results
type resultHeap []SearchResult

func (h resultHeap) Len() int           { return len(h) }
func (h resultHeap) Less(i, j int) bool { return h[i].Score < h[j].Score } // Min-heap by score
func (h resultHeap) Swap(i, j int)      { h[i], h[j] = h[j], h[i] }

func (h *resultHeap) Push(x interface{}) {
	*h = append(*h, x.(SearchResult))
}

func (h *resultHeap) Pop() interface{} {
	old := *h
	n := len(old)
	x := old[n-1]
	*h = old[0 : n-1]
	return x
}
