package rag

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
	"regexp"
	"strings"
	"unicode/utf8"
)

// DocumentParser handles parsing of various document formats
type DocumentParser struct {
	maxFileSize int64 // Maximum file size in bytes
}

// NewDocumentParser creates a new document parser
func NewDocumentParser() *DocumentParser {
	return &DocumentParser{
		maxFileSize: 100 * 1024 * 1024, // 100MB default limit
	}
}

// ParseResult contains the extracted content and metadata
type ParseResult struct {
	Content     string
	ContentType string
	Metadata    map[string]string
	PageCount   int
	WordCount   int
}

// Parse attempts to parse a document based on its extension
func (p *DocumentParser) Parse(content []byte, filename string, ext string) (*ParseResult, error) {
	if int64(len(content)) > p.maxFileSize {
		return nil, fmt.Errorf("file size exceeds maximum allowed (%d MB)", p.maxFileSize/(1024*1024))
	}

	ext = strings.ToLower(ext)

	switch ext {
	case ".pdf":
		return p.ParsePDF(content)
	case ".docx":
		return p.ParseDOCX(content)
	case ".xlsx":
		return p.ParseXLSX(content)
	case ".pptx":
		return p.ParsePPTX(content)
	case ".txt", ".md", ".markdown":
		return p.ParsePlainText(content)
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
	case ".go", ".py", ".js", ".ts", ".java", ".c", ".cpp", ".rs", ".rb", ".php":
		return p.ParseCode(content, ext)
	default:
		// Try to parse as plain text if valid UTF-8
		if utf8.Valid(content) {
			return p.ParsePlainText(content)
		}
		return nil, fmt.Errorf("unsupported file format: %s", ext)
	}
}

// ParsePDF extracts text from a PDF file
// This is a basic implementation that extracts text streams
func (p *DocumentParser) ParsePDF(content []byte) (*ParseResult, error) {
	// Check PDF signature
	if len(content) < 5 || string(content[:5]) != "%PDF-" {
		return nil, fmt.Errorf("invalid PDF file: missing PDF header")
	}

	var textBuilder strings.Builder
	pageCount := 0

	// Simple PDF text extraction - look for text streams
	// This handles basic PDFs; complex PDFs may need external tools
	contentStr := string(content)

	// Find all stream objects
	streamRegex := regexp.MustCompile(`stream\s*([\s\S]*?)\s*endstream`)
	matches := streamRegex.FindAllStringSubmatch(contentStr, -1)

	for _, match := range matches {
		if len(match) > 1 {
			streamContent := match[1]
			// Extract text from stream (look for Tj and TJ operators)
			text := extractPDFText(streamContent)
			if text != "" {
				textBuilder.WriteString(text)
				textBuilder.WriteString("\n")
			}
		}
	}

	// Also try to extract BT/ET text blocks
	btRegex := regexp.MustCompile(`BT\s*([\s\S]*?)\s*ET`)
	btMatches := btRegex.FindAllStringSubmatch(contentStr, -1)
	for _, match := range btMatches {
		if len(match) > 1 {
			text := extractPDFTextFromBlock(match[1])
			if text != "" {
				textBuilder.WriteString(text)
				textBuilder.WriteString("\n")
			}
		}
	}

	// Count pages
	pageRegex := regexp.MustCompile(`/Type\s*/Page[^s]`)
	pageMatches := pageRegex.FindAllString(contentStr, -1)
	pageCount = len(pageMatches)

	extractedText := strings.TrimSpace(textBuilder.String())
	if extractedText == "" {
		return nil, fmt.Errorf("no text content extracted from PDF (may be scanned/image-based)")
	}

	// Clean up the text
	extractedText = cleanExtractedText(extractedText)

	return &ParseResult{
		Content:     extractedText,
		ContentType: "application/pdf",
		Metadata: map[string]string{
			"format":     "pdf",
			"page_count": fmt.Sprintf("%d", pageCount),
		},
		PageCount: pageCount,
		WordCount: countWords(extractedText),
	}, nil
}

// extractPDFText extracts text from a PDF stream
func extractPDFText(stream string) string {
	var text strings.Builder

	// Look for text strings in parentheses (Tj operator)
	tjRegex := regexp.MustCompile(`\(([^)]*)\)\s*Tj`)
	matches := tjRegex.FindAllStringSubmatch(stream, -1)
	for _, m := range matches {
		if len(m) > 1 {
			text.WriteString(decodePDFString(m[1]))
			text.WriteString(" ")
		}
	}

	// Look for TJ arrays
	tjArrayRegex := regexp.MustCompile(`\[(.*?)\]\s*TJ`)
	arrayMatches := tjArrayRegex.FindAllStringSubmatch(stream, -1)
	for _, m := range arrayMatches {
		if len(m) > 1 {
			text.WriteString(extractTJArrayText(m[1]))
			text.WriteString(" ")
		}
	}

	return text.String()
}

