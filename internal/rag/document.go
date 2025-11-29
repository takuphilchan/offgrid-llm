package rag

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
	"time"
)

// Document represents an uploaded document
type Document struct {
	ID          string            `json:"id"`
	Name        string            `json:"name"`
	ContentType string            `json:"content_type"` // "text/plain", "application/pdf", etc.
	Size        int64             `json:"size"`
	ChunkCount  int               `json:"chunk_count"`
	Metadata    map[string]string `json:"metadata,omitempty"`
	CreatedAt   time.Time         `json:"created_at"`
	UpdatedAt   time.Time         `json:"updated_at"`
}

// Chunk represents a chunk of text from a document
type Chunk struct {
	ID         string    `json:"id"`
	DocumentID string    `json:"document_id"`
	Content    string    `json:"content"`
	Index      int       `json:"index"`      // Position in document
	StartChar  int       `json:"start_char"` // Character offset in original document
	EndChar    int       `json:"end_char"`
	Embedding  []float32 `json:"-"` // Stored separately for efficiency
	CreatedAt  time.Time `json:"created_at"`
}

// SearchResult represents a search result with relevance score
type SearchResult struct {
	Chunk      *Chunk  `json:"chunk"`
	Score      float32 `json:"score"` // Cosine similarity score (0-1)
	DocumentID string  `json:"document_id"`
	DocName    string  `json:"document_name"`
}

// GenerateDocumentID creates a unique ID for a document based on content hash
func GenerateDocumentID(content []byte) string {
	hash := sha256.Sum256(content)
	return hex.EncodeToString(hash[:16]) // First 16 bytes = 32 hex chars
}

// GenerateChunkID creates a unique ID for a chunk
func GenerateChunkID(documentID string, index int) string {
	data := []byte(documentID + string(rune(index)))
	hash := sha256.Sum256(data)
	return hex.EncodeToString(hash[:8]) // 16 hex chars
}

// ChunkingOptions configures how documents are chunked
type ChunkingOptions struct {
	ChunkSize    int    `json:"chunk_size"`    // Target size in characters
	ChunkOverlap int    `json:"chunk_overlap"` // Overlap between chunks
	Separator    string `json:"separator"`     // Primary separator (default: paragraph)
}

// DefaultChunkingOptions returns sensible defaults for general documents
func DefaultChunkingOptions() ChunkingOptions {
	return ChunkingOptions{
		ChunkSize:    512, // ~128 tokens - smaller chunks for better precision
		ChunkOverlap: 128, // 25% overlap for context continuity
		Separator:    "\n\n",
	}
}

// LargeDocumentChunkingOptions returns options for longer documents
func LargeDocumentChunkingOptions() ChunkingOptions {
	return ChunkingOptions{
		ChunkSize:    1024, // ~256 tokens
		ChunkOverlap: 256,  // 25% overlap
		Separator:    "\n\n",
	}
}

// SearchOptions configures search behavior
type SearchOptions struct {
	TopK           int      `json:"top_k"`           // Number of results to return
	MinScore       float32  `json:"min_score"`       // Minimum similarity score (0-1)
	DocumentFilter []string `json:"document_filter"` // Only search these document IDs
	IncludeContent bool     `json:"include_content"` // Include chunk content in results
}

// DefaultSearchOptions returns sensible defaults
func DefaultSearchOptions() SearchOptions {
	return SearchOptions{
		TopK:           5,
		MinScore:       0.35, // Slightly higher threshold for quality
		DocumentFilter: nil,
		IncludeContent: true,
	}
}

// RAGContext represents context to inject into LLM prompts
type RAGContext struct {
	Query   string         `json:"query"`
	Results []SearchResult `json:"results"`
	Context string         `json:"context"` // Formatted context string for injection
}

// FormatContext formats search results into a context string for LLM injection
func (rc *RAGContext) FormatContext() string {
	if len(rc.Results) == 0 {
		rc.Context = ""
		return ""
	}

	var sb strings.Builder
	sb.WriteString("<knowledge_base>\n")
	sb.WriteString("The following information was retrieved from the user's knowledge base and may be relevant:\n\n")

	for i, result := range rc.Results {
		sb.WriteString(fmt.Sprintf("[Document %d: %s | Relevance: %.0f%%]\n",
			i+1, result.DocName, result.Score*100))
		sb.WriteString(result.Chunk.Content)
		sb.WriteString("\n\n")
	}

	sb.WriteString("</knowledge_base>\n\n")
	sb.WriteString("Instructions: Use the knowledge base above to inform your response. ")
	sb.WriteString("Cite sources when using specific information. ")
	sb.WriteString("If the knowledge base doesn't contain relevant information for the question, say so and answer based on your general knowledge.\n\n")

	rc.Context = sb.String()
	return rc.Context
}

// TruncateContext truncates the context to fit within maxLen characters while keeping complete chunks
func (rc *RAGContext) TruncateContext(maxLen int) {
	if len(rc.Context) <= maxLen || len(rc.Results) == 0 {
		return
	}

	// Remove results from the end until we fit
	for len(rc.Results) > 1 {
		rc.Results = rc.Results[:len(rc.Results)-1]
		rc.FormatContext()
		if len(rc.Context) <= maxLen {
			return
		}
	}

	// If still too long with just one result, truncate the content
	if len(rc.Context) > maxLen && len(rc.Results) > 0 {
		// Truncate the chunk content itself
		chunk := rc.Results[0].Chunk
		if len(chunk.Content) > maxLen/2 {
			chunk.Content = chunk.Content[:maxLen/2] + "... [truncated]"
		}
		rc.FormatContext()
	}
}
