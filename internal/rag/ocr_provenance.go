package rag

import (
	"github.com/takuphilchan/offgrid-llm/internal/document"
	"github.com/takuphilchan/offgrid-llm/internal/documentocr"
)

func (p *DocumentParser) ocrRevision(legacy string) string {
	pack := p.ocrPack()
	if pack.Language == "eng" {
		return legacy
	}
	return "offgrid-ocr-language-v1-" + ocrDigest([]byte(legacy+"\x00"+pack.Language+"\x00"+pack.SHA256))
}

func (p *DocumentParser) bindOCRProvenance(result *ParseResult) {
	pack := p.ocrPack()
	result.Metadata["ocr_language"] = pack.Language
	result.Metadata["ocr_revision"] = ocrWorkerRevision
	result.Metadata["ocr_language_sha256"] = pack.SHA256
}

func ocrRuntimeIdentities(result *ParseResult) []document.RuntimeIdentity {
	language, digest := result.Metadata["ocr_language"], result.Metadata["ocr_language_sha256"]
	// Existing fixture/projection callers and retained legacy records are English.
	if language == "" && digest == "" {
		language, digest = "eng", ocrEnglishSHA
	}
	if selection, ok := documentocr.LookupSelection(language); ok && selection.SHA256 == digest {
		packs, _ := documentocr.SelectionPacks(language)
		identities := make([]document.RuntimeIdentity, 0, len(packs))
		for _, pack := range packs {
			identities = append(identities, document.RuntimeIdentity{Name: "tesseract-" + pack.Language, Revision: ocrWorkerRevision, SHA256: pack.SHA256})
		}
		return identities
	}
	return []document.RuntimeIdentity{{Name: "tesseract-" + language, Revision: ocrWorkerRevision, SHA256: digest}}
}
