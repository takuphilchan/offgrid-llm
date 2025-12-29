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
	Chunk      *Chunk            `json:"chunk"`
	Score      float32           `json:"score"` // Cosine similarity score (0-1)
	DocumentID string            `json:"document_id"`
	DocName    string            `json:"document_name"`
	Metadata   map[string]string `json:"metadata,omitempty"` // Source URL, author, etc.
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

// DocumentAnalysis contains analysis results for automatic chunking tuning
type DocumentAnalysis struct {
	TotalChars      int             `json:"total_chars"`
	TotalWords      int             `json:"total_words"`
	TotalParagraphs int             `json:"total_paragraphs"`
	TotalSentences  int             `json:"total_sentences"`
	AvgWordsPerPara float64         `json:"avg_words_per_para"`
	AvgWordsPerSent float64         `json:"avg_words_per_sent"`
	DocumentType    string          `json:"document_type"` // prose, technical, code, list, mixed
	RecommendedOpts ChunkingOptions `json:"recommended_options"`
	Reasoning       string          `json:"reasoning"`
}

// AutoTuneChunkingOptions analyzes document and returns optimized chunking options
func AutoTuneChunkingOptions(content string) (ChunkingOptions, DocumentAnalysis) {
	analysis := analyzeDocument(content)

	// Determine optimal chunk size based on document characteristics
	var opts ChunkingOptions
	var reasoning string

	switch analysis.DocumentType {
	case "code":
		// Code: smaller chunks, respect function boundaries
		opts = ChunkingOptions{
			ChunkSize:    384, // ~96 tokens - function-sized chunks
			ChunkOverlap: 64,  // Small overlap, code is more self-contained
			Separator:    "\n\n",
		}
		reasoning = "Code detected: using smaller chunks for function-level granularity"

	case "technical":
		// Technical docs: medium chunks, preserve sections
		opts = ChunkingOptions{
			ChunkSize:    768, // ~192 tokens - section-sized chunks
			ChunkOverlap: 192, // 25% overlap for cross-reference context
			Separator:    "\n\n",
		}
		reasoning = "Technical document detected: medium chunks for section preservation"

	case "list":
		// Lists/data: very small chunks, each item standalone
		opts = ChunkingOptions{
			ChunkSize:    256, // ~64 tokens - item-sized chunks
			ChunkOverlap: 32,  // Minimal overlap
			Separator:    "\n",
		}
		reasoning = "List/structured data detected: small chunks for item-level retrieval"

	case "prose":
		// Prose: larger chunks for narrative flow
		if analysis.TotalWords > 5000 {
			opts = ChunkingOptions{
				ChunkSize:    1024, // ~256 tokens - larger for long narratives
				ChunkOverlap: 256,  // 25% overlap
				Separator:    "\n\n",
			}
			reasoning = "Long prose document: larger chunks for narrative context"
		} else {
			opts = ChunkingOptions{
				ChunkSize:    512, // ~128 tokens
				ChunkOverlap: 128, // 25% overlap
				Separator:    "\n\n",
			}
			reasoning = "Prose document: standard chunks for balanced retrieval"
		}

	default: // mixed
		// Mixed content: balanced approach
		opts = DefaultChunkingOptions()
		reasoning = "Mixed content: using balanced default settings"
	}

	// Adjust for very short or very long documents
	expectedChunks := float64(analysis.TotalChars) / float64(opts.ChunkSize)
	if expectedChunks < 3 && analysis.TotalChars > 200 {
		// Too few chunks - reduce size for better granularity
		opts.ChunkSize = analysis.TotalChars / 4
		if opts.ChunkSize < 128 {
			opts.ChunkSize = 128
		}
		opts.ChunkOverlap = opts.ChunkSize / 4
		reasoning += "; reduced chunk size for short document"
	} else if expectedChunks > 100 {
		// Too many chunks - increase size
		opts.ChunkSize = 1024
		opts.ChunkOverlap = 256
		reasoning += "; increased chunk size for large document"
	}

	analysis.RecommendedOpts = opts
	analysis.Reasoning = reasoning

	return opts, analysis
}

// analyzeDocument analyzes content to determine its characteristics
func analyzeDocument(content string) DocumentAnalysis {
	analysis := DocumentAnalysis{
		TotalChars: len(content),
	}

	// Count words
	words := strings.Fields(content)
	analysis.TotalWords = len(words)

	// Count paragraphs
	paragraphs := strings.Split(content, "\n\n")
	nonEmpty := 0
	for _, p := range paragraphs {
		if strings.TrimSpace(p) != "" {
			nonEmpty++
		}
	}
	analysis.TotalParagraphs = nonEmpty

	// Count sentences (rough approximation)
	sentenceCount := 0
	for _, c := range content {
		if c == '.' || c == '!' || c == '?' {
			sentenceCount++
		}
	}
	analysis.TotalSentences = sentenceCount

	// Calculate averages
	if analysis.TotalParagraphs > 0 {
		analysis.AvgWordsPerPara = float64(analysis.TotalWords) / float64(analysis.TotalParagraphs)
	}
	if analysis.TotalSentences > 0 {
		analysis.AvgWordsPerSent = float64(analysis.TotalWords) / float64(analysis.TotalSentences)
	}

	// Detect document type
	analysis.DocumentType = detectDocumentType(content, analysis)

	return analysis
}

