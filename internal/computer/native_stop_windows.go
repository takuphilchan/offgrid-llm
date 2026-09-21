//go:build windows && amd64

package computer

import (
	"fmt"
	"runtime"
	"sync"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

// NativeStopSurface runs on its own message thread, independent of UIA and
// inference. Its visible button and Ctrl+Alt+Shift+F12 revoke input authority.
// The process owner must terminate a blocked provider after revocation; this
// window does not claim that an already dispatched operation was undone.
type NativeStopSurface struct {
	window uintptr
	button uintptr
	done   chan struct{}
	once   sync.Once
}

type nativeMessage struct {
	Window  uintptr
	Message uint32
	WParam  uintptr
	LParam  uintptr
	Time    uint32
	X, Y    int32
	Private uint32
}

type nativeStopHandler struct {
	revoke   func()
	previous uintptr
}

var nativeStopHandlers sync.Map
var nativeStopProcedure = syscall.NewCallback(func(hwnd uintptr, message uint32, wparam, lparam uintptr) uintptr {
	if entry, ok := nativeStopHandlers.Load(hwnd); ok {
		handler := entry.(nativeStopHandler)
		if message == 0x0312 || message == 0x0010 || message == 0x0111 && wparam == 1 || message == 0x0112 && wparam&0xfff0 == 0xf060 {
			handler.revoke()
			user32.NewProc("PostMessageW").Call(hwnd, 0x8002, 0, 0)
			return 0
		}
		value, _, _ := user32.NewProc("CallWindowProcW").Call(handler.previous, hwnd, uintptr(message), wparam, lparam)
		return value
	}
	value, _, _ := user32.NewProc("DefWindowProcW").Call(hwnd, uintptr(message), wparam, lparam)
	return value
})

func StartNativeStopSurface(revoke func()) (*NativeStopSurface, error) {
	type started struct {
		surface *NativeStopSurface
		err     error
	}
	ready := make(chan started, 1)
	go func() {
		runtime.LockOSThread()
		defer runtime.UnlockOSThread()
		// Built-in STATIC/BUTTON classes share a single process-wide callback;
		// UIA Invoke, mouse and keyboard activation all reach WM_COMMAND.
		class, _ := windows.UTF16PtrFromString("STATIC")
		title, _ := windows.UTF16PtrFromString("OffGrid computer control — Ctrl+Alt+Shift+F12 stops")
		create := user32.NewProc("CreateWindowExW")
		hwnd, _, err := create.Call(0x00000008, uintptr(unsafe.Pointer(class)), uintptr(unsafe.Pointer(title)), 0x10C80000, 40, 40, 620, 125, 0, 0, 0, 0)
		runtime.KeepAlive(class)
		runtime.KeepAlive(title)
		if hwnd == 0 {
			ready <- started{err: fmt.Errorf("computer_stop_surface_unavailable: %w", err)}
			return
		}
		defer user32.NewProc("DestroyWindow").Call(hwnd)
		previous, _, _ := user32.NewProc("SetWindowLongPtrW").Call(hwnd, ^uintptr(3), nativeStopProcedure)
		if previous == 0 {
			ready <- started{err: fmt.Errorf("computer_stop_surface_unavailable")}
			return
		}
		nativeStopHandlers.Store(hwnd, nativeStopHandler{revoke: revoke, previous: previous})
		defer nativeStopHandlers.Delete(hwnd)
		buttonClass, _ := windows.UTF16PtrFromString("BUTTON")
		label, _ := windows.UTF16PtrFromString("Stop computer control")
		button, _, _ := create.Call(0, uintptr(unsafe.Pointer(buttonClass)), uintptr(unsafe.Pointer(label)), 0x50010000, 20, 20, 560, 40, hwnd, 1, 0, 0)
		runtime.KeepAlive(buttonClass)
		runtime.KeepAlive(label)
		if button == 0 {
			ready <- started{err: fmt.Errorf("computer_stop_surface_unavailable")}
			return
		}
		// Mandatory hotkey registration: silently losing the emergency key is
		// not acceptable. A conflicting application gets an actionable error.
		registered, _, _ := user32.NewProc("RegisterHotKey").Call(hwnd, 1, 0x4000|0x0001|0x0002|0x0004, 0x7B)
		if registered == 0 {
			ready <- started{err: fmt.Errorf("computer_emergency_shortcut_in_use")}
			return
		}
		defer user32.NewProc("UnregisterHotKey").Call(hwnd, 1)
		surface := &NativeStopSurface{window: hwnd, button: button, done: make(chan struct{})}
		defer close(surface.done)
		user32.NewProc("ShowWindow").Call(hwnd, 5)
		ready <- started{surface: surface}
		var msg nativeMessage
		for {
			value, _, _ := user32.NewProc("GetMessageW").Call(uintptr(unsafe.Pointer(&msg)), 0, 0, 0)
			if int32(value) <= 0 {
				revoke()
				return
			}
			if msg.Window == hwnd {
				if msg.Message == 0x8001 || msg.Message == 0x8002 {
					return
				}
			}
			user32.NewProc("TranslateMessage").Call(uintptr(unsafe.Pointer(&msg)))
			user32.NewProc("DispatchMessageW").Call(uintptr(unsafe.Pointer(&msg)))
		}
	}()
	result := <-ready
	return result.surface, result.err
}

func (surface *NativeStopSurface) Close() {
	if surface == nil {
		return
	}
	surface.once.Do(func() {
		user32.NewProc("PostMessageW").Call(surface.window, 0x8001, 0, 0)
		<-surface.done
	})
}