// extractPDFTextFromBlock extracts text from a BT/ET block
func extractPDFTextFromBlock(block string) string {
	var text strings.Builder

	// Extract strings in parentheses
	stringRegex := regexp.MustCompile(`\(([^)]*)\)`)
	matches := stringRegex.FindAllStringSubmatch(block, -1)
	for _, m := range matches {
		if len(m) > 1 {
			decoded := decodePDFString(m[1])
			if decoded != "" {
				text.WriteString(decoded)
				text.WriteString(" ")
			}
		}
	}

	return text.String()
}

// extractTJArrayText extracts text from a TJ array
func extractTJArrayText(array string) string {
	var text strings.Builder
	stringRegex := regexp.MustCompile(`\(([^)]*)\)`)
	matches := stringRegex.FindAllStringSubmatch(array, -1)
	for _, m := range matches {
		if len(m) > 1 {
			text.WriteString(decodePDFString(m[1]))
		}
	}
	return text.String()
}

// decodePDFString decodes escape sequences in PDF strings
func decodePDFString(s string) string {
	// Handle common escape sequences
	s = strings.ReplaceAll(s, "\\n", "\n")
	s = strings.ReplaceAll(s, "\\r", "\r")
	s = strings.ReplaceAll(s, "\\t", "\t")
	s = strings.ReplaceAll(s, "\\(", "(")
	s = strings.ReplaceAll(s, "\\)", ")")
	s = strings.ReplaceAll(s, "\\\\", "\\")

	// Filter out non-printable characters
	var result strings.Builder
	for _, r := range s {
		if r >= 32 && r < 127 || r == '\n' || r == '\r' || r == '\t' || r > 127 {
			result.WriteRune(r)
		}
	}

	return result.String()
}

// ParseDOCX extracts text from a DOCX file
func (p *DocumentParser) ParseDOCX(content []byte) (*ParseResult, error) {
	reader, err := zip.NewReader(bytes.NewReader(content), int64(len(content)))
	if err != nil {
		return nil, fmt.Errorf("failed to open DOCX file: %w", err)
	}

	var textBuilder strings.Builder
	var pageCount int

	// Read document.xml
	for _, file := range reader.File {
		if file.Name == "word/document.xml" {
			rc, err := file.Open()
			if err != nil {
				return nil, fmt.Errorf("failed to open document.xml: %w", err)
			}
			defer rc.Close()

			xmlContent, err := io.ReadAll(rc)
			if err != nil {
				return nil, fmt.Errorf("failed to read document.xml: %w", err)
			}

			text, pages := extractDOCXText(xmlContent)
			textBuilder.WriteString(text)
			pageCount = pages
		}
	}

	extractedText := strings.TrimSpace(textBuilder.String())
	if extractedText == "" {
		return nil, fmt.Errorf("no text content extracted from DOCX")
	}

	return &ParseResult{
		Content:     extractedText,
		ContentType: "application/vnd.openxmlformats-officedocument.wordprocessingml.document",
		Metadata: map[string]string{
			"format":     "docx",
			"page_count": fmt.Sprintf("%d", pageCount),
		},
		PageCount: pageCount,
		WordCount: countWords(extractedText),
	}, nil
}

// DOCX XML structures
type docxDocument struct {
	Body docxBody `xml:"body"`
}

type docxBody struct {
	Paragraphs []docxParagraph `xml:"p"`
	Tables     []docxTable     `xml:"tbl"`
}

type docxParagraph struct {
	Runs []docxRun `xml:"r"`
}

type docxRun struct {
	Text  []docxText  `xml:"t"`
	Tab   []struct{}  `xml:"tab"`
	Break []docxBreak `xml:"br"`
}

type docxText struct {
	Content string `xml:",chardata"`
	Space   string `xml:"space,attr"`
}

type docxBreak struct {
	Type string `xml:"type,attr"`
}

type docxTable struct {
	Rows []docxTableRow `xml:"tr"`
}

type docxTableRow struct {
	Cells []docxTableCell `xml:"tc"`
}

type docxTableCell struct {
	Paragraphs []docxParagraph `xml:"p"`
}

