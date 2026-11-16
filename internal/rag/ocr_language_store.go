package rag

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"io"
	"os"
	"path/filepath"

	"github.com/takuphilchan/offgrid-llm/internal/documentocr"
)

const ocrLanguageDirectory = "document-languages"
const ocrLanguageFile = "document-languages/eng-" + ocrEnglishSHA + ".traineddata"
const MaxOCRLanguageBytes = 32 << 20

type OCRLanguageStatus struct {
	Language         string   `json:"language"`
	Name             string   `json:"name"`
	SHA256           string   `json:"sha256"`
	State            string   `json:"state"`
	Qualification    string   `json:"qualification"`
	InterfaceLocales []string `json:"interface_locales"`
	SourceURL        string   `json:"source_url"`
	Bytes            int      `json:"bytes"`
}

// Only reviewed data is accepted. Import cannot introduce executable adapters,
// choose output paths, download dependencies or change an existing index.
func ImportOCRLanguage(ctx context.Context, dataDir, language string, reader io.Reader) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	pack, supported := documentocr.LookupLanguage(language)
	if !supported {
		return &ParseError{Code: "document_ocr_language_unavailable", Message: "This OCR language is not yet supported."}
	}
	data, err := io.ReadAll(io.LimitReader(reader, MaxOCRLanguageBytes+1))
	if err != nil {
		return err
	}
	if len(data) > MaxOCRLanguageBytes {
		return &ParseError{Code: "document_limit", Message: "OCR language packs must be 32 MiB or smaller."}
	}
	if ocrDigest(data) != pack.SHA256 {
		return &ParseError{Code: "document_ocr_language_invalid", Message: "Choose the pinned OCR pack for the selected language. This file does not match its verified revision."}
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
	if err = root.Rename(tmp, ocrPackFile(pack)); err != nil {
		return ocrLanguageStorageError()
	}
	return nil
}

func ocrLanguageStorageError() error {
	return &ParseError{Code: "document_ocr_language_storage", Message: "The OCR language pack could not be accessed. Check workspace storage and try importing it again."}
}

func readOCRLanguage(dataDir string) ([]byte, error) {
	return readOCRLanguagePack(dataDir, "eng")
}

func ocrPackFile(pack documentocr.LanguagePack) string {
	return filepath.Join(ocrLanguageDirectory, pack.Language+"-"+pack.SHA256+".traineddata")
}

func readOCRLanguagePack(dataDir, language string) ([]byte, error) {
	pack, supported := documentocr.LookupLanguage(language)
	if !supported {
		return nil, &ParseError{Code: "document_ocr_language_unavailable", Message: "The selected OCR language has no supported pack. No substitute language was used."}
	}
	root, err := os.OpenRoot(dataDir)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, ocrLanguageStorageError()
	}
	defer root.Close()
	info, err := root.Lstat(ocrPackFile(pack))
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil || !info.Mode().IsRegular() || info.Size() > MaxOCRLanguageBytes {
		return nil, ocrLanguageStorageError()
	}
	file, err := root.Open(ocrPackFile(pack))
	if err != nil {
		return nil, ocrLanguageStorageError()
	}
	defer file.Close()
	data, err := io.ReadAll(io.LimitReader(file, MaxOCRLanguageBytes+1))
	if err != nil {
		return nil, ocrLanguageStorageError()
	}
	if len(data) > MaxOCRLanguageBytes || ocrDigest(data) != pack.SHA256 {
		return nil, &ParseError{Code: "document_ocr_language_invalid", Message: "The installed OCR language pack failed verification. Import the supported pack again."}
	}
	return data, nil
}

func OCRLanguages(dataDir string) []OCRLanguageStatus {
	result := []OCRLanguageStatus{}
	for _, pack := range documentocr.Languages().Packs {
		data, err := readOCRLanguagePack(dataDir, pack.Language)
		state := "missing"
		if err != nil {
			state = "invalid"
		} else if len(data) > 0 {
			state = "installed_verified"
		}
		result = append(result, OCRLanguageStatus{Language: pack.Language, Name: pack.Name, SHA256: pack.SHA256, State: state, Qualification: pack.Qualification, InterfaceLocales: pack.InterfaceLocales, SourceURL: pack.SourceURL, Bytes: pack.Bytes})
	}
	return result
}

// Each import takes its own immutable language snapshot. Replacing a pack does
// not mutate in-flight parsers or retroactively reindex saved documents.
func NewDocumentParserWithOCR(dataDir string) *DocumentParser {
	return NewDocumentParserForOCRLanguage(dataDir, "eng")
}

func NewDocumentParserForOCRLanguage(dataDir, language string) *DocumentParser {
	p := NewDocumentParser()
	if language == "" {
		language = "eng"
	}
	p.ocrLanguageID = language
	p.ocrLanguage, p.ocrSetupErr = readOCRSelection(dataDir, language)
	return p
}

func readOCRSelection(dataDir, selection string) ([]byte, error) {
	packs, ok := documentocr.SelectionPacks(selection)
	if !ok {
		return nil, &ParseError{Code: "document_ocr_language_unavailable", Message: "Choose up to three supported OCR languages. No substitute language was used."}
	}
	models := make([][]byte, 0, len(packs))
	for _, pack := range packs {
		data, err := readOCRLanguagePack(dataDir, pack.Language)
		if err != nil {
			return nil, err
		}
		// A missing member makes the entire selection unavailable. Never run a
		// convenient subset and persist it as the requested language selection.
		if len(data) == 0 {
			return nil, nil
		}
		models = append(models, data)
	}
	return documentocr.Models(selection, models)
}

func (e *Engine) DocumentParserForOCRLanguage(language string) *DocumentParser {
	return NewDocumentParserForOCRLanguage(e.dataDir, language)
}

func (e *Engine) DocumentParser() *DocumentParser   { return NewDocumentParserWithOCR(e.dataDir) }
func (e *Engine) OCRLanguages() []OCRLanguageStatus { return OCRLanguages(e.dataDir) }

// Service discovery reports every verified installed pack, not merely the
// legacy default. A parser still receives exactly the import's saved selection.
func (e *Engine) DocumentFormatReadiness() []FormatReadiness {
	formats := NewDocumentParser().FormatReadiness()
	languages := []string{}
	invalid := false
	for _, pack := range e.OCRLanguages() {
		if pack.State == "installed_verified" {
			languages = append(languages, pack.Language)
		}
		invalid = invalid || pack.State == "invalid"
	}
	for i := range formats {
		f := &formats[i]
		if f.OCRState == "" || f.OCRState == "unavailable" {
			continue
		}
		if len(languages) > 0 {
			f.OCRState = "installed_unchecked"
			f.OCRLanguages = append([]string{}, languages...)
			f.Limitations = []string{"Pinned local OCR packs are installed; select the document language explicitly. Recognition remains unqualified. Figures, tables and handwriting need separate review."}
		} else if invalid {
			f.OCRState = "language_invalid"
		}
	}
	return formats
}
func (e *Engine) ImportOCRLanguage(ctx context.Context, language string, reader io.Reader) error {
	return ImportOCRLanguage(ctx, e.dataDir, language, reader)
}
