package rag

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"
)

// DocumentParser handles parsing of various document formats
type DocumentParser struct {
	maxFileSize      int64 // Maximum file size in bytes
	maxExtractedSize int64
	pdfWorker        string // Trusted app-local worker, never supplied by a document.
	pdfCacheDir      string // Compiled engine code only; no document data.
	ocrWorker        string // Trusted packaged OCR worker, never from document input.
	ocrLanguage      []byte // Explicitly configured, digest-verified offline language data.
	ocrSetupErr      error
}

// NewDocumentParser creates a new document parser
func NewDocumentParser() *DocumentParser {
	worker := bundledPDFWorker()
	cache := ""
	if worker != "" {
		cache = pdfWorkerCacheDir()
	}
	return &DocumentParser{
		maxFileSize:      100 * 1024 * 1024, // 100MB default limit
		maxExtractedSize: 16 * 1024 * 1024,
		pdfWorker:        worker,
		pdfCacheDir:      cache,
		ocrWorker:        bundledDocumentWorker("offgrid-ocr-worker"),
	}
}

// ParseResult contains the extracted content and metadata
type ParseResult struct {
	Content     string
	ContentType string
	Metadata    map[string]string
	PageCount   int
	WordCount   int
	SourceUnits []SourceUnit        `json:"source_units,omitempty"`
	State       ExtractionState     `json:"state,omitempty"`
	Warnings    []ExtractionWarning `json:"warnings,omitempty"`
}

// Parse attempts to parse a document based on its extension
func (p *DocumentParser) Parse(content []byte, filename string, ext string) (*ParseResult, error) {
	return p.ParseContext(context.Background(), content, filename, ext)
}

// ParseContext keeps cancellation attached to external extraction. The legacy
// Parse entry point remains compatible; both enforce input and output bounds.
func (p *DocumentParser) ParseContext(ctx context.Context, content []byte, filename string, ext string) (result *ParseResult, err error) {
	release, err := sharedExtractionScheduler.acquire(ctx, extractionQueueTimeout)
	if err != nil {
		return nil, err
	}
	defer release()
	return p.parseContext(ctx, content, filename, ext)
}

// parseContext is called only while an extraction slot is owned. Durable imports
// acquire before loading their saved bytes; ordinary callers use ParseContext.
func (p *DocumentParser) parseContext(ctx context.Context, content []byte, filename string, ext string) (result *ParseResult, err error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	defer func() {
		if ctx.Err() != nil {
			result, err = nil, ctx.Err()
		}
		if err == nil && result != nil && int64(len(result.Content)) > p.maxExtractedSize {
			result, err = nil, &ParseError{Code: "document_limit", Message: "Extracted document text exceeds the supported limit."}
		}
		if err != nil {
			err = classifyParseError(err)
		} else {
			bindSourceUnits(result, content)
		}
	}()
	if int64(len(content)) > p.maxFileSize {
		return nil, &ParseError{Code: "document_limit", Message: fmt.Sprintf("File size exceeds maximum allowed (%d MB).", p.maxFileSize/(1024*1024))}
	}

	ext = strings.ToLower(ext)

	switch ext {
	// Reliable text-based formats
	case ".txt", ".md", ".markdown":
		return p.ParsePlainText(content)
	case ".docx":
		return p.ParseDOCX(content)
	case ".html", ".htm":
		return p.ParseHTML(content)
	case ".json":
		return p.ParseJSON(content)
	case ".csv":
		return p.ParseCSV(content)
	case ".xml":
		return p.ParseXML(content)
	case ".rtf":
		return p.ParseRTF(content)
	// Code files
	case ".go", ".py", ".js", ".ts", ".java", ".c", ".cpp", ".rs", ".rb", ".php":
		return p.ParseCode(content, ext)
	// Document formats
	case ".pdf":
		return p.parsePDF(ctx, content)
	case ".xls":
		return nil, &ParseError{Code: "document_unsupported_format", Message: "The legacy XLS format requires a separate converter; export this workbook as XLSX or CSV before importing."}
	case ".doc", ".ppt", ".odt", ".ods", ".odp", ".eml", ".msg", ".mbox", ".png", ".jpg", ".jpeg", ".tif", ".tiff":
		return nil, &ParseError{Code: "document_unsupported_format", Message: "This format requires a dedicated local document adapter. Export its text to a supported format before importing."}
	case ".xlsx":
		return p.ParseXLSX(content)
	case ".pptx":
		return p.ParsePPTX(content)
	default:
		// Try to parse as plain text if valid UTF-8
		if utf8.Valid(content) {
			return p.ParsePlainText(content)
		}
		return nil, &ParseError{Code: "document_unsupported_format", Message: fmt.Sprintf("Unsupported file format: %s", ext)}
	}
}

