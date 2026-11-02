package rag

import (
	"context"
	"errors"
	"io"
	"os/exec"
	"time"
)

// ParsePDF uses the packaged worker only. PATH tools and regex extraction cannot
// establish complete text or trustworthy source units and are never fallbacks.
func (p *DocumentParser) ParsePDF(content []byte) (*ParseResult, error) {
	return p.parsePDF(context.Background(), content)
}

func (p *DocumentParser) parsePDF(ctx context.Context, content []byte) (*ParseResult, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if len(content) < 5 || string(content[:5]) != "%PDF-" {
		return nil, &ParseError{Code: "document_invalid", Message: "This file does not have a valid PDF header."}
	}
	if p.pdfWorker == "" {
		return nil, &ParseError{Code: "document_parser_unavailable", Message: "The packaged local PDF reader is unavailable. Install an OffGrid build containing the document runtime, or export this document as text. No extracted text was accepted."}
	}
	result, err := p.extractPDFWorker(ctx, content)
	var problem *ParseError
	if result != nil && errors.As(err, &problem) && problem.Code == "document_ocr_required" && p.ocrWorker != "" {
		if p.ocrSetupErr != nil {
			return result, p.ocrSetupErr
		}
		if len(p.ocrLanguage) == 0 {
			return result, &ParseError{Code: "document_ocr_language_missing", Message: "Import a supported OCR language pack to read the scanned pages. Your original document is retained."}
		}
		return p.extractScannedPDFPages(ctx, content, result)
	}
	return result, err
}

// Both process lifetime and pipe draining are bounded. No stderr or temporary
// filesystem paths are returned to document clients.
func runParserCommand(ctx context.Context, cmd *exec.Cmd, limit int64) (string, error) {
	cmd.WaitDelay = time.Second
	output := &parserOutput{limit: limit}
	cmd.Stdout = output
	cmd.Stderr = io.Discard
	err := cmd.Run()
	if output.exceeded {
		return "", &ParseError{Code: "document_limit", Message: "Extracted PDF text exceeds the supported limit."}
	}
	if ctx.Err() != nil {
		return "", ctx.Err()
	}
	if err != nil {
		return "", err
	}

	return output.String(), nil
}
