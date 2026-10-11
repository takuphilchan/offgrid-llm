//go:build windows && amd64

package computer

import (
	"strings"
	"testing"
	"unicode/utf16"
)

func TestExactUTF16Readback(t *testing.T) {
	for _, text := range []string{"", "OffGrid test — café 日本語 12345", "😀 e\u0301 مرحبا\r\n", "\ufffd"} {
		got, err := exactUTF16Text(utf16.Encode([]rune(text)))
		if err != nil || got != text {
			t.Fatalf("exact Unicode read failed: %v", err)
		}
	}
	for _, invalid := range [][]uint16{{0}, {0xd800}, {0xdc00}, {0xd800, 'a'}, {0xd800, 0xd800}, utf16.Encode([]rune(strings.Repeat("日", MaxDocumentSnapshotBytes/3+1)))} {
		if _, err := exactUTF16Text(invalid); err == nil {
			t.Fatal("lossy/oversized read accepted")
		}
	}
}
