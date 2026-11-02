package rag

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"io"
	"os"
	"path/filepath"
)

const ocrLanguageDirectory = "document-languages"
const ocrLanguageFile = "document-languages/eng-" + ocrEnglishSHA + ".traineddata"
const MaxOCRLanguageBytes = 32 << 20

type OCRLanguageStatus struct {
	Language      string `json:"language"`
	Name          string `json:"name"`
	SHA256        string `json:"sha256"`
	State         string `json:"state"`
	Qualification string `json:"qualification"`
}

// Only reviewed data is accepted. Import cannot introduce executable adapters,
// choose output paths, download dependencies or change an existing index.
func ImportOCRLanguage(ctx context.Context, dataDir, language string, reader io.Reader) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if language != "eng" {
		return &ParseError{Code: "document_ocr_language_unavailable", Message: "This OCR language is not yet supported."}
	}
	data, err := io.ReadAll(io.LimitReader(reader, MaxOCRLanguageBytes+1))
	if err != nil {
		return err
	}
	if len(data) > MaxOCRLanguageBytes {
		return &ParseError{Code: "document_limit", Message: "OCR language packs must be 32 MiB or smaller."}
	}
	if ocrDigest(data) != ocrEnglishSHA {
		return &ParseError{Code: "document_ocr_language_invalid", Message: "Choose the supported English OCR language pack. This file does not match its verified revision."}
	}
	if err = ctx.Err(); err != nil {
		return err
	}
	root, err := os.OpenRoot(dataDir)
	if err != nil {
		return ocrLanguageStorageError()
	}
	defer root.Close()
	if err = root.MkdirAll(ocrLanguageDirectory, 0700); err != nil {
		return ocrLanguageStorageError()
	}
	info, err := root.Lstat(ocrLanguageDirectory)
	if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return ocrLanguageStorageError()
	}
	var id [16]byte
	if _, err = rand.Read(id[:]); err != nil {
		return err
	}
	tmp := filepath.Join(ocrLanguageDirectory, ".import-"+hex.EncodeToString(id[:]))
	file, err := root.OpenFile(tmp, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		return ocrLanguageStorageError()
	}
	defer root.Remove(tmp)
	_, err = file.Write(data)
	if err == nil {
		err = file.Sync()
	}
	closeErr := file.Close()
	if err != nil || closeErr != nil {
		return ocrLanguageStorageError()
	}
	if err = ctx.Err(); err != nil {
		return err
	}
	if err = root.Rename(tmp, ocrLanguageFile); err != nil {
		return ocrLanguageStorageError()
	}
	return nil
}

func ocrLanguageStorageError() error {
	return &ParseError{Code: "document_ocr_language_storage", Message: "The OCR language pack could not be accessed. Check workspace storage and try importing it again."}
}

func readOCRLanguage(dataDir string) ([]byte, error) {
	root, err := os.OpenRoot(dataDir)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, ocrLanguageStorageError()
	}
	defer root.Close()
	info, err := root.Lstat(ocrLanguageFile)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil || !info.Mode().IsRegular() || info.Size() > MaxOCRLanguageBytes {
		return nil, ocrLanguageStorageError()
	}
	file, err := root.Open(ocrLanguageFile)
	if err != nil {
		return nil, ocrLanguageStorageError()
	}
	defer file.Close()
	data, err := io.ReadAll(io.LimitReader(file, MaxOCRLanguageBytes+1))
	if err != nil {
		return nil, ocrLanguageStorageError()
	}
	if len(data) > MaxOCRLanguageBytes || ocrDigest(data) != ocrEnglishSHA {
		return nil, &ParseError{Code: "document_ocr_language_invalid", Message: "The installed OCR language pack failed verification. Import the supported pack again."}
	}
	return data, nil
}

func OCRLanguages(dataDir string) []OCRLanguageStatus {
	data, err := readOCRLanguage(dataDir)
	state := "missing"
	if err != nil {
		state = "invalid"
	} else if len(data) > 0 {
		state = "installed_verified"
	}
	return []OCRLanguageStatus{{Language: "eng", Name: "English", SHA256: ocrEnglishSHA, State: state, Qualification: "unverified"}}
}

// Each import takes its own immutable language snapshot. Replacing a pack does
// not mutate in-flight parsers or retroactively reindex saved documents.
func NewDocumentParserWithOCR(dataDir string) *DocumentParser {
	p := NewDocumentParser()
	p.ocrLanguage, p.ocrSetupErr = readOCRLanguage(dataDir)
	return p
}

func (e *Engine) DocumentParser() *DocumentParser   { return NewDocumentParserWithOCR(e.dataDir) }
func (e *Engine) OCRLanguages() []OCRLanguageStatus { return OCRLanguages(e.dataDir) }
func (e *Engine) ImportOCRLanguage(ctx context.Context, language string, reader io.Reader) error {
	return ImportOCRLanguage(ctx, e.dataDir, language, reader)
}
