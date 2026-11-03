package rag

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

const pdfWorkerRevision = "pdfium-8057-0001cb839e62425b-offgrid4"

// SourceUnit retains an actual engine-reported position and extraction state.
// A locator without its digest/revision is not a citation to a stable source.
type SourceUnit struct {
	Cells             []SourceCell `json:"cells,omitempty"` // Relative UTF-8 spans inside Text for worksheets.
	Kind              string       `json:"kind"`
	Index             int          `json:"index"`
	Name              string       `json:"name,omitempty"`
	Text              string       `json:"text"`
	State             string       `json:"state"`
	SourceSHA256      string       `json:"source_sha256"`
	ExtractorRevision string       `json:"extractor_revision"`
}

type pdfWorkerReceipt struct {
	Protocol int    `json:"protocol"`
	Revision string `json:"revision"`
	SHA256   string `json:"sha256"`
	Pages    []struct {
		Page  int    `json:"page"`
		Text  string `json:"text"`
		State string `json:"state"`
	} `json:"pages"`
	Error string `json:"error"`
}

// Independent of model admission: one untrusted PDF worker at a time, and a
// cancelled waiter cannot dispatch later. No inference lock is held here.
var pdfWorkerAdmission = make(chan struct{}, 1)

func pdfWorkerCacheDir() string {
	root, err := os.UserCacheDir()
	if err != nil || !filepath.IsAbs(root) {
		return ""
	}
	return filepath.Join(root, "offgrid", "document-code-cache", pdfWorkerRevision)
}

func bundledPDFWorker() string {
	return bundledDocumentWorker("offgrid-document-worker")
}

func bundledDocumentWorker(name string) string {
	exe, err := os.Executable()
	if err != nil {
		return ""
	}
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	path := filepath.Join(filepath.Dir(exe), name)
	info, err := os.Lstat(path)
	if err != nil || !info.Mode().IsRegular() {
		return ""
	}
	return path
}

func (p *DocumentParser) extractPDFWorker(ctx context.Context, content []byte) (*ParseResult, error) {
	if len(content) > 50<<20 {
		return nil, &ParseError{Code: "document_limit", Message: "PDF documents must be 50 MiB or smaller."}
	}
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	select {
	case pdfWorkerAdmission <- struct{}{}:
		defer func() { <-pdfWorkerAdmission }()
	case <-ctx.Done():
		return nil, ctx.Err()
	}
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}
	cmd := exec.CommandContext(ctx, p.pdfWorker)
	return p.runPDFWorker(ctx, cmd, content)
}

func (p *DocumentParser) runPDFWorker(ctx context.Context, cmd *exec.Cmd, content []byte) (*ParseResult, error) {
	output, err := p.runPDFWorkerOutput(ctx, cmd, content, 20<<20)
	if err != nil {
		return nil, err
	}
	return p.decodePDFWorker([]byte(output), content)
}

func (p *DocumentParser) runPDFWorkerOutput(ctx context.Context, cmd *exec.Cmd, content []byte, limit int64) (string, error) {
	configureParserProcess(cmd)
	cmd.Stdin = bytes.NewReader(content)
	cmd.WaitDelay = time.Second
	cmd.Env = []string{"GOMAXPROCS=1", "GOMEMLIMIT=256MiB"}
	if filepath.IsAbs(p.pdfCacheDir) {
		cmd.Env = append(cmd.Env, "OFFGRID_DOCUMENT_CODE_CACHE="+p.pdfCacheDir)
	}
	// Do not forward service credentials, proxy settings or the user's PATH.
	for _, name := range []string{"SystemRoot", "WINDIR", "TEMP", "TMP"} {
		if value := os.Getenv(name); value != "" {
			cmd.Env = append(cmd.Env, name+"="+value)
		}
	}
	output, err := runParserCommand(ctx, cmd, limit)
	if err != nil {
		if ctx.Err() != nil {
			return "", ctx.Err()
		}
		if isParseLimit(err) {
			return "", err
		}
		return "", &ParseError{Code: "document_parser_unavailable", Message: "The local PDF worker could not finish. No extracted text was accepted; retry after checking the installed parser runtime."}
	}
	return output, nil
}

