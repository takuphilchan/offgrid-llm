package rag

import (
	"strings"
	"unicode"
)

// Chunker handles splitting documents into chunks
type Chunker struct {
	options ChunkingOptions
}

// NewChunker creates a new chunker with the given options
func NewChunker(opts ChunkingOptions) *Chunker {
	if opts.ChunkSize <= 0 {
		opts.ChunkSize = 1000
	}
	if opts.ChunkOverlap < 0 {
		opts.ChunkOverlap = 0
	}
	if opts.ChunkOverlap >= opts.ChunkSize {
		opts.ChunkOverlap = opts.ChunkSize / 5
	}
	if opts.Separator == "" {
		opts.Separator = "\n\n"
	}
	return &Chunker{options: opts}
}

// ChunkText splits text into overlapping chunks
func (c *Chunker) ChunkText(documentID, text string) []*Chunk {
	// Clean the text
	text = strings.TrimSpace(text)
	if text == "" {
		return nil
	}

	// First, try to split by paragraphs
	paragraphs := c.splitByParagraphs(text)

	// Then merge/split paragraphs to target chunk size
	chunks := c.mergeToChunks(documentID, paragraphs, text)

	return chunks
}

// splitByParagraphs splits text into paragraphs
func (c *Chunker) splitByParagraphs(text string) []string {
	// Split by double newlines (paragraphs)
	parts := strings.Split(text, c.options.Separator)

	// Filter empty parts and trim
	result := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			result = append(result, part)
		}
	}

	// If no paragraphs found, try single newlines
	if len(result) <= 1 && len(text) > c.options.ChunkSize {
		parts = strings.Split(text, "\n")
		result = make([]string, 0, len(parts))
		for _, part := range parts {
			part = strings.TrimSpace(part)
			if part != "" {
				result = append(result, part)
			}
		}
	}

	return result
}

// mergeToChunks merges paragraphs into chunks of target size with overlap
func (c *Chunker) mergeToChunks(documentID string, paragraphs []string, originalText string) []*Chunk {
	if len(paragraphs) == 0 {
		return nil
	}

	chunks := make([]*Chunk, 0)
	currentChunk := strings.Builder{}
	chunkIndex := 0
	startChar := 0

	for i, para := range paragraphs {
		// Check if adding this paragraph would exceed chunk size
		potentialSize := currentChunk.Len() + len(para)
		if currentChunk.Len() > 0 {
			potentialSize += 2 // For "\n\n" separator
		}

		if potentialSize > c.options.ChunkSize && currentChunk.Len() > 0 {
			// Save current chunk
			content := currentChunk.String()
			endChar := startChar + len(content)

			chunk := &Chunk{
				ID:         GenerateChunkID(documentID, chunkIndex),
				DocumentID: documentID,
				Content:    content,
				Index:      chunkIndex,
				StartChar:  startChar,
				EndChar:    endChar,
			}
			chunks = append(chunks, chunk)
			chunkIndex++

			// Start new chunk with overlap
			overlap := c.getOverlapText(content)
			currentChunk.Reset()
			if overlap != "" {
				currentChunk.WriteString(overlap)
				startChar = endChar - len(overlap)
			} else {
				startChar = endChar
			}
		}

		// Add paragraph to current chunk
		if currentChunk.Len() > 0 {
			currentChunk.WriteString("\n\n")
		}
		currentChunk.WriteString(para)

		// If this is a very long paragraph, split it further
		if currentChunk.Len() > c.options.ChunkSize*2 {
			content := currentChunk.String()
			subChunks := c.splitLongText(documentID, content, chunkIndex, startChar)
			chunks = append(chunks, subChunks...)
			chunkIndex += len(subChunks)
			if len(subChunks) > 0 {
				lastSubChunk := subChunks[len(subChunks)-1]
				startChar = lastSubChunk.EndChar
			}
			currentChunk.Reset()
		}

		// Handle last paragraph
		if i == len(paragraphs)-1 && currentChunk.Len() > 0 {
			content := currentChunk.String()
			chunk := &Chunk{
				ID:         GenerateChunkID(documentID, chunkIndex),
				DocumentID: documentID,
				Content:    content,
				Index:      chunkIndex,
				StartChar:  startChar,
				EndChar:    startChar + len(content),
			}
			chunks = append(chunks, chunk)
		}
	}

	return chunks
}