// extractDOCXText extracts text from DOCX XML
func extractDOCXText(xmlContent []byte) (string, int) {
	var doc docxDocument
	var text strings.Builder
	pageCount := 1

	// Handle the namespace
	xmlContent = removeNamespace(xmlContent)

	if err := xml.Unmarshal(xmlContent, &doc); err != nil {
		// Fallback: simple regex extraction
		return extractTextFromXML(string(xmlContent)), 1
	}

	for _, para := range doc.Body.Paragraphs {
		paraText := extractDOCXParagraph(para)
		if paraText != "" {
			text.WriteString(paraText)
			text.WriteString("\n\n")
		}
	}

	// Extract table text
	for _, table := range doc.Body.Tables {
		for _, row := range table.Rows {
			var rowTexts []string
			for _, cell := range row.Cells {
				var cellText strings.Builder
				for _, para := range cell.Paragraphs {
					cellText.WriteString(extractDOCXParagraph(para))
					cellText.WriteString(" ")
				}
				rowTexts = append(rowTexts, strings.TrimSpace(cellText.String()))
			}
			text.WriteString(strings.Join(rowTexts, " | "))
			text.WriteString("\n")
		}
		text.WriteString("\n")
	}

	return text.String(), pageCount
}

func extractDOCXParagraph(para docxParagraph) string {
	var text strings.Builder
	for _, run := range para.Runs {
		for _, t := range run.Text {
			text.WriteString(t.Content)
		}
		for range run.Tab {
			text.WriteString("\t")
		}
		for _, br := range run.Break {
			if br.Type == "page" {
				text.WriteString("\n\n--- PAGE BREAK ---\n\n")
			} else {
				text.WriteString("\n")
			}
		}
	}
	return strings.TrimSpace(text.String())
}

// ParseXLSX extracts text from an XLSX file
func (p *DocumentParser) ParseXLSX(content []byte) (*ParseResult, error) {
	reader, err := zip.NewReader(bytes.NewReader(content), int64(len(content)))
	if err != nil {
		return nil, fmt.Errorf("failed to open XLSX file: %w", err)
	}

	// Read shared strings
	sharedStrings := make([]string, 0)
	for _, file := range reader.File {
		if file.Name == "xl/sharedStrings.xml" {
			rc, err := file.Open()
			if err != nil {
				continue
			}
			data, _ := io.ReadAll(rc)
			rc.Close()
			sharedStrings = extractXLSXSharedStrings(data)
		}
	}

	var textBuilder strings.Builder
	sheetCount := 0

	// Read worksheets
	for _, file := range reader.File {
		if strings.HasPrefix(file.Name, "xl/worksheets/sheet") && strings.HasSuffix(file.Name, ".xml") {
			sheetCount++
			rc, err := file.Open()
			if err != nil {
				continue
			}
			data, _ := io.ReadAll(rc)
			rc.Close()

			sheetText := extractXLSXSheet(data, sharedStrings)
			if sheetText != "" {
				textBuilder.WriteString(fmt.Sprintf("--- Sheet %d ---\n", sheetCount))
				textBuilder.WriteString(sheetText)
				textBuilder.WriteString("\n\n")
			}
		}
	}

	extractedText := strings.TrimSpace(textBuilder.String())
	if extractedText == "" {
		return nil, fmt.Errorf("no text content extracted from XLSX")
	}

	return &ParseResult{
		Content:     extractedText,
		ContentType: "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet",
		Metadata: map[string]string{
			"format":      "xlsx",
			"sheet_count": fmt.Sprintf("%d", sheetCount),
		},
		PageCount: sheetCount,
		WordCount: countWords(extractedText),
	}, nil
}

// XLSX shared strings XML structure
type xlsxSharedStrings struct {
	Strings []xlsxString `xml:"si"`
}

type xlsxString struct {
	Text     string     `xml:"t"`
	RichText []xlsxRich `xml:"r"`
}

type xlsxRich struct {
	Text string `xml:"t"`
}

func extractXLSXSharedStrings(data []byte) []string {
	data = removeNamespace(data)
	var sst xlsxSharedStrings
	if err := xml.Unmarshal(data, &sst); err != nil {
		return nil
	}

	strings := make([]string, len(sst.Strings))
	for i, s := range sst.Strings {
		if s.Text != "" {
			strings[i] = s.Text
		} else {
			// Rich text
			var text bytes.Buffer
			for _, r := range s.RichText {
				text.WriteString(r.Text)
			}
			strings[i] = text.String()
		}
	}
	return strings
}

// XLSX worksheet structure
type xlsxWorksheet struct {
	SheetData xlsxSheetData `xml:"sheetData"`
}

type xlsxSheetData struct {
	Rows []xlsxRow `xml:"row"`
}

type xlsxRow struct {
	Cells []xlsxCell `xml:"c"`
}

