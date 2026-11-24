//go:build windows

package main

import (
	"context"
	"errors"
	"io"
	"os"
	"time"
	"unicode/utf16"
	"unsafe"

	"github.com/erikgeiser/coninput"
	"golang.org/x/sys/windows"
)

var workspacePeekPipe = windows.NewLazySystemDLL("kernel32.dll").NewProc("PeekNamedPipe")

func openWorkspaceKeyHandle(path string) (*os.File, error) { return os.Open(path) }

func flushWorkspaceTerminalInput(file *os.File) error {
	return windows.FlushConsoleInputBuffer(windows.Handle(file.Fd()))
}

func readWorkspaceSecretFile(ctx context.Context, file *os.File, p []byte) (int, error) {
	if len(p) == 0 {
		return 0, nil
	}
	handle := windows.Handle(file.Fd())
	kind, err := windows.GetFileType(handle)
	if err != nil {
		return 0, err
	}
	for {
		if err := ctx.Err(); err != nil {
			return 0, err
		}
		if kind == windows.FILE_TYPE_PIPE {
			var available uint32
			ok, _, callErr := workspacePeekPipe.Call(uintptr(handle), 0, 0, 0, uintptr(unsafe.Pointer(&available)), 0)
			if ok == 0 {
				if errors.Is(callErr, windows.ERROR_BROKEN_PIPE) {
					return 0, io.EOF
				}
				return 0, callErr
			}
			if available == 0 {
				select {
				case <-ctx.Done():
					return 0, ctx.Err()
				case <-time.After(50 * time.Millisecond):
					continue
				}
			}
			p = p[:min(len(p), int(available))]
		} else if kind != windows.FILE_TYPE_DISK {
			return 0, errors.New("unsupported redirected passphrase input")
		}
		return file.Read(p)
	}
}

func workspaceTerminalInput(ctx context.Context, file *os.File) func() ([]byte, error) {
	handle := windows.Handle(file.Fd())
	var high rune
	var repeated []byte
	var repeats uint16
	return func() ([]byte, error) {
		for {
			if err := ctx.Err(); err != nil {
				return nil, err
			}
			if repeats > 0 {
				repeats--
				return repeated, nil
			}
			ready, err := windows.WaitForSingleObject(handle, 100)
			if err != nil {
				return nil, err
			}
			if ready == uint32(windows.WAIT_TIMEOUT) {
				continue
			}
			if ready != windows.WAIT_OBJECT_0 {
				return nil, errors.New("terminal input unavailable")
			}
			records, err := coninput.ReadNConsoleInputs(handle, 1)
			if err != nil {
				return nil, err
			}
			if len(records) != 1 {
				continue
			}
			key, ok := records[0].Unwrap().(coninput.KeyEventRecord)
			if !ok || !key.KeyDown || key.Char == 0 {
				continue
			}
			value := key.Char
			if high != 0 {
				if value < 0xdc00 || value > 0xdfff {
					return nil, errors.New("invalid terminal Unicode input")
				}
				value = utf16.DecodeRune(high, value)
				high = 0
			} else if value >= 0xd800 && value <= 0xdbff {
				high = value
				continue
			} else if value >= 0xdc00 && value <= 0xdfff {
				return nil, errors.New("invalid terminal Unicode input")
			}
			repeated = []byte(string(value))
			if key.RepeatCount > 0 {
				repeats = key.RepeatCount - 1
			}
			return repeated, nil
		}
	}
}
