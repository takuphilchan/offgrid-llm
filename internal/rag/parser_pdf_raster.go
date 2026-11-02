package rag

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"image/png"
	"io"
	"os/exec"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"
)

const pdfRasterRevision = pdfWorkerRevision + "-raster2"

type pdfNativeRegion struct {
	Text   string `json:"text"`
	Bounds [4]int `json:"bounds"`
}

type pdfPageRaster struct {
	Protocol int               `json:"protocol"`
	Revision string            `json:"revision"`
	SHA256   string            `json:"sha256"`
	Page     int               `json:"page"`
	Width    int               `json:"width"`
	Height   int               `json:"height"`
	PNG      []byte            `json:"png,omitempty"`
	Native   []pdfNativeRegion `json:"native,omitempty"`
	Error    string            `json:"error,omitempty"`
}

// Internal OCR input, not an arbitrary screenshot or host-path interface. The
// selected page, source digest and dimensions are independently checked before
// another local worker may consume the pixels.
func (p *DocumentParser) rasterPDFPage(ctx context.Context, source []byte, page int) (*pdfPageRaster, error) {
	if len(source) > 50<<20 || page < 1 || page > 1000 {
		return nil, &ParseError{Code: "document_limit", Message: "PDF page exceeds the local rendering limits."}
	}
	if p.pdfWorker == "" {
		return nil, &ParseError{Code: "document_parser_unavailable", Message: "The packaged PDF reader is unavailable."}
	}
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	select {
	case pdfWorkerAdmission <- struct{}{}:
		defer func() { <-pdfWorkerAdmission }()
	case <-ctx.Done():
		return nil, ctx.Err()
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	cmd := exec.CommandContext(ctx, p.pdfWorker, "--raster-page", strconv.Itoa(page))
	output, err := p.runPDFWorkerOutput(ctx, cmd, source, 34<<20)
	if err != nil {
		return nil, err
	}
	return decodePDFRaster([]byte(output), source, page)
}

func decodePDFRaster(output, source []byte, page int) (*pdfPageRaster, error) {
	invalid := func() (*pdfPageRaster, error) {
		return nil, &ParseError{Code: "document_parser_protocol", Message: "The PDF reader returned invalid page pixels. No OCR input was accepted."}
	}
	if len(output) > 34<<20 {
		return invalid()
	}
	decoder := json.NewDecoder(bytes.NewReader(output))
	decoder.DisallowUnknownFields()
	var r pdfPageRaster
	if decoder.Decode(&r) != nil {
		return invalid()
	}
	var trailing any
	if decoder.Decode(&trailing) != io.EOF {
		return invalid()
	}
	digest := sha256.Sum256(source)
	if r.Protocol != 1 || r.Revision != pdfRasterRevision || r.Page != page || page < 1 || page > 1000 || r.SHA256 != hex.EncodeToString(digest[:]) {
		return invalid()
	}
	if r.Error != "" {
		if len(r.PNG) != 0 || r.Width != 0 || r.Height != 0 || len(r.Native) != 0 {
			return invalid()
		}
		messages := map[string]string{
			"document_invalid":            "The PDF page could not be read.",
			"document_encrypted":          "Export an unencrypted PDF before importing.",
			"document_empty":              "The PDF contains no readable pages.",
			"document_limit":              "The PDF page exceeds local rendering limits.",
			"document_timeout":            "PDF page rendering timed out.",
			"document_parser_unavailable": "The local PDF reader could not start.",
		}
		message, ok := messages[r.Error]
		if !ok {
			return invalid()
		}
		return nil, &ParseError{Code: r.Error, Message: message}
	}
	if r.Width < 1 || r.Height < 1 || r.Width > 8192 || r.Height > 8192 || int64(r.Width)*int64(r.Height) > 16_000_000 || len(r.PNG) == 0 || len(r.PNG) > 24<<20 {
		return invalid()
	}
	if len(r.Native) > 20000 {
		return invalid()
	}
	nativeBytes := 0
	for _, region := range r.Native {
		nativeBytes += len(region.Text)
		b := region.Bounds
		if !utf8.ValidString(region.Text) || strings.TrimSpace(region.Text) == "" || nativeBytes > 80000 || b[0] < 0 || b[1] < 0 || b[2] > r.Width || b[3] > r.Height || b[0] >= b[2] || b[1] >= b[3] {
			return invalid()
		}
	}
	config, err := png.DecodeConfig(bytes.NewReader(r.PNG))
	if err != nil || config.Width != r.Width || config.Height != r.Height {
		return invalid()
	}
	// Validate all chunks/CRCs, not just a plausible image header. Allocation is
	// bounded above before decoding; the OCR worker receives no external references.
	if _, err = png.Decode(bytes.NewReader(r.PNG)); err != nil {
		return invalid()
	}
	return &r, nil
}
