package rag

import (
	"github.com/takuphilchan/offgrid-llm/internal/documentocr"
	"os"
	"strings"
)

const builtinDocumentRevision = "offgrid-document-v2"
const xlsxDocumentRevision = "offgrid-xlsx-structure-v4"
const docxDocumentRevision = "offgrid-docx-structure-v7"
const pptxDocumentRevision = "offgrid-pptx-structure-v3"

// FormatReadiness describes extraction, independently of embedding/index setup.
// Installed is deliberately not equivalent to tested or qualified.
type FormatReadiness struct {
	ID                string   `json:"id"`
	Extensions        []string `json:"extensions"`
	Runtime           string   `json:"runtime"`
	RuntimeState      string   `json:"runtime_state"`
	ExtractorRevision string   `json:"extractor_revision,omitempty"`
	Qualification     string   `json:"qualification"`
	SourceUnits       []string `json:"source_units"`
	MaxFileBytes      int64    `json:"max_file_bytes"`
	MaxTextBytes      int64    `json:"max_text_bytes"`
	OCRAvailable      bool     `json:"ocr_available"`
	OCRLanguages      []string `json:"ocr_languages"`
	OCRState          string   `json:"ocr_state,omitempty"`
	EncryptedPolicy   string   `json:"encrypted_policy"`
	Limitations       []string `json:"limitations"`
}