type xlsxCell struct {
	Type  string `xml:"t,attr"` // "s" for shared string
	Value string `xml:"v"`
}

func extractXLSXSheet(data []byte, sharedStrings []string) string {
	data = removeNamespace(data)
	var ws xlsxWorksheet
	if err := xml.Unmarshal(data, &ws); err != nil {
		return ""
	}

	var text strings.Builder
	for _, row := range ws.SheetData.Rows {
		var cells []string
		for _, cell := range row.Cells {
			value := cell.Value
			if cell.Type == "s" {
				// Shared string index
				var idx int
				fmt.Sscanf(value, "%d", &idx)
				if idx >= 0 && idx < len(sharedStrings) {
					value = sharedStrings[idx]
				}
			}
			cells = append(cells, value)
		}
		if len(cells) > 0 {
			text.WriteString(strings.Join(cells, " | "))
			text.WriteString("\n")
		}
	}
	return text.String()
}

// ParsePPTX extracts text from a PPTX file
func (p *DocumentParser) ParsePPTX(content []byte) (*ParseResult, error) {
	reader, err := zip.NewReader(bytes.NewReader(content), int64(len(content)))
	if err != nil {
		return nil, fmt.Errorf("failed to open PPTX file: %w", err)
	}

	var textBuilder strings.Builder
	slideCount := 0

	for _, file := range reader.File {
		if strings.HasPrefix(file.Name, "ppt/slides/slide") && strings.HasSuffix(file.Name, ".xml") {
			slideCount++
			rc, err := file.Open()
			if err != nil {
				continue
			}
			data, _ := io.ReadAll(rc)
			rc.Close()

			slideText := extractTextFromXML(string(data))
			if slideText != "" {
				textBuilder.WriteString(fmt.Sprintf("--- Slide %d ---\n", slideCount))
				textBuilder.WriteString(slideText)
				textBuilder.WriteString("\n\n")
			}
		}
	}

	extractedText := strings.TrimSpace(textBuilder.String())
	if extractedText == "" {
		return nil, fmt.Errorf("no text content extracted from PPTX")
	}

	return &ParseResult{
		Content:     extractedText,
		ContentType: "application/vnd.openxmlformats-officedocument.presentationml.presentation",
		Metadata: map[string]string{
			"format":      "pptx",
			"slide_count": fmt.Sprintf("%d", slideCount),
		},
		PageCount: slideCount,
		WordCount: countWords(extractedText),
	}, nil
}

// ParsePlainText handles plain text files
func (p *DocumentParser) ParsePlainText(content []byte) (*ParseResult, error) {
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
	var data interface{}
	if err := json.Unmarshal(content, &data); err != nil {
		return nil, fmt.Errorf("invalid JSON: %w", err)
	}

	pretty, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return nil, err
	}

	return &ParseResult{
		Content:     string(pretty),
		ContentType: "application/json",
		Metadata: map[string]string{
			"format": "json",
		},
		WordCount: countWords(string(pretty)),
	}, nil
}

// ParseCSV handles CSV files
func (p *DocumentParser) ParseCSV(content []byte) (*ParseResult, error) {
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
	text := extractTextFromXML(string(content))
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

// removeNamespace strips XML namespaces for easier parsing
func removeNamespace(data []byte) []byte {
	// Remove xmlns attributes
	nsRegex := regexp.MustCompile(`\s+xmlns[^=]*="[^"]*"`)
	data = nsRegex.ReplaceAll(data, []byte{})

	// Remove namespace prefixes from tags
	prefixRegex := regexp.MustCompile(`<(/?)(\w+):`)
	data = prefixRegex.ReplaceAll(data, []byte("<$1"))

	return data
}

// extractTextFromXML extracts text content from XML
func extractTextFromXML(xmlContent string) string {
	// Remove all tags and extract text
	tagRegex := regexp.MustCompile(`<[^>]+>`)
	text := tagRegex.ReplaceAllString(xmlContent, " ")
	return cleanExtractedText(text)
}

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
	// Normalize whitespace
	wsRegex := regexp.MustCompile(`[ \t]+`)
	text = wsRegex.ReplaceAllString(text, " ")

	// Normalize line breaks
	nlRegex := regexp.MustCompile(`\n{3,}`)
	text = nlRegex.ReplaceAllString(text, "\n\n")

	// Decode HTML entities
	text = decodeHTMLEntities(text)

	return strings.TrimSpace(text)
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
func SupportedExtensions() []string {
	return []string{
		".pdf", ".docx", ".xlsx", ".pptx",
		".txt", ".md", ".markdown",
		".html", ".htm",
		".json", ".csv", ".xml", ".rtf",
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
