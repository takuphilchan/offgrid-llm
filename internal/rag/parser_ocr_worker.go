package rag

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/takuphilchan/offgrid-llm/internal/documentocr"
)

const ocrWorkerRevision = "tesseract-5.5.4-acfa0590d55b0762-offgrid1"
const ocrEnglishSHA = "7d4322bd2a7749724879683fc3912cb542f19906c83bcc1a52132556427170b2"

type ocrReceipt struct {
	Protocol    int    `json:"protocol"`
	Revision    string `json:"revision"`
	PNG_SHA     string `json:"png_sha256"`
	LanguageSHA string `json:"language_sha256"`
	Width       int    `json:"width,omitempty"`
	Height      int    `json:"height,omitempty"`
	Rotation    int    `json:"rotation"`
	Attempts    int    `json:"attempts,omitempty"`
	HOCR        string `json:"hocr,omitempty"`
	State       string `json:"state,omitempty"`
	Error       string `json:"error,omitempty"`
}

type ocrPageResult struct {
	Words              []documentocr.Word
	Text, State        string
	Rotation, Attempts int
}

func ocrDigest(data []byte) string { sum := sha256.Sum256(data); return hex.EncodeToString(sum[:]) }

// This is an explicit configuration operation, never a document-directed model
// load. Other languages need their own reviewed pack and qualification evidence.
func (p *DocumentParser) UseOCRLanguage(language string, data []byte) error {
	if language != "eng" || len(data) > 32<<20 || ocrDigest(data) != ocrEnglishSHA {
		return &ParseError{Code: "document_ocr_language_invalid", Message: "The OCR language pack is not a supported, verified revision."}
	}
	p.ocrLanguage = bytes.Clone(data)
	return nil
}

func (p *DocumentParser) recognizePDFRaster(ctx context.Context, raster *pdfPageRaster) (*ocrPageResult, error) {
	if raster == nil || raster.Error != "" {
		return nil, &ParseError{Code: "document_invalid", Message: "A verified PDF page is required before OCR."}
	}
	if p.ocrWorker == "" {
		return nil, &ParseError{Code: "document_ocr_unavailable", Message: "The packaged local OCR reader is unavailable."}
	}
	if len(p.ocrLanguage) == 0 {
		return nil, &ParseError{Code: "document_ocr_language_missing", Message: "Install a supported OCR language pack to read scanned pages."}
	}
	if ocrDigest(p.ocrLanguage) != ocrEnglishSHA {
		return nil, &ParseError{Code: "document_ocr_language_invalid", Message: "The installed OCR language pack did not pass verification."}
	}
	ctx, cancel := context.WithTimeout(ctx, 45*time.Second)
	defer cancel()
	// Share document admission; raster rendering has already released its slot.
	select {
	case pdfWorkerAdmission <- struct{}{}:
		defer func() { <-pdfWorkerAdmission }()
	case <-ctx.Done():
		return nil, ctx.Err()
	}
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}
	input, err := json.Marshal(struct {
		Protocol    int    `json:"protocol"`
		Language    string `json:"language"`
		LanguageSHA string `json:"language_sha256"`
		Model       []byte `json:"model"`
		PNG         []byte `json:"png"`
		PNG_SHA     string `json:"png_sha256"`
	}{1, "eng", ocrEnglishSHA, p.ocrLanguage, raster.PNG, ocrDigest(raster.PNG)})
	if err != nil {
		return nil, err
	}
	cmd := exec.CommandContext(ctx, p.ocrWorker)
	configureParserProcess(cmd)
	cmd.Stdin = bytes.NewReader(input)
	cmd.Env = []string{"GOMAXPROCS=1", "GOMEMLIMIT=256MiB"}
	for _, key := range []string{"SystemRoot", "WINDIR", "TEMP", "TMP"} {
		if value := os.Getenv(key); value != "" {
			cmd.Env = append(cmd.Env, key+"="+value)
		}
	}
	output, err := runParserCommand(ctx, cmd, 3<<20)
	if err != nil {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		return nil, &ParseError{Code: "document_ocr_failed", Message: "The local OCR worker could not finish. No OCR text was accepted."}
	}
	return decodeOCRReceipt([]byte(output), raster)
}

func decodeOCRReceipt(raw []byte, raster *pdfPageRaster) (*ocrPageResult, error) {
	invalid := func() (*ocrPageResult, error) {
		return nil, &ParseError{Code: "document_parser_protocol", Message: "The local OCR reader returned invalid source evidence. No OCR text was accepted."}
	}
	if len(raw) > 3<<20 || raster == nil {
		return invalid()
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	var r ocrReceipt
	if decoder.Decode(&r) != nil {
		return invalid()
	}
	var extra any
	if decoder.Decode(&extra) != io.EOF {
		return invalid()
	}
	if r.Protocol != 1 || r.Revision != ocrWorkerRevision || r.PNG_SHA != ocrDigest(raster.PNG) || r.LanguageSHA != ocrEnglishSHA {
		return invalid()
	}
	if r.Error != "" {
		if r.HOCR != "" || r.State != "" || r.Width != 0 || r.Height != 0 || r.Rotation != 0 || r.Attempts != 0 {
			return invalid()
		}
		messages := map[string]string{
			"document_invalid": "The scanned page could not be read.", "document_limit": "The scanned page exceeds local OCR limits.",
			"document_timeout": "Reading the scanned page timed out. Try a smaller document.", "document_ocr_language_missing": "Install an OCR language pack to read scanned pages.",
			"document_ocr_language_unavailable": "This OCR language is not supported by the installed reader.", "document_ocr_language_invalid": "The OCR language pack did not pass verification.",
			"document_ocr_unavailable": "The local OCR reader could not start.", "document_ocr_failed": "The local OCR reader could not finish this page.",
		}
		message, ok := messages[r.Error]
		if !ok {
			return invalid()
		}
		return nil, &ParseError{Code: r.Error, Message: message}
	}
	if r.Width != raster.Width || r.Height != raster.Height || r.Attempts < 1 || r.Attempts > 4 || r.State != "complete" && r.State != "partial" {
		return invalid()
	}
	w, h := r.Width, r.Height
	if r.Rotation == 90 || r.Rotation == 270 {
		w, h = h, w
	}
	if r.Rotation != 0 && r.Rotation != 90 && r.Rotation != 180 && r.Rotation != 270 {
		return invalid()
	}
	words, err := documentocr.ParseHOCR(r.HOCR, w, h)
	if err != nil {
		return invalid()
	}
	_, reliable := documentocr.Quality(words)
	if r.State == "complete" && !reliable {
		return invalid()
	}
	var text strings.Builder
	line := 0
	for i := range words {
		box, err := documentocr.OriginalBounds(words[i].Bounds, r.Width, r.Height, r.Rotation)
		if err != nil {
			return invalid()
		}
		words[i].Bounds = box
		if text.Len() > 0 {
			if line != words[i].Line {
				text.WriteByte('\n')
			} else {
				text.WriteByte(' ')
			}
		}
		text.WriteString(words[i].Text)
		line = words[i].Line
	}
	return &ocrPageResult{Words: words, Text: text.String(), State: r.State, Rotation: r.Rotation, Attempts: r.Attempts}, nil
}
