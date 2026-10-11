//go:build windows && amd64

package computer

import (
	"context"
	"runtime"
	"strings"
	"time"
	"unicode"
	"unicode/utf16"
	"unicode/utf8"
	"unsafe"

	"github.com/clipperhouse/uax29/v2/graphemes"
	ole "github.com/go-ole/go-ole"
	"golang.org/x/sys/windows"
)

// INPUT is 40 bytes on Windows x64. Never emit arbitrary virtual-key macros:
// text packets use only VK_PACKET; deletion is one explicit selected-range edit.
type documentKeyboardPacket struct {
	Type, Padding uint32
	VK, Scan      uint16
	Flags, Time   uint32
	Padding2      uint32
	ExtraInfo     uintptr
	UnionPadding  [8]byte
}

const maxDocumentBatchUTF16 = 64

func documentUnicodeBatches(text string) ([][]documentKeyboardPacket, error) {
	if ValidateDocumentInput(text) != nil {
		return nil, ErrInvalidControl
	}
	for _, r := range text {
		// Do not turn text containing escape/backspace into keyboard controls.
		if unicode.IsControl(r) && r != '\r' && r != '\n' && r != '\t' {
			return nil, ErrInvalidControl
		}
	}
	var result [][]documentKeyboardPacket
	batch := []documentKeyboardPacket{}
	clusters := graphemes.FromString(text)
	for clusters.Next() {
		units := utf16.Encode([]rune(clusters.Value()))
		if len(units) > maxDocumentBatchUTF16 {
			return nil, ErrDocumentRange
		}
		if len(batch)/2+len(units) > maxDocumentBatchUTF16 {
			result = append(result, batch)
			batch = nil
		}
		for _, u := range units {
			batch = append(batch, documentKeyboardPacket{Type: 1, Scan: u, Flags: 4}, documentKeyboardPacket{Type: 1, Scan: u, Flags: 6})
		}
	}
	if len(batch) > 0 {
		result = append(result, batch)
	}
	return result, nil
}

func documentPacketText(packets []documentKeyboardPacket) (string, error) {
	units := make([]uint16, 0, len(packets)/2)
	if len(packets)%2 != 0 {
		return "", ErrInvalidControl
	}
	for i := 0; i < len(packets); i += 2 {
		down, up := packets[i], packets[i+1]
		if down.Type != 1 || up.Type != 1 || down.VK != 0 || up.VK != 0 || down.Flags != 4 || up.Flags != 6 || down.Scan != up.Scan {
			return "", ErrInvalidControl
		}
		units = append(units, down.Scan)
	}
	return exactUTF16Text(units)
}

func documentKeysReleased() bool {
	getKey := user32.NewProc("GetAsyncKeyState")
	// Refuse interference; never synthesize releases for a key held by a user.
	for _, key := range []uintptr{0x10, 0x11, 0x12, 0x5b, 0x5c, 0x01, 0x02} {
		state, _, _ := getKey.Call(key)
		if state&0x8000 != 0 {
			return false
		}
	}
	return true
}

func sendDocumentPackets(packets []documentKeyboardPacket) error {
	if len(packets) == 0 || len(packets) > 2*maxDocumentBatchUTF16 || !documentKeysReleased() {
		return ErrControlScope
	}
	n, _, _ := sendInput.Call(uintptr(len(packets)), uintptr(unsafe.Pointer(&packets[0])), unsafe.Sizeof(packets[0]))
	runtime.KeepAlive(packets)
	if n != uintptr(len(packets)) {
		return ErrUncertain
	}
	return nil
}

