//go:build windows && amd64

package computer

import (
	"context"
	"fmt"
	"image"
	"image/png"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"testing"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

const consentFixtureText = "Enable Full access for this desktop session?\n\nLet tasks work across supported apps without asking again this session.\n\n60 minutes or 500 changes per task. No direct file access; sensitive actions stay blocked.\n\nStop at any time. Access clears on quit, lock, suspend or account/workspace change.\n\nAccess details\nAlways blocked\nCredentials, payments, financial transactions, privilege/security changes, installation, scripts, permanent deletion and uncertain retries.\n\nPermission is never saved to disk.\n\nConnected workspace\nService: http://127.0.0.1:11611\nWorkspace: owned-consent-layout-fixture"

func TestConsentFonts(t *testing.T) {
	d := &nativeConsent{}
	d.setFonts(144)
	defer d.dispose()
	dc, _, _ := consentGDI.NewProc("CreateCompatibleDC").Call(0)
	defer consentGDI.NewProc("DeleteDC").Call(dc)
	for i, font := range d.fonts {
		previous, _, _ := consentGDI.NewProc("SelectObject").Call(dc, font)
		var face [64]uint16
		consentGDI.NewProc("GetTextFaceW").Call(dc, 64, uintptr(unsafe.Pointer(&face[0])))
		consentGDI.NewProc("SelectObject").Call(dc, previous)
		want := "Segoe UI"
		if i > 0 {
			want = "Segoe UI Semibold"
		}
		if actual := windows.UTF16ToString(face[:]); actual != want {
			t.Errorf("consent font substituted: %s", actual)
		}
	}
}

func TestNativeConsentFixtureHost(t *testing.T) {
	if os.Getenv("OFFGRID_CONSENT_FIXTURE_CHILD") != "1" {
		t.Skip("owned fixture only")
	}
	if os.Getenv("OFFGRID_CONSENT_FIXTURE_DPI") == "200" {
		go func() {
			var dialog uintptr
			callback := syscall.NewCallback(func(hwnd, _ uintptr) uintptr {
				var pid uint32
				user32.NewProc("GetWindowThreadProcessId").Call(hwnd, uintptr(unsafe.Pointer(&pid)))
				visible, _, _ := user32.NewProc("IsWindowVisible").Call(hwnd)
				if pid == uint32(os.Getpid()) && visible != 0 && nativeConsentTestText(hwnd) == "OffGrid computer access" {
					dialog = hwnd
					return 0
				}
				return 1
			})
			for i := 0; i < 200 && dialog == 0; i++ {
				user32.NewProc("EnumWindows").Call(callback, 0)
				time.Sleep(10 * time.Millisecond)
			}
			if dialog != 0 {
				r := consentRect{Left: 20, Top: 20, Right: 900, Bottom: 680}
				// Pointer stays in this fixture process; no cross-process marshalling.
				user32.NewProc("SendMessageW").Call(dialog, 0x02e0, 192|(192<<16), uintptr(unsafe.Pointer(&r)))
			}
		}()
	}
	fmt.Printf("CONSENT_RESULT %t\n", NativeConfirm(consentFixtureText))
}

// Runs real native dialogs only in owned child processes with a synthetic
// permission summary. It never sends approval/input to an installed worker.
func TestNativeConsentPresentation(t *testing.T) {
	if os.Getenv("OFFGRID_TEST_NATIVE_WINDOWS") != "1" {
		t.Skip("opt-in owned native consent fixture")
	}
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	previousDPI, _, _ := user32.NewProc("SetThreadDpiAwarenessContext").Call(^uintptr(3))
	if previousDPI != 0 {
		defer user32.NewProc("SetThreadDpiAwarenessContext").Call(previousDPI)
	}
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	for _, action := range []string{"allow", "allow-keyboard", "cancel", "cancel-keyboard", "close", "escape", "enter", "bare-command", "details", "details-keyboard", "short", "200dpi"} {
		t.Run(action, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
			defer cancel()
			cmd := exec.CommandContext(ctx, exe, "-test.run=^TestNativeConsentFixtureHost$", "-test.v")
			cmd.Env = append(os.Environ(), "OFFGRID_CONSENT_FIXTURE_CHILD=1")
			if action == "200dpi" {
				cmd.Env = append(cmd.Env, "OFFGRID_CONSENT_FIXTURE_DPI=200")
			}
			cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
			out, err := cmd.StdoutPipe()
			if err != nil {
				t.Fatal(err)
			}
			cmd.Stderr = os.Stderr
			if err := cmd.Start(); err != nil {
				t.Fatal(err)
			}
			defer func() { cancel(); cmd.Wait() }()
			var dialog uintptr
			var observed string
			callback := syscall.NewCallback(func(hwnd, _ uintptr) uintptr {
				var pid uint32
				user32.NewProc("GetWindowThreadProcessId").Call(hwnd, uintptr(unsafe.Pointer(&pid)))
				if pid != uint32(cmd.Process.Pid) {
					return 1
				}
				if title := nativeConsentTestText(hwnd); title != "Default IME" && title != "MSCTFIME UI" {
					observed = fmt.Sprintf("owned window %x title=%q", hwnd, title)
				}
				if nativeConsentTestText(hwnd) != "OffGrid computer access" {
					return 1
				}
				body, _, _ := user32.NewProc("GetDlgItem").Call(hwnd, 0xffff)
				visible, _, _ := user32.NewProc("IsWindowVisible").Call(hwnd)
				observed += fmt.Sprintf(" visible=%d body=%q", visible, nativeConsentTestText(body))
				if visible != 0 && strings.Contains(nativeConsentTestText(body), "60 minutes or 500 changes per task.") {
					dialog = hwnd
					return 0
				}
				return 1
			})
			for dialog == 0 && ctx.Err() == nil {
				user32.NewProc("EnumWindows").Call(callback, 0)
				time.Sleep(20 * time.Millisecond)
			}
			if dialog == 0 {
				cancel()
				output, _ := io.ReadAll(out)
				t.Fatalf("owned native consent not visible: %s output=%s", observed, output)
			}
			button := func(id uintptr) uintptr { h, _, _ := user32.NewProc("GetDlgItem").Call(dialog, id); return h }
			if nativeConsentTestText(button(6)) != "Enable access" || nativeConsentTestText(button(7)) != "Cancel" {
				t.Fatal("missing explicit action labels")
			}
			for _, id := range []uintptr{6, 7, 103} {
				style, _, _ := user32.NewProc("GetWindowLongPtrW").Call(button(id), ^uintptr(15))
				if style&15 != 11 {
					t.Fatalf("button %d lost custom styling: %x", id, style)
				}
			}
			style, _, _ := user32.NewProc("GetWindowLongPtrW").Call(button(0xffff), ^uintptr(15))
			if style&0x0844 != 0x0844 {
				t.Fatal("permission details must be read-only and scrollable")
			}
			defaultID, _, _ := user32.NewProc("SendMessageW").Call(dialog, 0x0400, 0, 0)
			if defaultID&0xffff != 7 {
				t.Fatal("approval became default")
			}
			if action == "200dpi" {
				time.Sleep(150 * time.Millisecond)
			}
			var initial consentRect
			user32.NewProc("GetWindowRect").Call(dialog, uintptr(unsafe.Pointer(&initial)))
			dpi, _, _ := user32.NewProc("GetDpiForWindow").Call(dialog)
			if action == "200dpi" {
				dpi = 192 // synthetic WM_DPICHANGED does not change the physical monitor
			}
			if initial.Right-initial.Left > int32(440*dpi/96)+2 || initial.Bottom-initial.Top > int32(330*dpi/96)+2 {
				t.Fatalf("default consent is page-sized: %+v at %d DPI", initial, dpi)
			}
			body := nativeConsentTestText(button(0xffff))
			for _, required := range []string{"without asking again this session", "60 minutes or 500 changes", "No direct file access", "sensitive actions stay blocked", "Stop at any time", "account/workspace change"} {
				if !strings.Contains(body, required) {
					t.Fatalf("consent summary hid %q", required)
				}
			}
			if strings.Contains(body, "owned-consent-layout-fixture") || nativeConsentTestText(button(103)) != "Details" {
				t.Fatal("technical metadata should initially be collapsed")
			}
			if action == "details" || action == "details-keyboard" {
				if action == "details-keyboard" {
					user32.NewProc("SendMessageW").Call(dialog, 0x0028, button(103), 1)
					user32.NewProc("SendMessageW").Call(button(103), 0x0100, 0x0d, 0)
				} else {
					user32.NewProc("SendMessageW").Call(button(103), 0x00f5, 0, 0)
				}
				if !strings.Contains(nativeConsentTestText(button(0xffff)), "owned-consent-layout-fixture") || !strings.Contains(nativeConsentTestText(button(0xffff)), "uncertain retries") || nativeConsentTestText(button(103)) != "Hide details" {
					t.Fatal("connection details did not expand")
				}
				user32.NewProc("SendMessageW").Call(button(103), 0x00f5, 0, 0)
				if strings.Contains(nativeConsentTestText(button(0xffff)), "owned-consent-layout-fixture") {
					t.Fatal("connection details did not collapse")
				}
				var collapsed consentRect
				user32.NewProc("GetWindowRect").Call(dialog, uintptr(unsafe.Pointer(&collapsed)))
				if collapsed.Bottom-collapsed.Top != initial.Bottom-initial.Top {
					t.Fatal("collapsing details did not restore compact height")
				}
				defaultID, _, _ := user32.NewProc("SendMessageW").Call(dialog, 0x0400, 0, 0)
				if defaultID&0xffff != 7 {
					t.Fatal("details changed the consent default")
				}
			}
			if action == "short" {
				user32.NewProc("SendMessageW").Call(button(103), 0x00f5, 0, 0)
				user32.NewProc("SetWindowPos").Call(dialog, 0, 20, 20, 360*dpi/96, 300*dpi/96, 4)
				user32.NewProc("SendMessageW").Call(button(0xffff), 0x00b6, 0, 200) // EM_LINESCROLL
				line, _, _ := user32.NewProc("SendMessageW").Call(button(0xffff), 0x00ce, 0, 0)
				if line == 0 {
					t.Fatal("short dialog did not scroll")
				}
				var client, allow consentRect
				user32.NewProc("GetWindowRect").Call(dialog, uintptr(unsafe.Pointer(&client)))
				user32.NewProc("GetWindowRect").Call(button(6), uintptr(unsafe.Pointer(&allow)))
				if allow.Bottom > client.Bottom || allow.Top < client.Top {
					t.Fatal("actions clipped on short screen")
				}
				var details, cancel consentRect
				user32.NewProc("GetWindowRect").Call(button(103), uintptr(unsafe.Pointer(&details)))
				user32.NewProc("GetWindowRect").Call(button(7), uintptr(unsafe.Pointer(&cancel)))
				if details.Right >= cancel.Left || cancel.Right >= allow.Left {
					t.Fatal("footer actions overlap on short/narrow screen")
				}
			}
			if action == "cancel-keyboard" {
				user32.NewProc("SendMessageW").Call(dialog, 0x0127, 2|(1<<16), 0) // UI focus cues visible
				user32.NewProc("SendMessageW").Call(dialog, 0x0028, button(7), 1)
			}
			if dir := os.Getenv("OFFGRID_NATIVE_CONSENT_SCREENSHOTS"); dir != "" && (action == "allow" || action == "cancel-keyboard" || action == "short" || action == "200dpi") {
				if err := captureConsentFixture(dialog, filepath.Join(dir, action+".png")); err != nil {
					t.Fatal(err)
				}
			}
			switch action {
			case "allow":
				user32.NewProc("SendMessageW").Call(button(6), 0x00f5, 0, 0)
			case "allow-keyboard":
				user32.NewProc("SendMessageW").Call(dialog, 0x0028, button(6), 1)
				user32.NewProc("PostMessageW").Call(button(6), 0x0100, 0x0d, 0)
			case "bare-command":
				user32.NewProc("SendMessageW").Call(dialog, 0x0111, 6, 0)
				user32.NewProc("SendMessageW").Call(button(7), 0x00f5, 0, 0)
			case "close":
				user32.NewProc("PostMessageW").Call(dialog, 0x0010, 0, 0)
			case "escape", "enter", "cancel-keyboard":
				key := uintptr(0x1b)
				if action != "escape" {
					key = 0x0d
				}
				user32.NewProc("PostMessageW").Call(button(7), 0x0100, key, 0)
			default:
				user32.NewProc("SendMessageW").Call(button(7), 0x00f5, 0, 0)
			}
			output, err := io.ReadAll(out)
			if err != nil {
				t.Fatal(err)
			}
			if err := cmd.Wait(); err != nil {
				t.Fatalf("fixture did not exit: %v %s", err, output)
			}
			want := "CONSENT_RESULT false"
			if action == "allow" || action == "allow-keyboard" {
				want = "CONSENT_RESULT true"
			}
			if !strings.Contains(string(output), want) {
				t.Fatalf("unsafe result: %s", output)
			}
		})
	}
}