func (p *DocumentParser) decodePDFWorker(output, source []byte) (*ParseResult, error) {
	invalid := func() (*ParseResult, error) {
		return nil, &ParseError{Code: "document_parser_protocol", Message: "The local PDF worker returned an invalid receipt. No extracted text was accepted."}
	}
	var receipt pdfWorkerReceipt
	decoder := json.NewDecoder(bytes.NewReader(output))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&receipt); err != nil {
		return invalid()
	}
	var trailing any
	if decoder.Decode(&trailing) != io.EOF {
		return invalid()
	}
	digest := sha256.Sum256(source)
	if receipt.Protocol != 1 || receipt.Revision != pdfWorkerRevision || receipt.SHA256 != hex.EncodeToString(digest[:]) {
		return invalid()
	}
	if receipt.Error != "" {
		if len(receipt.Pages) > 0 {
			return invalid()
		}
		messages := map[string]string{
			"document_invalid":            "This PDF is damaged or is not a valid PDF document.",
			"document_encrypted":          "This PDF requires a password. Export an unencrypted copy before importing.",
			"document_empty":              "This PDF contains no readable pages or text.",
			"document_limit":              "This PDF exceeds the local parser's page, memory or text limits.",
			"document_timeout":            "PDF extraction took too long. Try a smaller document.",
			"document_parser_unavailable": "The packaged PDF runtime could not start. Check the installed runtime.",
		}
		message, ok := messages[receipt.Error]
		if !ok {
			return invalid()
		}
		return nil, &ParseError{Code: receipt.Error, Message: message}
	}
	if len(receipt.Pages) < 1 || len(receipt.Pages) > 1000 {
		return invalid()
	}
	result := &ParseResult{ContentType: "application/pdf", State: ExtractionComplete, PageCount: len(receipt.Pages), Metadata: map[string]string{
		"format": "pdf", "extractor": "pdfium_wasm", "extractor_revision": receipt.Revision, "source_sha256": receipt.SHA256,
		"page_count": fmt.Sprint(len(receipt.Pages)), "extraction_state": "complete",
	}}
	var text strings.Builder
	partial := false
	for i, page := range receipt.Pages {
		if page.Page != i+1 || (page.State != "ready" && page.State != "empty" && page.State != "ocr_required" && page.State != "native_text_with_visuals") {
			return invalid()
		}
		if page.State == "empty" && strings.TrimSpace(page.Text) != "" {
			return invalid()
		}
		if (page.State == "ready" || page.State == "native_text_with_visuals") && strings.TrimSpace(page.Text) == "" {
			return invalid()
		}
		if int64(text.Len()+len(page.Text)+1) > p.maxExtractedSize {
			return nil, &ParseError{Code: "document_limit", Message: "Extracted PDF text exceeds the supported limit."}
		}
		if text.Len() > 0 {
			text.WriteByte('\n')
		}
		text.WriteString(page.Text)
		partial = partial || page.State == "ocr_required" || page.State == "native_text_with_visuals"
		if page.State == "native_text_with_visuals" {
			result.Warnings = append(result.Warnings, ExtractionWarning{Code: "document_visual_content_unread", SourceIndex: page.Page, Message: "Native text is available. Images and figures have not been interpreted."})
		}
		if page.State == "ocr_required" {
			result.Warnings = append(result.Warnings, ExtractionWarning{Code: "document_ocr_required", SourceIndex: page.Page, Message: "This page contains visual content that needs local OCR; native text alone may be incomplete."})
		}
		result.SourceUnits = append(result.SourceUnits, SourceUnit{Kind: "page", Index: page.Page, Text: page.Text, State: page.State, SourceSHA256: receipt.SHA256, ExtractorRevision: receipt.Revision})
	}
	result.Content = strings.TrimSpace(text.String())
	result.WordCount = countWords(result.Content)
	if partial {
		result.State = ExtractionPartial
		result.Metadata["extraction_state"] = "partial"
		return result, &ParseError{Code: "document_ocr_required", Message: "This PDF includes image or unqualified visual content that needs local OCR. Native page text is available, but the document is not fully extracted."}
	}
	if result.Content == "" {
		return nil, &ParseError{Code: "document_empty", Message: "This PDF has no readable text."}
	}
	return result, nil
}
