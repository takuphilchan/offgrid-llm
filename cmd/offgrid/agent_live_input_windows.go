//go:build windows

package main

import (
	"fmt"
	"os"
	"unicode/utf16"
	"unsafe"

	"golang.org/x/sys/windows"
)

// Read only available native records. No blocking/background reader survives
// the handoff to an approval. Unlike cancelreader, this never flushes input or
// relies on cancelling a Windows console ReadFile operation.
func newAgentInputPoller() (func() ([]byte, error), error) {
	dll := windows.NewLazySystemDLL("kernel32.dll")
	count := dll.NewProc("GetNumberOfConsoleInputEvents")
	read := dll.NewProc("ReadConsoleInputW")
	if err := count.Find(); err != nil {
		return nil, err
	}
	if err := read.Find(); err != nil {
		return nil, err
	}
	var high uint16
	return func() ([]byte, error) {
		// Key-up, mouse and resize records can precede pending text. Do not
		// report an empty queue until those records have actually been drained.
		for batch := 0; batch < 64; batch++ {
			var available uint32
			if ok, _, err := count.Call(os.Stdin.Fd(), uintptr(unsafe.Pointer(&available))); ok == 0 {
				return nil, err
			}
			if available == 0 {
				return nil, nil
			}
			type record struct {
				kind, pad               uint16
				down                    int32
				repeat, key, scan, char uint16
				control                 uint32
			}
			var records [64]record
			var used uint32
			if ok, _, err := read.Call(os.Stdin.Fd(), uintptr(unsafe.Pointer(&records[0])), uintptr(min(available, 64)), uintptr(unsafe.Pointer(&used))); ok == 0 {
				return nil, err
			}
			var result []byte
			for _, event := range records[:used] {
				if event.kind != 1 || event.down == 0 {
					continue
				}
				text := ""
				if event.char >= 0xD800 && event.char <= 0xDBFF {
					high = event.char
					continue
				}
				if high != 0 {
					text = string(utf16.DecodeRune(rune(high), rune(event.char)))
					high = 0
				} else if event.char != 0 {
					text = string(rune(event.char))
				} else {
					text = map[uint16]string{0x25: "\x1b[D", 0x26: "\x1b[A", 0x27: "\x1b[C", 0x28: "\x1b[B", 0x24: "\x1b[H", 0x23: "\x1b[F", 0x2e: "\x1b[3~"}[event.key]
				}
				if event.key == 0x0d && event.control&0x13 != 0 {
					text = "\n"
				} // Shift/Alt Enter
				for i := 0; i < int(max(1, event.repeat)); i++ {
					result = append(result, text...)
				}
			}
			if len(result) > 0 {
				return result, nil
			}
		}
		return nil, fmt.Errorf("console input remains queued; input ownership cannot be transferred")
	}, nil
}
