//go:build windows && amd64

package computer

// UIA vtable slots follow Microsoft's UIAutomationClient.h. TextPattern reads
// and selects ranges; it is deliberately not treated as an insertion API.
import (
	"context"
	"strings"
	"unicode/utf16"
	"unicode/utf8"
	"unsafe"

	ole "github.com/go-ole/go-ole"
)

func uiaRangeText(r *ole.IUnknown) (string, error) {
	var value *int16
	// Request one more than the largest supported snapshot. Never ask a
	// provider to allocate unbounded document text with maxLength=-1.
	if err := comCall(r, 12, MaxDocumentSnapshotBytes+1, uintptr(unsafe.Pointer(&value))); err != nil {
		return "", err
	}
	if value == nil {
		return "", nil
	}
	defer ole.SysFreeString(value)
	size := ole.SysStringLen(value)
	if size > MaxDocumentSnapshotBytes {
		return "", ErrDocumentIncomplete
	}
	units := unsafe.Slice((*uint16)(unsafe.Pointer(value)), int(size))
	text, err := exactUTF16Text(units)
	if err != nil {
		return "", err
	}
	if len(text) > MaxDocumentSnapshotBytes || !utf8.ValidString(text) || strings.ContainsRune(text, 0) {
		return "", ErrDocumentIncomplete
	}
	return text, nil
}

func exactUTF16Text(units []uint16) (string, error) {
	for i := 0; i < len(units); i++ {
		u := units[i]
		if u == 0 {
			return "", ErrDocumentIncomplete
		}
		if u >= 0xd800 && u <= 0xdbff {
			if i+1 >= len(units) || units[i+1] < 0xdc00 || units[i+1] > 0xdfff {
				return "", ErrDocumentIncomplete
			}
			i++
		} else if u >= 0xdc00 && u <= 0xdfff {
			return "", ErrDocumentIncomplete
		}
	}
	text := string(utf16.Decode(units))
	if len(text) > MaxDocumentSnapshotBytes {
		return "", ErrDocumentIncomplete
	}
	return text, nil
}

func uiaCloneRange(r *ole.IUnknown) (*ole.IUnknown, error) {
	var copy *ole.IUnknown
	if err := comCall(r, 3, uintptr(unsafe.Pointer(&copy))); err != nil {
		return nil, err
	}
	if copy == nil {
		return nil, ErrDocumentUnavailable
	}
	return copy, nil
}

func uiaDocumentRange(editor *ole.IUnknown) (*ole.IUnknown, *ole.IUnknown, error) {
	p := pattern(editor, 10014) // UIA_TextPatternId
	if p == nil {
		return nil, nil, ErrDocumentUnavailable
	}
	var r *ole.IUnknown
	if err := comCall(p, 7, uintptr(unsafe.Pointer(&r))); err != nil || r == nil {
		p.Release()
		return nil, nil, ErrDocumentUnavailable
	}
	return p, r, nil
}

func uiaSelection(document, pattern *ole.IUnknown, text string) (*DocumentTextRange, error) {
	var ranges *ole.IUnknown
	if err := comCall(pattern, 5, uintptr(unsafe.Pointer(&ranges))); err != nil || ranges == nil {
		return nil, ErrDocumentUnavailable
	}
	defer ranges.Release()
	count, err := comInt(ranges, 3)
	if err != nil || count != 1 {
		return nil, ErrDocumentRange
	}
	var selected *ole.IUnknown
	if err := comCall(ranges, 4, 0, uintptr(unsafe.Pointer(&selected))); err != nil || selected == nil {
		return nil, ErrDocumentRange
	}
	defer selected.Release()
	offsets := [2]int{}
	for endpoint := 0; endpoint < 2; endpoint++ {
		prefix, err := uiaCloneRange(document)
		if err != nil {
			return nil, err
		}
		err = comCall(prefix, 15, 1, uintptr(unsafe.Pointer(selected)), uintptr(endpoint))
		value := ""
		if err == nil {
			value, err = uiaRangeText(prefix)
		}
		prefix.Release()
		if err != nil || !strings.HasPrefix(text, value) {
			return nil, ErrDocumentRange
		}
		offsets[endpoint] = utf8.RuneCountInString(value)
	}
	rangeValue := DocumentTextRange{Start: offsets[0], End: offsets[1]}
	if _, err := DocumentRangeToNative(text, rangeValue, TextUnitScalar); err != nil {
		return nil, err
	}
	value, err := uiaRangeText(selected)
	bytes, e := DocumentRangeToNative(text, rangeValue, TextUnitUTF8)
	if err != nil || e != nil || value != text[bytes.Start:bytes.End] {
		return nil, ErrDocumentRange
	}
	return &rangeValue, nil
}

