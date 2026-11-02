package rag

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// IngestFile ingests a file from the filesystem
// Now supports PDF, DOCX, XLSX, PPTX, and many more formats
func (e *Engine) IngestFile(ctx context.Context, filePath string, metadata map[string]string) (*Document, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	parser := e.DocumentParser()
	file, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() {
		return nil, &ParseError{Code: "document_invalid", Message: "Choose a regular document file."}
	}
	if info.Size() > parser.maxFileSize {
		return nil, &ParseError{Code: "document_limit", Message: "Document file exceeds the supported input limit."}
	}
	// Bound the read even if another process grows the file after Stat.
	content, err := readParserBytes(file, parser.maxFileSize)
	if err != nil {
		return nil, err
	}

	// Detect content type
	ext := strings.ToLower(filepath.Ext(filePath))

	// Use the document parser for advanced formats
	result, err := parser.ParseContext(ctx, content, filepath.Base(filePath), ext)
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

	return e.IngestParsed(ctx, filepath.Base(filePath), result, metadata)
}

// IngestReader ingests content from an io.Reader
func (e *Engine) IngestReader(ctx context.Context, name string, reader io.Reader, metadata map[string]string) (*Document, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	parser := e.DocumentParser()
	content, err := readParserBytes(reader, parser.maxFileSize)
	if err != nil {
		return nil, fmt.Errorf("failed to read content: %w", err)
	}
	result, err := parser.ParseContext(ctx, content, name, ".txt")
	if err != nil {
		return nil, err
	}
	return e.IngestText(ctx, name, result.Content, metadata)
}