// The identityCheck callback must re-read the provider's document/container
// relationship as well as validating the OS target. It is deliberately required:
// a reused editor pointer cannot establish document continuity on its own.
// This helper is private and only called after the host journal's dispatch claim.
func applyUIADocumentText(ctx context.Context, editor *ole.IUnknown, plan PreparedDocumentText, identityCheck func() error) error {
	if editor == nil || plan.ID() == "" || identityCheck == nil {
		return ErrInvalidControl
	}
	op, before := plan.Operation(), plan.Preimage()
	if op.Validate() != nil || before.Selection == nil {
		return ErrInvalidControl
	}
	guard := func() error {
		if err := ctx.Err(); err != nil {
			return err
		}
		if err := identityCheck(); err != nil {
			return err
		}
		focused, err := comInt(editor, 26)
		if err != nil || focused == 0 || !documentKeysReleased() {
			return ErrControlScope
		}
		return nil
	}
	check := func(text string, selection DocumentTextRange) error {
		if err := guard(); err != nil {
			return err
		}
		current, selected, composition, err := readUIADocument(editor)
		if err != nil {
			return err
		}
		if composition != "stable" {
			return ErrDocumentComposition
		}
		if current != text || selected == nil || *selected != selection {
			return ErrDocumentChanged
		}
		return nil
	}
	if err := check(before.Text, *before.Selection); err != nil {
		return err
	}
	if op.Kind == "select_text" {
		p, r, err := uiaDocumentRange(editor)
		if err != nil {
			return err
		}
		defer p.Release()
		defer r.Release()
		selected, err := uiaExactRange(ctx, r, before.Text, *op.Range)
		if err != nil {
			return err
		}
		defer selected.Release()
		if err = check(before.Text, *before.Selection); err != nil {
			return err
		}
		// Selection is itself a recorded action, not permission to type later.
		return comCall(selected, 16)
	}
	if op.Kind == "replace_value" {
		p := pattern(editor, 10002)
		if p == nil {
			return ErrDocumentUnavailable
		}
		defer p.Release()
		readOnly, err := comInt(p, 5)
		if err != nil || readOnly != 0 {
			return ErrDocumentUnavailable
		}
		value, err := windows.UTF16PtrFromString(*op.Text)
		if err != nil {
			return ErrInvalidControl
		}
		if err = check(before.Text, *before.Selection); err != nil {
			return err
		}
		err = comCall(p, 3, uintptr(unsafe.Pointer(value)))
		runtime.KeepAlive(value)
		return err // Executor independently reads the actual result; no fallback.
	}
	if op.Kind != "insert_text" && op.Kind != "replace_selection" {
		return ErrInvalidControl
	}
	batches, err := documentUnicodeBatches(*op.Text)
	if err != nil {
		return err
	}
	span, err := DocumentRangeToNative(before.Text, *before.Selection, TextUnitUTF8)
	if err != nil {
		return err
	}
	if op.Kind == "insert_text" && span.Start != span.End || op.Kind == "replace_selection" && span.Start == span.End {
		return ErrDocumentRange
	}
	if len(batches) == 0 {
		if op.Kind == "insert_text" {
			return nil
		}
		if err = check(before.Text, *before.Selection); err != nil {
			return err
		}
		return sendDocumentPackets([]documentKeyboardPacket{{Type: 1, VK: 0x2e}, {Type: 1, VK: 0x2e, Flags: 2}})
	}
	prefix, suffix := before.Text[:span.Start], before.Text[span.End:]
	currentText, selection := before.Text, *before.Selection
	var inserted strings.Builder
	for _, batch := range batches {
		part, err := documentPacketText(batch)
		if err != nil {
			return err
		}
		if err = check(currentText, selection); err != nil {
			return err
		}
		if err = sendDocumentPackets(batch); err != nil {
			return err
		}
		inserted.WriteString(part)
		currentText = prefix + inserted.String() + suffix
		caret := before.Selection.Start + utf8.RuneCountInString(inserted.String())
		selection = DocumentTextRange{Start: caret, End: caret}
		// Poll read-back only. A timeout/partial write never resends this batch.
		deadline := time.Now().Add(2 * time.Second)
		for {
			// Losing focus/authority/composition is terminal even if the user
			// restores it during the read-back window. Never resume queued input.
			if err = guard(); err != nil {
				return ErrUncertain
			}
			actual, selected, composition, readErr := readUIADocument(editor)
			if readErr != nil || composition != "stable" {
				return ErrUncertain
			}
			if actual == currentText && selected != nil && *selected == selection {
				break
			}
			if !time.Now().Before(deadline) {
				return ErrUncertain
			}
			timer := time.NewTimer(25 * time.Millisecond)
			select {
			case <-ctx.Done():
				timer.Stop()
				return ErrUncertain
			case <-timer.C:
			}
		}
	}
	return nil
}