// FormatReadiness never executes an extractor or scans PATH. Runtime presence
// says nothing about a particular document, page completeness or retrieval quality.
func (p *DocumentParser) FormatReadiness() []FormatReadiness {
	formats := []FormatReadiness{
		{ID: "text", Extensions: []string{".txt", ".md", ".markdown", ".go", ".py", ".js", ".ts", ".java", ".c", ".cpp", ".rs", ".rb", ".php"}, SourceUnits: []string{"section"}, Limitations: []string{"UTF-8 text only; code is never executed."}},
		{ID: "structured_text", Extensions: []string{".json", ".csv", ".xml", ".html", ".htm", ".rtf"}, SourceUnits: []string{"section"}, Limitations: []string{"Extracted text only; layout, scripts, external resources and embedded objects are not rendered."}},
		{ID: "docx", Extensions: []string{".docx"}, SourceUnits: []string{"section"}, Limitations: []string{"Current body text, tables, referenced notes, headers and footers. Page placement and fields are not recalculated. Images and embedded objects are not interpreted. Tracked changes are disclosed."}},
		{ID: "xlsx", Extensions: []string{".xlsx"}, SourceUnits: []string{"sheet"}, Limitations: []string{"Native typed cells, raw numbers, stored styles, supported numeric/date displays and explicit table headers. Unsupported display formats retain raw values and format codes with review warnings. Hidden cells and merged ranges are marked. Formula caches may be stale or missing; formulas and external links are never evaluated. Up to 100,000 populated cells per worksheet. Charts, images, conditional formatting and macros are not interpreted."}},
		{ID: "pptx", Extensions: []string{".pptx"}, SourceUnits: []string{"slide"}, Limitations: []string{"Native slide text, speaker notes and table structure in XML order. Master/layout text, animations and visual reading order are not resolved. Images, charts and embedded objects are identified but not interpreted. Table styling is not proof of semantic headers."}},
	}
	for i := range formats {
		formats[i].Runtime = "builtin"
		formats[i].RuntimeState = "available"
		formats[i].ExtractorRevision = builtinDocumentRevision
		if formats[i].ID == "docx" {
			formats[i].ExtractorRevision = docxDocumentRevision
		}
		if formats[i].ID == "xlsx" {
			formats[i].ExtractorRevision = xlsxDocumentRevision
		}
		if formats[i].ID == "pptx" {
			formats[i].ExtractorRevision = pptxDocumentRevision
		}
		formats[i].Qualification = "unverified"
		formats[i].MaxFileBytes = p.maxFileSize
		formats[i].MaxTextBytes = p.maxExtractedSize
		formats[i].OCRLanguages = []string{}
		formats[i].EncryptedPolicy = "reject"
	}
	pdf := FormatReadiness{
		ID: "pdf", Extensions: []string{".pdf"}, Runtime: "pdfium_wasm", RuntimeState: "unavailable",
		ExtractorRevision: pdfWorkerRevision, Qualification: "unverified", SourceUnits: []string{"page"},
		MaxFileBytes: min(p.maxFileSize, 50<<20), MaxTextBytes: p.maxExtractedSize,
		OCRLanguages: []string{}, EncryptedPolicy: "reject",
		Limitations: []string{"Requires the packaged PDF worker. Scans and mixed visual content need OCR, which is unavailable. Installed runtime presence is not qualification."},
	}
	if info, err := os.Lstat(p.pdfWorker); err == nil && info.Mode().IsRegular() {
		pdf.RuntimeState = "installed_unchecked"
	}
	pdf.OCRState = "unavailable"
	if info, err := os.Lstat(p.ocrWorker); err == nil && info.Mode().IsRegular() {
		pdf.OCRState = "language_missing"
		if p.ocrSetupErr != nil {
			pdf.OCRState = "language_invalid"
		} else if documentocr.ValidateModels(p.ocrPack().Language, p.ocrLanguage) {
			pdf.OCRState = "installed_unchecked"
			pdf.OCRLanguages = strings.Split(p.ocrPack().Language, "+")
			pdf.Limitations = []string{"Local OCR is installed but not release-qualified. Low-confidence or ambiguous mixed content remains partial. Installed runtime presence is not qualification."}
		}
	}
	formats = append(formats, pdf)
	image := FormatReadiness{ID: "image_document", Extensions: []string{".png", ".jpg", ".jpeg"}, Runtime: "tesseract_wasm", RuntimeState: "unavailable",
		ExtractorRevision: imageDocumentRevision, Qualification: "unverified", SourceUnits: []string{"page"}, MaxFileBytes: min(p.maxFileSize, imageDocumentMaxBytes), MaxTextBytes: p.maxExtractedSize,
		OCRState: pdf.OCRState, OCRLanguages: append([]string{}, pdf.OCRLanguages...), EncryptedPolicy: "reject",
		Limitations: []string{"Single still PNG/JPEG, at most 8192 pixels per side and 16 million pixels. Requires the packaged local OCR worker and verified English pack. Printed-text transcription only; figures, tables, handwriting and signatures are not interpreted. EXIF orientation is preserved through original-pixel source coordinates. Runtime presence is not language or format qualification."}}
	if pdf.OCRState != "unavailable" {
		image.RuntimeState = "installed_unchecked"
	}
	formats = append(formats, image)
	tiff := image
	tiff.ID = "tiff"
	tiff.Extensions = []string{".tif", ".tiff"}
	tiff.ExtractorRevision = tiffDocumentRevision
	tiff.Limitations = []string{"Classic multipage TIFF: up to 100 pages, 8192 pixels per side, 16 million pixels per page and 200 million pixels total. BigTIFF, subimages, planar samples and JPEG compression require conversion. Page orientation is retained. Requires the packaged OCR worker and verified English pack. Printed text only; deskew, visual interpretation and format/language qualification remain unavailable."}
	formats = append(formats, tiff)
	for _, f := range []struct {
		id  string
		ext []string
	}{
		{"legacy_office", []string{".doc", ".xls", ".ppt"}},
		{"opendocument", []string{".odt", ".ods", ".odp"}},
		{"email", []string{".eml", ".msg", ".mbox"}},
	} {
		formats = append(formats, FormatReadiness{ID: f.id, Extensions: f.ext, Runtime: "none", RuntimeState: "unavailable", Qualification: "unverified", SourceUnits: []string{}, OCRLanguages: []string{}, EncryptedPolicy: "reject", Limitations: []string{"Requires a dedicated local document adapter. Chat image understanding is a separate model capability."}})
	}
	return formats
}