func nativeConsentTestText(hwnd uintptr) string {
	var text [8192]uint16
	user32.NewProc("SendMessageW").Call(hwnd, 0x000d, 8192, uintptr(unsafe.Pointer(&text[0])))
	return windows.UTF16ToString(text[:])
}

// PrintWindow targets this already PID/fixture-validated dialog only. It never
// captures the user's desktop, other windows, or a real permission summary.
func captureConsentFixture(hwnd uintptr, path string) error {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	previousDPI, _, _ := user32.NewProc("SetThreadDpiAwarenessContext").Call(^uintptr(3))
	if previousDPI != 0 {
		defer user32.NewProc("SetThreadDpiAwarenessContext").Call(previousDPI)
	}
	var rect consentRect
	user32.NewProc("GetWindowRect").Call(hwnd, uintptr(unsafe.Pointer(&rect)))
	w, h := int(rect.Right-rect.Left), int(rect.Bottom-rect.Top)
	if w <= 0 || h <= 0 || w*h > 16_000_000 {
		return fmt.Errorf("invalid fixture size")
	}
	dc, _, _ := user32.NewProc("GetWindowDC").Call(hwnd)
	defer user32.NewProc("ReleaseDC").Call(hwnd, dc)
	copyDC, _, _ := consentGDI.NewProc("CreateCompatibleDC").Call(dc)
	defer consentGDI.NewProc("DeleteDC").Call(copyDC)
	bitmap, _, _ := consentGDI.NewProc("CreateCompatibleBitmap").Call(dc, uintptr(w), uintptr(h))
	defer consentGDI.NewProc("DeleteObject").Call(bitmap)
	previous, _, _ := consentGDI.NewProc("SelectObject").Call(copyDC, bitmap)
	if ok, _, _ := user32.NewProc("PrintWindow").Call(hwnd, copyDC, 2); ok == 0 {
		return fmt.Errorf("fixture capture failed")
	}
	consentGDI.NewProc("SelectObject").Call(copyDC, previous)
	var info struct {
		Size                   uint32
		Width, Height          int32
		Planes, Bits           uint16
		Compression, ImageSize uint32
		XPels, YPels           int32
		Used, Important        uint32
	}
	info.Size, info.Width, info.Height, info.Planes, info.Bits = 40, int32(w), -int32(h), 1, 32
	pixels := make([]byte, w*h*4)
	if ok, _, _ := consentGDI.NewProc("GetDIBits").Call(copyDC, bitmap, 0, uintptr(h), uintptr(unsafe.Pointer(&pixels[0])), uintptr(unsafe.Pointer(&info)), 0); ok == 0 {
		return fmt.Errorf("fixture pixels unavailable")
	}
	for i := 0; i < len(pixels); i += 4 {
		pixels[i], pixels[i+2], pixels[i+3] = pixels[i+2], pixels[i], 255
	}
	file, err := os.Create(path)
	if err != nil {
		return err
	}
	defer file.Close()
	return png.Encode(file, &image.RGBA{Pix: pixels, Stride: w * 4, Rect: image.Rect(0, 0, w, h)})
}