// Providers choose their own TextUnit_Character semantics. Do not assume a
// character is a UTF-16 code unit, scalar or grapheme. Find the exact endpoint
// with bounded binary search and compare the entire prefix byte-for-byte.
func uiaPrefixRange(ctx context.Context, document *ole.IUnknown, text string, bytes int) (*ole.IUnknown, error) {
	if bytes < 0 || bytes > len(text) {
		return nil, ErrDocumentRange
	}
	lo, hi := 0, MaxDocumentSnapshotBytes+1
	for attempt := 0; attempt < 18 && lo <= hi; attempt++ {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		count := (lo + hi) / 2
		prefix, err := uiaCloneRange(document)
		if err != nil {
			return nil, err
		}
		// Collapse the end to the document start before moving it.
		err = comCall(prefix, 15, 1, uintptr(unsafe.Pointer(document)), 0)
		var moved int32
		if err == nil {
			err = comCall(prefix, 14, 1, 0, uintptr(count), uintptr(unsafe.Pointer(&moved)))
		}
		value := ""
		if err == nil {
			value, err = uiaRangeText(prefix)
		}
		if err != nil || !strings.HasPrefix(text, value) || moved < 0 || moved > int32(count) {
			prefix.Release()
			return nil, ErrDocumentRange
		}
		if len(value) == bytes {
			return prefix, nil
		}
		prefix.Release()
		if len(value) < bytes {
			if moved < int32(count) {
				return nil, ErrDocumentRange
			}
			lo = count + 1
		} else {
			hi = count - 1
		}
	}
	return nil, ErrDocumentRange
}

func uiaExactRange(ctx context.Context, document *ole.IUnknown, text string, selection DocumentTextRange) (*ole.IUnknown, error) {
	bytes, err := DocumentRangeToNative(text, selection, TextUnitUTF8)
	if err != nil {
		return nil, err
	}
	start, err := uiaPrefixRange(ctx, document, text, bytes.Start)
	if err != nil {
		return nil, err
	}
	defer start.Release()
	end, err := uiaPrefixRange(ctx, document, text, bytes.End)
	if err != nil {
		return nil, err
	}
	if err = comCall(end, 15, 0, uintptr(unsafe.Pointer(start)), 1); err != nil {
		end.Release()
		return nil, err
	}
	actual, err := uiaRangeText(end)
	if err != nil || actual != text[bytes.Start:bytes.End] {
		end.Release()
		return nil, ErrDocumentRange
	}
	return end, nil
}

func uiaComposition(editor *ole.IUnknown) string {
	p := pattern(editor, 10032) // UIA_TextEditPatternId
	if p == nil {
		return "unknown"
	}
	defer p.Release()
	var composition *ole.IUnknown
	if err := comCall(p, 9, uintptr(unsafe.Pointer(&composition))); err != nil {
		return "unknown"
	}
	if composition != nil {
		composition.Release()
		return "active"
	}
	return "stable"
}

func readUIADocument(editor *ole.IUnknown) (string, *DocumentTextRange, string, error) {
	p, r, err := uiaDocumentRange(editor)
	if err != nil {
		return "", nil, "unknown", err
	}
	defer p.Release()
	defer r.Release()
	text, err := uiaRangeText(r)
	if err != nil {
		return "", nil, "unknown", err
	}
	selection, err := uiaSelection(r, p, text)
	if err != nil {
		return text, nil, "unknown", err
	}
	composition := uiaComposition(editor)
	// Re-read content after collecting the selection; do not assemble a
	// snapshot from two different concurrent document states.
	// UIA ranges track edits. Reusing the old range can miss appended text;
	// acquire the document extent again from the provider for this check.
	p2, r2, err := uiaDocumentRange(editor)
	if err != nil {
		return "", nil, "unknown", err
	}
	defer p2.Release()
	defer r2.Release()
	current, err := uiaRangeText(r2)
	if err != nil || current != text {
		return "", nil, "unknown", ErrDocumentChanged
	}
	currentSelection, err := uiaSelection(r2, p2, current)
	if err != nil || currentSelection == nil || *currentSelection != *selection || uiaComposition(editor) != composition {
		return "", nil, "unknown", ErrDocumentChanged
	}
	return text, selection, composition, nil
}
