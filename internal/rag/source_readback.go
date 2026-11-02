package rag

import (
	"fmt"
	"unicode/utf8"
)

// ReadSourcePassage resolves a saved citation against the current retained
// extraction. A stale citation must not silently open different document text.
// Callers must authorize document access before invoking this method.
func (d *Document) ReadSourcePassage(start, end int, digest, revision, contentHash string) (string, error) {
	if !d.SourceRetained || start < 0 || end <= start || end > len(d.RawContent) || end-start > 800000 {
		return "", fmt.Errorf("source passage unavailable")
	}
	if err := validateSourceRanges(d.RawContent, d.SourceRanges); err != nil {
		return "", err
	}
	for _, unit := range d.SourceRanges {
		if start < unit.Start || end > unit.End || digest != unit.SHA256 || revision != unit.Revision {
			continue
		}
		text := d.RawContent[start:end]
		if utf8.ValidString(text) && GenerateContentHash(text) == contentHash {
			return text, nil
		}
	}
	return "", fmt.Errorf("source passage no longer matches its saved citation")
}
