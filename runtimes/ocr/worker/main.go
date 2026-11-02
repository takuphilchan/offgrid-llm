// offgrid-ocr-worker accepts one bounded image/language request, never paths,
// commands, URLs or credentials. It does not download language or runtime data.
package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"image/png"
	"io"
	"os"
	"time"
)

const languageSHA = "7d4322bd2a7749724879683fc3912cb542f19906c83bcc1a52132556427170b2"
const revision = "tesseract-5.5.4-acfa0590d55b0762-offgrid1"

type request struct {
	Protocol    int    `json:"protocol"`
	Language    string `json:"language"`
	LanguageSHA string `json:"language_sha256"`
	Model       []byte `json:"model"`
	PNG         []byte `json:"png"`
	PNG_SHA     string `json:"png_sha256"`
}

type receipt struct {
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

func digest(data []byte) string {
	hash := sha256.Sum256(data)
	return hex.EncodeToString(hash[:])
}

func process(ctx context.Context, input []byte) receipt {
	ctx, cancel := context.WithTimeout(ctx, 40*time.Second)
	defer cancel()
	out := receipt{Protocol: 1, Revision: revision}
	fail := func(code string) receipt {
		return receipt{Protocol: 1, Revision: revision, PNG_SHA: out.PNG_SHA, LanguageSHA: out.LanguageSHA, Error: code}
	}
	if ctx.Err() != nil {
		return fail("document_timeout")
	}
	if len(input) > 80<<20 {
		return fail("document_limit")
	}
	decoder := json.NewDecoder(bytes.NewReader(input))
	decoder.DisallowUnknownFields()
	var req request
	if decoder.Decode(&req) != nil {
		return fail("document_invalid")
	}
	var extra any
	if decoder.Decode(&extra) != io.EOF || req.Protocol != 1 {
		return fail("document_invalid")
	}
	out.PNG_SHA, out.LanguageSHA = req.PNG_SHA, req.LanguageSHA
	if req.Language != "eng" {
		return fail("document_ocr_language_unavailable")
	}
	if len(req.Model) == 0 {
		return fail("document_ocr_language_missing")
	}
	if len(req.Model) > 32<<20 || len(req.PNG) > 24<<20 {
		return fail("document_limit")
	}
	if req.LanguageSHA != languageSHA || digest(req.Model) != languageSHA {
		return fail("document_ocr_language_invalid")
	}
	if digest(req.PNG) != req.PNG_SHA {
		return fail("document_invalid")
	}
	config, err := png.DecodeConfig(bytes.NewReader(req.PNG))
	if err != nil {
		return fail("document_invalid")
	}
	if config.Width < 1 || config.Height < 1 || config.Width > 8192 || config.Height > 8192 || int64(config.Width)*int64(config.Height) > 16_000_000 {
		return fail("document_limit")
	}
	bitmap, err := png.Decode(bytes.NewReader(req.PNG))
	if err != nil {
		return fail("document_invalid")
	}
	engine, err := newEngine(ctx, req.Model)
	if err != nil {
		if ctx.Err() != nil {
			return fail("document_timeout")
		}
		return fail("document_ocr_unavailable")
	}
	defer engine.close()
	result, err := recognize(ctx, bitmap, engine.recognize)
	if ctx.Err() != nil {
		return fail("document_timeout")
	}
	if err != nil {
		return fail("document_ocr_failed")
	}
	out.Width, out.Height = config.Width, config.Height
	out.Rotation, out.Attempts, out.HOCR, out.State = result.rotation, result.attempts, result.hocr, result.state
	return out
}

func main() {
	if len(os.Args) == 2 && os.Args[1] == "--licenses" {
		_, _ = io.WriteString(os.Stdout, engineNotices)
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 40*time.Second)
	defer cancel()
	input, err := io.ReadAll(io.LimitReader(os.Stdin, (80<<20)+1))
	var result receipt
	if len(os.Args) != 1 || err != nil {
		result = receipt{Protocol: 1, Revision: revision, Error: "document_invalid"}
	} else {
		result = process(ctx, input)
	}
	if json.NewEncoder(os.Stdout).Encode(result) != nil {
		os.Exit(1)
	}
}
