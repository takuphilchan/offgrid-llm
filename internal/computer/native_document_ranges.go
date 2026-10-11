package computer

import (
	"strings"
	"unicode/utf8"

	"github.com/clipperhouse/uax29/v2/graphemes"
)

type NativeTextUnit string

const (
	TextUnitScalar NativeTextUnit = "scalar"
	TextUnitUTF16  NativeTextUnit = "utf16"
	TextUnitUTF8   NativeTextUnit = "utf8"
)

// These conversions do not normalize, trim, or approximate provider offsets.
// A provider must prove that its text representation matches this exact snapshot
// (including CRLF), then verify the resolved native range text before input.
func documentBoundaries(text string, unit NativeTextUnit) (map[int]int, error) {
	if len(text) > MaxDocumentSnapshotBytes || !utf8.ValidString(text) || strings.ContainsRune(text, 0) {
		return nil, ErrDocumentRange
	}
	if unit != TextUnitScalar && unit != TextUnitUTF16 && unit != TextUnitUTF8 {
		return nil, ErrDocumentRange
	}
	boundaries := map[int]int{0: 0}
	scalars, native := 0, 0
	clusters := graphemes.FromString(text)
	for clusters.Next() {
		cluster := clusters.Value()
		for _, r := range cluster {
			scalars++
			switch unit {
			case TextUnitScalar:
				native++
			case TextUnitUTF16:
				native++
				if r > 0xffff {
					native++
				}
			case TextUnitUTF8:
				native += utf8.RuneLen(r)
			}
		}
		boundaries[scalars] = native
	}
	return boundaries, nil
}

func DocumentRangeToNative(text string, selection DocumentTextRange, unit NativeTextUnit) (DocumentTextRange, error) {
	if selection.Start < 0 || selection.End < selection.Start {
		return DocumentTextRange{}, ErrDocumentRange
	}
	boundaries, err := documentBoundaries(text, unit)
	if err != nil {
		return DocumentTextRange{}, err
	}
	start, ok := boundaries[selection.Start]
	end, okEnd := boundaries[selection.End]
	if !ok || !okEnd {
		return DocumentTextRange{}, ErrDocumentRange
	}
	return DocumentTextRange{Start: start, End: end}, nil
}

func DocumentRangeFromNative(text string, selection DocumentTextRange, unit NativeTextUnit) (DocumentTextRange, error) {
	if selection.Start < 0 || selection.End < selection.Start {
		return DocumentTextRange{}, ErrDocumentRange
	}
	boundaries, err := documentBoundaries(text, unit)
	if err != nil {
		return DocumentTextRange{}, err
	}
	start, end := -1, -1
	for scalar, native := range boundaries {
		if native == selection.Start {
			start = scalar
		}
		if native == selection.End {
			end = scalar
		}
	}
	if start < 0 || end < 0 {
		return DocumentTextRange{}, ErrDocumentRange
	}
	return DocumentTextRange{Start: start, End: end}, nil
}