// detectDocumentType determines the type of document based on its characteristics
func detectDocumentType(content string, analysis DocumentAnalysis) string {
	// Check for code indicators
	codeIndicators := []string{
		"func ", "function ", "def ", "class ", "import ", "package ",
		"var ", "let ", "const ", "return ", "if ", "for ", "while ",
		"{", "}", "//", "/*", "*/", "->", "=>",
	}
	codeScore := 0
	for _, indicator := range codeIndicators {
		codeScore += strings.Count(content, indicator)
	}
	codeRatio := float64(codeScore) / float64(analysis.TotalWords+1)
	if codeRatio > 0.05 {
		return "code"
	}

	// Check for list indicators
	listIndicators := []string{
		"\n- ", "\n* ", "\n1.", "\n2.", "\n3.", "\n| ", "\t",
	}
	listScore := 0
	for _, indicator := range listIndicators {
		listScore += strings.Count(content, indicator)
	}
	listRatio := float64(listScore) / float64(analysis.TotalParagraphs+1)
	if listRatio > 0.3 {
		return "list"
	}

	// Check for technical document indicators
	technicalIndicators := []string{
		"##", "###", "API", "endpoint", "parameter", "configuration",
		"install", "usage", "example", "documentation", "reference",
	}
	techScore := 0
	contentLower := strings.ToLower(content)
	for _, indicator := range technicalIndicators {
		techScore += strings.Count(contentLower, strings.ToLower(indicator))
	}
	techRatio := float64(techScore) / float64(analysis.TotalParagraphs+1)
	if techRatio > 0.2 {
		return "technical"
	}

	// Check for prose (longer sentences, narrative structure)
	if analysis.AvgWordsPerSent > 12 && analysis.AvgWordsPerPara > 40 {
		return "prose"
	}

	return "mixed"
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
// Groups chunks by their source document to avoid confusion
func (rc *RAGContext) FormatContext() string {
	if len(rc.Results) == 0 {
		rc.Context = ""
		return ""
	}

	// Group chunks by document
	docChunks := make(map[string][]SearchResult)
	docOrder := []string{} // Preserve order of first appearance

	for _, result := range rc.Results {
		docID := result.DocumentID
		if _, exists := docChunks[docID]; !exists {
			docOrder = append(docOrder, docID)
		}
		docChunks[docID] = append(docChunks[docID], result)
	}

	var sb strings.Builder
	sb.WriteString("<knowledge_base>\n")
	sb.WriteString("The following information was retrieved from the user's knowledge base and may be relevant:\n\n")

	for i, docID := range docOrder {
		chunks := docChunks[docID]
		docName := chunks[0].DocName

		// Calculate average relevance for the document
		var totalScore float32
		for _, c := range chunks {
			totalScore += c.Score
		}
		avgScore := totalScore / float32(len(chunks))

		sb.WriteString(fmt.Sprintf("[Source %d: %s | Relevance: %.0f%%]\n",
			i+1, docName, avgScore*100))

		// Combine chunks from the same document
		for j, chunk := range chunks {
			if len(chunks) > 1 {
				sb.WriteString(fmt.Sprintf("--- Section %d ---\n", j+1))
			}
			sb.WriteString(chunk.Chunk.Content)
			sb.WriteString("\n")
		}
		sb.WriteString("\n")
	}

	sb.WriteString("</knowledge_base>\n\n")

	// Add citation references section
	sb.WriteString("Citations:\n")
	for i, docID := range docOrder {
		chunks := docChunks[docID]
		docName := chunks[0].DocName
		metadata := chunks[0].Metadata

		sb.WriteString(fmt.Sprintf("[%d] %s", i+1, docName))
		if metadata != nil {
			if url, ok := metadata["source_url"]; ok && url != "" {
				sb.WriteString(fmt.Sprintf(" <%s>", url))
			}
			if author, ok := metadata["author"]; ok && author != "" {
				sb.WriteString(fmt.Sprintf(" by %s", author))
			}
		}
		sb.WriteString("\n")
	}
	sb.WriteString("\n")

	sb.WriteString("Instructions: Use the knowledge base above to inform your response. ")
	sb.WriteString("Cite sources using [N] format when referencing specific information. ")
	sb.WriteString("If the knowledge base doesn't contain relevant information, say so and answer based on your general knowledge.\n\n")

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

// UniqueDocumentCount returns the number of unique documents in the results
func (rc *RAGContext) UniqueDocumentCount() int {
	seen := make(map[string]bool)
	for _, r := range rc.Results {
		seen[r.DocumentID] = true
	}
	return len(seen)
}
