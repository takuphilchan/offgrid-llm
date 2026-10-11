//go:build windows

package main

// All fixture controls belong to this disposable process. The private stdin
// channel configures fault injection and reads synthetic state; there is no
// network listener, arbitrary command, filesystem write or production bypass.
import (
	"bufio"
	"encoding/json"
	"os"
	"runtime"
	"sync"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

type fixtureTab struct {
	ID    int    `json:"id"`
	Text  string `json:"text"`
	Start uint32 `json:"start"`
	End   uint32 `json:"end"`
}
type fixtureRequest struct {
	Kind  string `json:"kind"`
	Index int    `json:"index,omitempty"`
	Delay int    `json:"delay_ms,omitempty"`
	Start uint32 `json:"start,omitempty"`
	End   uint32 `json:"end,omitempty"`
	Text  string `json:"text,omitempty"`
}
type fixtureTabItem struct {
	Mask, State, StateMask uint32
	Text                   *uint16
	MaxText, Image         int32
	Data                   uintptr
}
type fixtureNotification struct {
	Window, ID uintptr
	Code       int32
}

func runDocumentFixture() {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	u := windows.NewLazySystemDLL("user32.dll")
	common := windows.NewLazySystemDLL("comctl32.dll")
	initialization := struct{ Size, Classes uint32 }{8, 8}
	if value, _, _ := common.NewProc("InitCommonControlsEx").Call(uintptr(unsafe.Pointer(&initialization))); value == 0 {
		return
	}
	if windows.NewLazySystemDLL("msftedit.dll").Load() != nil {
		return
	}
	ptr := func(value string) *uint16 { p, _ := windows.UTF16PtrFromString(value); return p }
	create := func(class, title string, style uintptr, x, y, width, height int, parent, id uintptr) uintptr {
		c, n := ptr(class), ptr(title)
		h, _, _ := u.NewProc("CreateWindowExW").Call(0, uintptr(unsafe.Pointer(c)), uintptr(unsafe.Pointer(n)), style, uintptr(x), uintptr(y), uintptr(width), uintptr(height), parent, id, 0, 0)
		runtime.KeepAlive(c)
		runtime.KeepAlive(n)
		return h
	}
	root := create("STATIC", "OffGrid disposable document fixture", 0x10CF0000, 180, 180, 680, 380, 0, 0)
	if root == 0 {
		return
	}
	defer u.NewProc("DestroyWindow").Call(root)
	tabs := create("SysTabControl32", "Documents", 0x50010000, 16, 16, 636, 36, root, 10)
	// One RichEdit HWND is deliberately reused between all tabs.
	edit := create("RICHEDIT50W", "", 0x50A110C4, 16, 66, 636, 206, root, 11)
	button := create("BUTTON", "New blank document", 0x50010000, 16, 288, 220, 32, root, 12)
	focusSink := create("EDIT", "Owned focus-loss target", 0x50810080, 264, 288, 330, 32, root, 13)
	if tabs == 0 || edit == 0 || button == 0 || focusSink == 0 {
		return
	}
	send := func(hwnd uintptr, message uint32, a, b uintptr) uintptr {
		v, _, _ := u.NewProc("SendMessageW").Call(hwnd, uintptr(message), a, b)
		return v
	}
	setText := func(value string) {
		p := ptr(value)
		u.NewProc("SetWindowTextW").Call(edit, uintptr(unsafe.Pointer(p)))
		runtime.KeepAlive(p)
	}
	state := []fixtureTab{{ID: 1, Text: "Synthetic lifecycle content"}, {ID: 2, Text: "Untouched — café 日本語 12345"}}
	current, nextID, delay, partialBudget := 0, 3, 0, -1
	store := func() {
		size := send(edit, 0x000E, 0, 0)
		if size > 65536 {
			return
		}
		text := make([]uint16, size+1)
		send(edit, 0x000D, uintptr(len(text)), uintptr(unsafe.Pointer(&text[0])))
		state[current].Text = windows.UTF16ToString(text)
		send(edit, 0x00B0, uintptr(unsafe.Pointer(&state[current].Start)), uintptr(unsafe.Pointer(&state[current].End)))
	}
	load := func() {
		setText(state[current].Text)
		send(edit, 0x00B1, uintptr(state[current].Start), uintptr(state[current].End))
	}
	insertTab := func(index int) {
		caption := ptr("Untitled") // intentionally duplicated captions
		item := fixtureTabItem{Mask: 1, Text: caption}
		send(tabs, 0x133E, uintptr(index), uintptr(unsafe.Pointer(&item)))
		runtime.KeepAlive(caption)
	}
	insertTab(0)
	insertTab(1)
	load()
	selectTab := func(index int) {
		if index < 0 || index >= len(state) {
			return
		}
		store()
		current = index
		send(tabs, 0x130C, uintptr(index), 0)
		load()
	}
	newTab := func() {
		if len(state) >= 16 {
			return
		}
		store()
		state = append(state, fixtureTab{ID: nextID})
		nextID++
		insertTab(len(state) - 1)
		current = len(state) - 1
		send(tabs, 0x130C, uintptr(current), 0)
		load()
		u.NewProc("SetFocus").Call(edit)
	}
	var queueMu sync.Mutex
	queue := []fixtureRequest{}
	encoder := json.NewEncoder(os.Stdout)
	respond := func(kind string) {
		store()
		encoder.Encode(map[string]any{"kind": kind, "pid": os.Getpid(), "window": root, "editor": edit, "selected": current, "documents": state, "delay_ms": delay})
	}
	oldEdit := uintptr(0)
	editCallback := syscall.NewCallback(func(w uintptr, m uint32, a, b uintptr) uintptr {
		if m == 0x0102 && partialBudget >= 0 {
			if partialBudget == 0 {
				u.NewProc("SetFocus").Call(focusSink)
				return 0
			}
			partialBudget--
		}
		v, _, _ := u.NewProc("CallWindowProcW").Call(oldEdit, w, uintptr(m), a, b)
		return v
	})
	oldEdit, _, _ = u.NewProc("SetWindowLongPtrW").Call(edit, ^uintptr(3), editCallback)
	oldRoot := uintptr(0)
	rootCallback := syscall.NewCallback(func(w uintptr, m uint32, a, b uintptr) uintptr {
		switch m {
		case 0x0010:
			u.NewProc("PostQuitMessage").Call(0)
			return 0
		case 0x0113:
			if a == 2 {
				u.NewProc("KillTimer").Call(root, 2)
				newTab()
			} else {
				u.NewProc("PostQuitMessage").Call(0)
			}
			return 0
		case 0x0111:
			if a&0xffff == 12 && (a>>16)&0xffff == 0 {
				if delay > 0 {
					u.NewProc("SetTimer").Call(root, 2, uintptr(delay), 0)
				} else {
					newTab()
				}
				return 0
			}
		case 0x004E:
			if b != 0 {
				notice := (*fixtureNotification)(unsafe.Pointer(b))
				if notice.Window == tabs && notice.Code == -551 {
					selectTab(int(send(tabs, 0x130B, 0, 0)))
					return 0
				}
			}
		case 0x8010:
			queueMu.Lock()
			pending := queue
			queue = nil
			queueMu.Unlock()
			for _, req := range pending {
				switch req.Kind {
				case "state":
				case "select":
					selectTab(req.Index)
				case "new":
					if delay > 0 {
						u.NewProc("SetTimer").Call(root, 2, uintptr(delay), 0)
					} else {
						newTab()
					}
				case "delay":
					if req.Delay < 0 || req.Delay > 2000 {
						continue
					}
					delay = req.Delay
				case "selection":
					send(edit, 0x00B1, uintptr(req.Start), uintptr(req.End))
				case "text":
					if len(req.Text) > 16000 {
						continue
					}
					p := ptr(req.Text)
					send(edit, 0x00C2, 1, uintptr(unsafe.Pointer(p)))
					runtime.KeepAlive(p)
				case "focus_loss":
					u.NewProc("SetFocus").Call(focusSink)
				case "partial_input":
					if req.Index < 0 || req.Index > 16000 {
						continue
					}
					partialBudget = req.Index
				case "quit":
					u.NewProc("PostQuitMessage").Call(0)
				default:
					continue
				}
				respond(req.Kind)
			}
			return 0
		}
		v, _, _ := u.NewProc("CallWindowProcW").Call(oldRoot, w, uintptr(m), a, b)
		return v
	})
	oldRoot, _, _ = u.NewProc("SetWindowLongPtrW").Call(root, ^uintptr(3), rootCallback)
	u.NewProc("SetTimer").Call(root, 1, 120000, 0) // bounded cleanup after an abandoned test
	respond("ready")
	go func() {
		scanner := bufio.NewScanner(os.Stdin)
		scanner.Buffer(make([]byte, 4096), 20000)
		for scanner.Scan() {
			var req fixtureRequest
			if json.Unmarshal(scanner.Bytes(), &req) != nil {
				continue
			}
			queueMu.Lock()
			if len(queue) < 16 {
				queue = append(queue, req)
			}
			queueMu.Unlock()
			u.NewProc("PostMessageW").Call(root, 0x8010, 0, 0)
		}
	}()
	type message struct {
		Window         uintptr
		Message        uint32
		WParam, LParam uintptr
		Time           uint32
		X, Y           int32
		Private        uint32
	}
	var msg message
	for {
		v, _, _ := u.NewProc("GetMessageW").Call(uintptr(unsafe.Pointer(&msg)), 0, 0, 0)
		if int32(v) <= 0 {
			return
		}
		u.NewProc("TranslateMessage").Call(uintptr(unsafe.Pointer(&msg)))
		u.NewProc("DispatchMessageW").Call(uintptr(unsafe.Pointer(&msg)))
	}
}
