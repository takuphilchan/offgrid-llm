package rag

import "os"

const builtinDocumentRevision = "offgrid-document-v2"
const xlsxDocumentRevision = "offgrid-xlsx-v3"

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
		{ID: "docx", Extensions: []string{".docx"}, SourceUnits: []string{"section"}, Limitations: []string{"Body text and tables only; rendered page numbers, headers, notes and images are not available."}},
		{ID: "xlsx", Extensions: []string{".xlsx"}, SourceUnits: []string{"sheet"}, Limitations: []string{"Stored cells with cell references and formula text only; cached formula results may be stale. Up to 100,000 populated cells per worksheet. Charts, images and macros are not interpreted."}},
		{ID: "pptx", Extensions: []string{".pptx"}, SourceUnits: []string{"slide"}, Limitations: []string{"Slide text only; notes, charts, images and embedded documents are not interpreted."}},
	}
	for i := range formats {
		formats[i].Runtime = "builtin"
		formats[i].RuntimeState = "available"
		formats[i].ExtractorRevision = builtinDocumentRevision
		if formats[i].ID == "xlsx" {
			formats[i].ExtractorRevision = xlsxDocumentRevision
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
		} else if len(p.ocrLanguage) > 0 && ocrDigest(p.ocrLanguage) == ocrEnglishSHA {
			pdf.OCRState = "installed_unchecked"
			pdf.OCRLanguages = []string{"eng"}
			pdf.Limitations = []string{"Local English OCR is installed but not release-qualified. Low-confidence or ambiguous mixed content remains partial. Installed runtime presence is not qualification."}
		}
	}
	formats = append(formats, pdf)
	for _, f := range []struct {
		id  string
		ext []string
	}{
		{"legacy_office", []string{".doc", ".xls", ".ppt"}},
		{"opendocument", []string{".odt", ".ods", ".odp"}},
		{"email", []string{".eml", ".msg", ".mbox"}},
		{"image_document", []string{".png", ".jpg", ".jpeg", ".tif", ".tiff"}},
	} {
		formats = append(formats, FormatReadiness{ID: f.id, Extensions: f.ext, Runtime: "none", RuntimeState: "unavailable", Qualification: "unverified", SourceUnits: []string{}, OCRLanguages: []string{}, EncryptedPolicy: "reject", Limitations: []string{"Requires a dedicated local document adapter. Chat image understanding is a separate model capability."}})
	}
	return formats
}
