//go:build windows && amd64

package computer

import (
	"strings"
	"testing"
	"unicode/utf16"
	"unsafe"
)

func TestDocumentUnicodePacketsPreserveExactTextAndBoundaries(t *testing.T) {
	if unsafe.Sizeof(documentKeyboardPacket{}) != 40 {
		t.Fatal("invalid Win32 INPUT layout")
	}
	for _, text := range []string{"", "OffGrid test — café 日本語 12345", strings.Repeat("x", 63) + "😀e\u0301\r\n👩‍👩‍👧‍👦", "مرحبا עברית\tनमस्ते", strings.Repeat("日", 1000)} {
		batches, err := documentUnicodeBatches(text)
		if err != nil {
			t.Fatal(err)
		}
		actual := ""
		for _, batch := range batches {
			if len(batch) > 2*maxDocumentBatchUTF16 {
				t.Fatal("unbounded input batch")
			}
			part, err := documentPacketText(batch)
			if err != nil {
				t.Fatal(err)
			}
			actual += part
			if _, err := DocumentRangeToNative(text, DocumentTextRange{Start: len([]rune(actual)), End: len([]rune(actual))}, TextUnitUTF16); err != nil {
				t.Fatal("split grapheme/CRLF", err)
			}
			if len(utf16.Encode([]rune(part))) != len(batch)/2 {
				t.Fatal("incorrect surrogate encoding")
			}
		}
		if actual != text {
			t.Fatal("Unicode changed")
		}
	}
}

func TestDocumentUnicodePacketsRejectUnsafeOrOversizedClustersBeforeInput(t *testing.T) {
	for _, text := range []string{"escape\x1b", "erase\b", "zero\x00", string([]byte{0xff}), "a" + strings.Repeat("\u0301", 100), strings.Repeat("x", MaxDocumentInputBytes+1)} {
		if _, err := documentUnicodeBatches(text); err == nil {
			t.Fatal("invalid input encoded")
		}
	}
	if _, err := documentPacketText([]documentKeyboardPacket{{Type: 1, VK: 0x41}, {Type: 1, VK: 0x41, Flags: 2}}); err == nil {
		t.Fatal("arbitrary virtual keys treated as text")
	}
}