// splitLongText splits text that's too long into smaller chunks
func (c *Chunker) splitLongText(documentID, text string, startIndex, startChar int) []*Chunk {
	chunks := make([]*Chunk, 0)

	// Try to split at sentence boundaries
	sentences := c.splitBySentences(text)

	currentChunk := strings.Builder{}
	chunkIndex := startIndex
	charOffset := startChar

	for i, sentence := range sentences {
		potentialSize := currentChunk.Len() + len(sentence)
		if currentChunk.Len() > 0 {
			potentialSize += 1 // For space
		}

		if potentialSize > c.options.ChunkSize && currentChunk.Len() > 0 {
			content := currentChunk.String()
			chunk := &Chunk{
				ID:         GenerateChunkID(documentID, chunkIndex),
				DocumentID: documentID,
				Content:    content,
				Index:      chunkIndex,
				StartChar:  charOffset,
				EndChar:    charOffset + len(content),
			}
			chunks = append(chunks, chunk)
			chunkIndex++
			charOffset += len(content)

			// Start new chunk with overlap
			overlap := c.getOverlapText(content)
			currentChunk.Reset()
			if overlap != "" {
				currentChunk.WriteString(overlap)
				charOffset -= len(overlap)
			}
		}

		if currentChunk.Len() > 0 {
			currentChunk.WriteString(" ")
		}
		currentChunk.WriteString(sentence)

		if i == len(sentences)-1 && currentChunk.Len() > 0 {
			content := currentChunk.String()
			chunk := &Chunk{
				ID:         GenerateChunkID(documentID, chunkIndex),
				DocumentID: documentID,
				Content:    content,
				Index:      chunkIndex,
				StartChar:  charOffset,
				EndChar:    charOffset + len(content),
			}
			chunks = append(chunks, chunk)
		}
	}

	return chunks
}

// splitBySentences attempts to split text by sentence boundaries
func (c *Chunker) splitBySentences(text string) []string {
	var sentences []string
	var current strings.Builder

	runes := []rune(text)
	for i := 0; i < len(runes); i++ {
		current.WriteRune(runes[i])

		// Check for sentence endings
		if runes[i] == '.' || runes[i] == '!' || runes[i] == '?' {
			// Look ahead for space or end
			if i+1 >= len(runes) || unicode.IsSpace(runes[i+1]) {
				sentence := strings.TrimSpace(current.String())
				if sentence != "" {
					sentences = append(sentences, sentence)
				}
				current.Reset()
			}
		}
	}

	// Don't forget remaining text
	if current.Len() > 0 {
		sentence := strings.TrimSpace(current.String())
		if sentence != "" {
			sentences = append(sentences, sentence)
		}
	}

	return sentences
}

// getOverlapText gets the last N characters for overlap
func (c *Chunker) getOverlapText(text string) string {
	if c.options.ChunkOverlap <= 0 || len(text) <= c.options.ChunkOverlap {
		return ""
	}

	// Try to find a good break point (sentence or word boundary)
	overlapStart := len(text) - c.options.ChunkOverlap

	// Look for sentence boundary
	for i := overlapStart; i < len(text); i++ {
		if text[i] == '.' || text[i] == '!' || text[i] == '?' {
			if i+1 < len(text) && (text[i+1] == ' ' || text[i+1] == '\n') {
				return strings.TrimSpace(text[i+2:])
			}
		}
	}

	// Look for word boundary
	for i := overlapStart; i < len(text); i++ {
		if text[i] == ' ' {
			return strings.TrimSpace(text[i+1:])
		}
	}

	return text[overlapStart:]
}