// ParsePlainText handles plain text files
func (p *DocumentParser) ParsePlainText(content []byte) (*ParseResult, error) {
	if !utf8.Valid(content) {
		return nil, &ParseError{Code: "document_invalid", Message: "This text file is not valid UTF-8. Export it as UTF-8 before importing."}
	}
	text := string(content)
	return &ParseResult{
		Content:     text,
		ContentType: "text/plain",
		Metadata: map[string]string{
			"format": "text",
		},
		WordCount: countWords(text),
	}, nil
}

// ParseHTML extracts text from HTML
func (p *DocumentParser) ParseHTML(content []byte) (*ParseResult, error) {
	if !utf8.Valid(content) {
		return nil, invalidDocumentEncoding()
	}
	text := stripHTMLTags(string(content))
	text = cleanExtractedText(text)

	return &ParseResult{
		Content:     text,
		ContentType: "text/html",
		Metadata: map[string]string{
			"format": "html",
		},
		WordCount: countWords(text),
	}, nil
}

// ParseJSON formats JSON for readability
func (p *DocumentParser) ParseJSON(content []byte) (*ParseResult, error) {
	if !utf8.Valid(content) {
		return nil, invalidDocumentEncoding()
	}
	// Format syntax without decoding into maps/floats: preserve large identifiers,
	// decimal spelling, member order and even duplicate members from the source.
	var pretty bytes.Buffer
	if err := json.Indent(&pretty, content, "", "  "); err != nil {
		return nil, fmt.Errorf("invalid JSON: %w", err)
	}

	return &ParseResult{
		Content:     pretty.String(),
		ContentType: "application/json",
		Metadata: map[string]string{
			"format": "json",
		},
		WordCount: countWords(pretty.String()),
	}, nil
}

// ParseCSV handles CSV files
func (p *DocumentParser) ParseCSV(content []byte) (*ParseResult, error) {
	if !utf8.Valid(content) {
		return nil, invalidDocumentEncoding()
	}
	return &ParseResult{
		Content:     string(content),
		ContentType: "text/csv",
		Metadata: map[string]string{
			"format": "csv",
		},
		WordCount: countWords(string(content)),
	}, nil
}

// ParseXML extracts text from XML
func (p *DocumentParser) ParseXML(content []byte) (*ParseResult, error) {
	text, err := extractXMLDocument(content, p.maxExtractedSize)
	if err != nil {
		return nil, err
	}
	return &ParseResult{
		Content:     text,
		ContentType: "application/xml",
		Metadata: map[string]string{
			"format": "xml",
		},
		WordCount: countWords(text),
	}, nil
}

// ParseRTF extracts text from RTF
func (p *DocumentParser) ParseRTF(content []byte) (*ParseResult, error) {
	text := extractRTFText(string(content))
	return &ParseResult{
		Content:     text,
		ContentType: "application/rtf",
		Metadata: map[string]string{
			"format": "rtf",
		},
		WordCount: countWords(text),
	}, nil
}

