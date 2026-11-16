package rag

import (
	"encoding/hex"
	"fmt"
	"unicode/utf8"
)

// ExpectedEvidence identifies an exact labelled passage, including exceptions.
// Labels are supplied by the corpus reviewer, never inferred by the evaluator.
type ExpectedEvidence struct {
	DocumentID        string `json:"document_id"`
	SourceSHA256      string `json:"source_sha256"`
	ExtractorRevision string `json:"extractor_revision"`
	StartByte         int    `json:"start_byte"`
	EndByte           int    `json:"end_byte"`
	ContentHash       string `json:"content_hash"`
	Role              string `json:"role,omitempty"`
}

func validateExpectedEvidence(labels []ExpectedEvidence) error {
	if len(labels) > 100 {
		return fmt.Errorf("at most 100 evidence targets per question")
	}
	for _, x := range labels {
		if x.DocumentID == "" || len(x.SourceSHA256) != 64 || x.ExtractorRevision == "" || x.StartByte < 0 || x.EndByte <= x.StartByte || len(x.ContentHash) != 32 {
			return fmt.Errorf("evidence targets require exact document, source revision, byte range and content hash")
		}
		if _, err := hex.DecodeString(x.SourceSHA256); err != nil {
			return fmt.Errorf("invalid evidence source hash")
		}
		if _, err := hex.DecodeString(x.ContentHash); err != nil {
			return fmt.Errorf("invalid evidence content hash")
		}
	}
	return nil
}

// Each labelled span must be present in a returned excerpt from that revision.
// Retrieving a different paragraph of the right document is not a hit.
func scoreExpectedEvidence(labels []ExpectedEvidence, results []SearchResult) ([]int, int) {
	ranks := make([]int, len(labels))
	hits := 0
	for j, label := range labels {
		for i, result := range results {
			c := result.Chunk
			if result.DocumentID != label.DocumentID || c == nil || c.Source == nil || c.Source.SHA256 != label.SourceSHA256 || c.Source.Revision != label.ExtractorRevision || c.StartChar > label.StartByte || c.EndChar < label.EndByte {
				continue
			}
			start, end := label.StartByte-c.StartChar, label.EndByte-c.StartChar
			if start < 0 || end > len(c.Content) || !utf8.ValidString(c.Content[start:end]) || GenerateContentHash(c.Content[start:end]) != label.ContentHash {
				continue
			}
			ranks[j] = i + 1
			hits++
			break
		}
	}
	return ranks, hits
}
