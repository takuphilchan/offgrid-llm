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
// inference. Ctrl+Alt+Shift+F12 and OS session loss revoke input authority.
// Desktop-owned workers use a hidden monitor; standalone workers also have a button.
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
		// Revoke for the session event itself, even if it unlocks again before
		// the next request. A later successful desktop check cannot revive access.
		sessionLost := message == 0x02B1 && (wparam == 2 || wparam == 4 || wparam == 6 || wparam == 7 || wparam == 0x0B)
		suspending := message == 0x0218 && wparam == 4
		if sessionLost || suspending || message == 0x0312 || message == 0x0010 || message == 0x0111 && wparam == 1 || message == 0x0112 && wparam&0xfff0 == 0xf060 {
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
	return startNativeStopSurface(revoke, true)
}

// StartNativeStopMonitor retains the independent hotkey/session message thread
// without another visible window. The owning desktop provides Stop over private
// IPC and must terminate this worker if its app controls become unavailable.
func StartNativeStopMonitor(revoke func()) (*NativeStopSurface, error) {
	return startNativeStopSurface(revoke, false)
}

func startNativeStopSurface(revoke func(), visible bool) (*NativeStopSurface, error) {
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
		// Small, non-activating utility at the edge of the work area. This is an
		// independent escape control, not a modal approval or another task window.
		var work struct{ Left, Top, Right, Bottom int32 }
		ok, _, _ := user32.NewProc("SystemParametersInfoW").Call(0x0030, 0, uintptr(unsafe.Pointer(&work)), 0)
		x, y := int32(40), int32(40)
		if ok != 0 {
			x, y = work.Right-300, work.Top+12
		}
		hwnd, _, err := create.Call(0x00000008|0x00000080|0x08000000, uintptr(unsafe.Pointer(class)), uintptr(unsafe.Pointer(title)), 0x80800000, uintptr(x), uintptr(y), 288, 52, 0, 0, 0, 0)
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
		wts := windows.NewLazySystemDLL("wtsapi32.dll")
		registeredSession, _, _ := wts.NewProc("WTSRegisterSessionNotification").Call(hwnd, 0) // this OS session only
		if registeredSession == 0 {
			ready <- started{err: fmt.Errorf("computer_stop_surface_unavailable")}
			return
		}
		defer wts.NewProc("WTSUnRegisterSessionNotification").Call(hwnd)
		buttonClass, _ := windows.UTF16PtrFromString("BUTTON")
		label, _ := windows.UTF16PtrFromString("OffGrid · Stop access")
		button, _, _ := create.Call(0, uintptr(unsafe.Pointer(buttonClass)), uintptr(unsafe.Pointer(label)), 0x50010000, 6, 6, 274, 38, hwnd, 1, 0, 0)
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
		if visible {
			user32.NewProc("ShowWindow").Call(hwnd, 4) // SW_SHOWNOACTIVATE
		}
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