// ParseCode handles source code files with syntax awareness
func (p *DocumentParser) ParseCode(content []byte, ext string) (*ParseResult, error) {
	if !utf8.Valid(content) {
		return nil, invalidDocumentEncoding()
	}
	text := string(content)
	language := strings.TrimPrefix(ext, ".")

	// Add language metadata for context
	return &ParseResult{
		Content:     text,
		ContentType: "text/x-" + language,
		Metadata: map[string]string{
			"format":   "code",
			"language": language,
		},
		WordCount: countWords(text),
	}, nil
}

// Helper functions

// extractRTFText extracts text from RTF content
func extractRTFText(rtf string) string {
	// Remove RTF control words and groups
	controlRegex := regexp.MustCompile(`\\[a-z]+\d*\s?`)
	text := controlRegex.ReplaceAllString(rtf, "")

	// Remove braces
	text = strings.ReplaceAll(text, "{", "")
	text = strings.ReplaceAll(text, "}", "")

	return cleanExtractedText(text)
}

// cleanExtractedText cleans up extracted text
func cleanExtractedText(text string) string {
	// First, filter out non-printable and garbled characters
	var cleaned strings.Builder
	garbledCount := 0
	totalCount := 0

	for _, r := range text {
		totalCount++
		// Allow: printable ASCII, common Unicode (letters, digits, punctuation)
		// Also allow common whitespace and newlines
		if r == '\n' || r == '\r' || r == '\t' || r == ' ' {
			cleaned.WriteRune(r)
		} else if r != utf8.RuneError && (unicode.IsPrint(r) || r == '\u200c' || r == '\u200d') {
			// Unicode categories preserve all scripts, combining marks and symbols.
			// A language-range allowlist silently erased valid source documents.
			cleaned.WriteRune(r)
		} else {
			garbledCount++
		}
	}

	result := cleaned.String()

	// If more than 50% of characters were garbled, the extraction likely failed
	if totalCount > 10 && float64(garbledCount)/float64(totalCount) > 0.5 {
		// Return empty to signal extraction failure
		return ""
	}

	// Normalize whitespace
	wsRegex := regexp.MustCompile(`[ \t]+`)
	result = wsRegex.ReplaceAllString(result, " ")

	// Normalize line breaks
	nlRegex := regexp.MustCompile(`\n{3,}`)
	result = nlRegex.ReplaceAllString(result, "\n\n")

	// Decode HTML entities
	result = decodeHTMLEntities(result)

	return strings.TrimSpace(result)
}

// decodeHTMLEntities decodes common HTML entities
func decodeHTMLEntities(s string) string {
	replacer := strings.NewReplacer(
		"&nbsp;", " ",
		"&amp;", "&",
		"&lt;", "<",
		"&gt;", ">",
		"&quot;", "\"",
		"&#39;", "'",
		"&apos;", "'",
		"&#x27;", "'",
		"&mdash;", "—",
		"&ndash;", "–",
		"&hellip;", "...",
		"&copy;", "©",
		"&reg;", "®",
		"&trade;", "™",
	)
	return replacer.Replace(s)
}

// countWords counts words in text
func countWords(text string) int {
	words := strings.Fields(text)
	return len(words)
}

// SupportedExtensions returns all supported file extensions
// Includes formats that can be parsed with built-in or optional external tools
func SupportedExtensions() []string {
	return []string{
		// Documents
		".docx", ".pdf", ".pptx", ".xlsx", ".xls", ".txt", ".md", ".markdown", ".rtf",
		// Data
		".json", ".csv", ".xml",
		// Web
		".html", ".htm",
		// Code
		".go", ".py", ".js", ".ts", ".java", ".c", ".cpp", ".rs", ".rb", ".php",
	}
}

// IsSupportedExtension checks if a file extension is supported
func IsSupportedExtension(ext string) bool {
	ext = strings.ToLower(ext)
	for _, supported := range SupportedExtensions() {
		if ext == supported {
			return true
		}
	}
	return false
}
