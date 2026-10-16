//go:build windows && amd64

package computer

import (
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/registry"
)

// Consent remains an owned native modal, not a renderer message or a service-
// supplied approval. The dialog manager provides keyboard navigation and native
// accessibility; only the explicit Allow button returns approval. No web code,
// remote content, shell process, or persisted permission is involved.
func NativeConfirm(message string) bool {
	if strings.TrimSpace(message) == "" || strings.ContainsRune(message, 0) {
		return false
	}
	result := make(chan bool, 1)
	go func() {
		runtime.LockOSThread()
		defer runtime.UnlockOSThread()
		// The companion's UIA thread keeps its own DPI context.
		previous, _, _ := user32.NewProc("SetThreadDpiAwarenessContext").Call(^uintptr(3)) // PER_MONITOR_AWARE_V2
		if previous != 0 {
			defer user32.NewProc("SetThreadDpiAwarenessContext").Call(previous)
		}
		dialog := &nativeConsent{message: message}
		id := uintptr(nativeConsentSequence.Add(1))
		nativeConsentPending.Store(id, dialog)
		defer nativeConsentPending.Delete(id)
		defer dialog.dispose()
		// DLGTEMPLATE, no controls: child controls are created in WM_INITDIALOG.
		// Caption, close, resize, Segoe UI; never an affirmative default button.
		template := []uint16{0x08c0, 0x80cc, 0x0001, 0, 0, 0, 0, 400, 380, 0, 0}
		template = append(template, windows.StringToUTF16("OffGrid computer access")...)
		template = append(template, 11)
		template = append(template, windows.StringToUTF16("Segoe UI")...)
		value, _, _ := user32.NewProc("DialogBoxIndirectParamW").Call(0, uintptr(unsafe.Pointer(&template[0])), 0, nativeConsentProcedure, id)
		runtime.KeepAlive(template)
		result <- value == 6
	}()
	return <-result
}

type consentRect struct{ Left, Top, Right, Bottom int32 }
type nativeConsent struct {
	message, summary, details                            string
	expanded                                             bool
	window, heading, body, disclosure, cancel, allow     uintptr
	fonts                                                []uintptr
	background, foreground, border, primary, primaryText uint32
	brush                                                uintptr
	dpi                                                  int32
}

var nativeConsentSequence atomic.Uint64
var nativeConsentPending, nativeConsentWindows sync.Map
var nativeConsentButtonHandlers sync.Map
var consentGDI = windows.NewLazySystemDLL("gdi32.dll")

// Owner-drawn BUTTONs retain native Space/Tab behavior. Request Return only
// while that button has focus; otherwise the dialog's default remains Cancel.
var nativeConsentButtonProcedure = syscall.NewCallback(func(hwnd uintptr, msg uint32, wparam, lparam uintptr) uintptr {
	previous, ok := nativeConsentButtonHandlers.Load(hwnd)
	if !ok {
		v, _, _ := user32.NewProc("DefWindowProcW").Call(hwnd, uintptr(msg), wparam, lparam)
		return v
	}
	if msg == 0x0100 && wparam == 0x0d {
		user32.NewProc("SendMessageW").Call(hwnd, 0x00f5, 0, 0)
		return 0
	}
	value, _, _ := user32.NewProc("CallWindowProcW").Call(previous.(uintptr), hwnd, uintptr(msg), wparam, lparam)
	if msg == 0x0087 && lparam != 0 { // WM_GETDLGCODE
		key := (*nativeMessage)(unsafe.Pointer(lparam))
		if key.Message == 0x0100 && key.WParam == 0x0d {
			return value | 4
		}
	}
	if msg == 0x0082 {
		nativeConsentButtonHandlers.Delete(hwnd)
	}
	return value
})

func consentText(value string) *uint16 { p, _ := windows.UTF16PtrFromString(value); return p }

var nativeConsentProcedure = syscall.NewCallback(func(hwnd uintptr, msg uint32, wparam, lparam uintptr) uintptr {
	if msg == 0x0110 { // WM_INITDIALOG
		entry, ok := nativeConsentPending.Load(lparam)
		if !ok {
			user32.NewProc("EndDialog").Call(hwnd, 7)
			return 1
		}
		d := entry.(*nativeConsent)
		d.window = hwnd
		nativeConsentWindows.Store(hwnd, d)
		// We own font/layout scaling, including keeping the action row visible.
		user32.NewProc("SetDialogDpiChangeBehavior").Call(hwnd, 1, 1)
		if !d.create() {
			user32.NewProc("EndDialog").Call(hwnd, 7)
			return 1
		}
		user32.NewProc("SendMessageW").Call(hwnd, 0x0401, 7, 0) // DM_SETDEFID: Cancel
		user32.NewProc("SetFocus").Call(d.cancel)
		// The private worker is launched with SW_HIDE (no console). Show after
		// DialogBox has consumed that initial startup show state, not during init.
		user32.NewProc("PostMessageW").Call(hwnd, 0x8031, 0, 0)
		return 0 // keep explicitly selected safe focus
	}
	entry, ok := nativeConsentWindows.Load(hwnd)
	if !ok {
		return 0
	}
	d := entry.(*nativeConsent)
	switch msg {
	case 0x8031:
		user32.NewProc("ShowWindow").Call(hwnd, 5)
		user32.NewProc("SetForegroundWindow").Call(hwnd)
		user32.NewProc("SetFocus").Call(d.cancel)
		return 1
	case 0x0111: // WM_COMMAND; only a real button activation, not edit notifications
		id := wparam & 0xffff
		if wparam>>16 == 0 && id == 103 && lparam == d.disclosure {
			d.toggleDetails()
			return 1
		}
		if wparam>>16 == 0 && (id == 6 && lparam == d.allow || id == 7 || id == 2) {
			answer := uintptr(7)
			if id == 6 {
				answer = 6
			}
			user32.NewProc("EndDialog").Call(hwnd, answer)
			return 1
		}
	case 0x0010: // WM_CLOSE always denies
		user32.NewProc("EndDialog").Call(hwnd, 7)
		return 1
	case 0x0002: // WM_DESTROY
		nativeConsentWindows.Delete(hwnd)
	case 0x0005: // WM_SIZE
		d.layout()
		return 1
	case 0x0024: // WM_GETMINMAXINFO: keep heading and fixed actions usable
		limits := (*[10]int32)(unsafe.Pointer(lparam))
		limits[6], limits[7] = d.px(360), d.px(300)
		return 1
	case 0x02e0: // WM_DPICHANGED; refit to destination monitor's work area
		r := (*consentRect)(unsafe.Pointer(lparam))
		d.setFonts(int32(wparam & 0xffff))
		d.fit(r)
		return 1
	case 0x0136, 0x0138, 0x0133: // dialog/static/read-only edit colors
		consentGDI.NewProc("SetTextColor").Call(wparam, uintptr(d.foreground))
		consentGDI.NewProc("SetBkColor").Call(wparam, uintptr(d.background))
		return d.brush
	case 0x002b: // WM_DRAWITEM: real native BUTTONs, custom monochrome paint
		d.drawButton(lparam)
		return 1
	}
	return 0
})

func (d *nativeConsent) create() bool {
	d.background, d.foreground, d.border, d.primary, d.primaryText = 0xffffff, 0x181818, 0xd8d8d8, 0x181818, 0xffffff
	// Follow the local Windows app theme, not untrusted task content. High
	// contrast uses system colors. No app/theme preference creates permission.
	if key, err := registry.OpenKey(registry.CURRENT_USER, `Software\Microsoft\Windows\CurrentVersion\Themes\Personalize`, registry.QUERY_VALUE); err == nil {
		light, _, err := key.GetIntegerValue("AppsUseLightTheme")
		key.Close()
		if err == nil && light == 0 {
			d.background, d.foreground, d.border, d.primary, d.primaryText = 0x171717, 0xf3f3f3, 0x555555, 0xf3f3f3, 0x171717
		}
	}
	var highContrast struct {
		Size, Flags uint32
		Scheme      uintptr
	}
	highContrast.Size = uint32(unsafe.Sizeof(highContrast))
	user32.NewProc("SystemParametersInfoW").Call(0x0042, uintptr(highContrast.Size), uintptr(unsafe.Pointer(&highContrast)), 0)
	if highContrast.Flags&1 != 0 {
		color := func(index uintptr) uint32 { v, _, _ := user32.NewProc("GetSysColor").Call(index); return uint32(v) }
		d.background, d.foreground, d.border, d.primary, d.primaryText = color(5), color(8), color(8), color(13), color(14)
	}
	d.brush, _, _ = consentGDI.NewProc("CreateSolidBrush").Call(uintptr(d.background))
	heading, body, found := strings.Cut(d.message, "\n\n")
	if !found {
		heading, body = "Allow this action?", d.message
	}
	if heading == "Enable Full access for this desktop session?" {
		heading = "Enable Full access?"
		// Only the host's known session-consent format has collapsible technical
		// metadata. Never hide an exact-action summary or arbitrary task text.
		if summary, details, ok := strings.Cut(body, "\n\nAccess details\n"); ok {
			body, d.details = summary, details
		}
	}
	if len([]rune(heading)) > 80 {
		// Never clip an exact-action summary to make it fit a decorative title.
		heading, body = "Allow this action?", d.message
	}
	create := func(class, text string, style, id uintptr) uintptr {
		v, _, _ := user32.NewProc("CreateWindowExW").Call(0, uintptr(unsafe.Pointer(consentText(class))), uintptr(unsafe.Pointer(consentText(text))), style|0x50000000, 0, 0, 1, 1, d.window, id, 0, 0)
		return v
	}
	d.heading = create("STATIC", heading, 0x0080, 102) // SS_NOPREFIX
	// Native read-only multiline edit: selectable, screen-reader accessible,
	// word-wrapped, with scrolling rather than clipped permission details.
	body = strings.ReplaceAll(strings.ReplaceAll(body, "\r\n", "\n"), "\n", "\r\n")
	d.summary = body
	d.body = create("EDIT", body, 0x00210844, 0xffff) // VSCROLL, TABSTOP, READONLY, AUTOVSCROLL, MULTILINE
	if d.details != "" {
		d.disclosure = create("BUTTON", "Details", 0x1000b, 103)
		if d.disclosure == 0 {
			return false
		}
	}
	d.cancel = create("BUTTON", "Cancel", 0x1000b, 7) // TABSTOP | OWNERDRAW
	label := "Allow access"
	if strings.HasPrefix(d.message, "Enable Full access") {
		label = "Enable access"
	}
	if strings.HasPrefix(d.message, "Approve") {
		label = "Allow action"
	}
	d.allow = create("BUTTON", label, 0x1000b, 6)
	if d.brush == 0 || d.heading == 0 || d.body == 0 || d.cancel == 0 || d.allow == 0 {
		return false
	}
	for _, button := range []uintptr{d.cancel, d.allow, d.disclosure} {
		if button == 0 {
			continue
		}
		previous, _, _ := user32.NewProc("SetWindowLongPtrW").Call(button, ^uintptr(3), nativeConsentButtonProcedure)
		if previous == 0 {
			return false
		}
		nativeConsentButtonHandlers.Store(button, previous)
	}
	dpi, _, _ := user32.NewProc("GetDpiForWindow").Call(d.window)
	d.setFonts(int32(dpi))
	var bounds consentRect
	user32.NewProc("GetWindowRect").Call(d.window, uintptr(unsafe.Pointer(&bounds)))
	bounds.Right, bounds.Bottom = bounds.Left+d.px(480), bounds.Top+d.px(400)
	if d.details != "" {
		bounds.Right, bounds.Bottom = bounds.Left+d.px(440), bounds.Top+d.px(330)
	}
	d.fit(&bounds)
	return true
}

func (d *nativeConsent) px(value int32) int32 { return value * d.dpi / 96 }
func (d *nativeConsent) setFonts(dpi int32) {
	if dpi < 96 {
		dpi = 96
	}
	d.dpi = dpi
	old := d.fonts
	d.fonts = nil
	font := func(height int32, face string) uintptr {
		f, _, _ := consentGDI.NewProc("CreateFontW").Call(uintptr(-d.px(height)), 0, 0, 0, 400, 0, 0, 0, 1, 0, 0, 5, 0, uintptr(unsafe.Pointer(consentText(face))))
		d.fonts = append(d.fonts, f)
		return f
	}
	regular, title, button := font(14, "Segoe UI"), font(20, "Segoe UI Semibold"), font(14, "Segoe UI Semibold")
	for _, item := range [][2]uintptr{{d.heading, title}, {d.body, regular}, {d.cancel, button}, {d.allow, button}, {d.disclosure, button}} {
		if item[0] != 0 {
			user32.NewProc("SendMessageW").Call(item[0], 0x0030, item[1], 1)
		}
	}
	for _, f := range old {
		consentGDI.NewProc("DeleteObject").Call(f)
	}
}

func (d *nativeConsent) toggleDetails() {
	if d.details == "" {
		return
	}
	d.expanded = !d.expanded
	text, label, height := d.summary, "Details", int32(330)
	if d.expanded {
		text += "\r\n\r\n" + strings.ReplaceAll(d.details, "\n", "\r\n")
		label, height = "Hide details", 490
	}
	user32.NewProc("SetWindowTextW").Call(d.body, uintptr(unsafe.Pointer(consentText(text))))
	user32.NewProc("SetWindowTextW").Call(d.disclosure, uintptr(unsafe.Pointer(consentText(label))))
	var bounds consentRect
	user32.NewProc("GetWindowRect").Call(d.window, uintptr(unsafe.Pointer(&bounds)))
	bounds.Bottom = bounds.Top + d.px(height)
	d.fit(&bounds)
	// Expanding is presentation only; keep keyboard focus here, never on Allow.
	user32.NewProc("SetFocus").Call(d.disclosure)
}

func (d *nativeConsent) fit(suggested *consentRect) {
	monitor, _, _ := user32.NewProc("MonitorFromRect").Call(uintptr(unsafe.Pointer(suggested)), 2)
	var info struct {
		Size          uint32
		Monitor, Work consentRect
		Flags         uint32
	}
	info.Size = uint32(unsafe.Sizeof(info))
	if ok, _, _ := user32.NewProc("GetMonitorInfoW").Call(monitor, uintptr(unsafe.Pointer(&info))); ok != 0 {
		margin := d.px(16)
		width := min(suggested.Right-suggested.Left, info.Work.Right-info.Work.Left-2*margin)
		height := min(suggested.Bottom-suggested.Top, info.Work.Bottom-info.Work.Top-2*margin)
		x := max(info.Work.Left+margin, min(suggested.Left, info.Work.Right-margin-width))
		y := max(info.Work.Top+margin, min(suggested.Top, info.Work.Bottom-margin-height))
		user32.NewProc("SetWindowPos").Call(d.window, 0, uintptr(x), uintptr(y), uintptr(width), uintptr(height), 0x0004)
	}
	d.layout()
}

func (d *nativeConsent) layout() {
	if d.dpi == 0 {
		return
	}
	var r consentRect
	user32.NewProc("GetClientRect").Call(d.window, uintptr(unsafe.Pointer(&r)))
	gap, margin, footer := d.px(8), d.px(20), d.px(68)
	width := max(d.px(120), r.Right-2*margin)
	move := func(h uintptr, x, y, w, height int32) {
		user32.NewProc("MoveWindow").Call(h, uintptr(x), uintptr(y), uintptr(w), uintptr(max(1, height)), 1)
	}
	// Measure wrapping so a short title does not leave a large empty band,
	// while longer task/limited-window summaries remain readable.
	var headingRect = consentRect{Right: width}
	dc, _, _ := user32.NewProc("GetDC").Call(d.heading)
	font, _, _ := user32.NewProc("SendMessageW").Call(d.heading, 0x0031, 0, 0)
	old, _, _ := consentGDI.NewProc("SelectObject").Call(dc, font)
	var text [2048]uint16
	user32.NewProc("GetWindowTextW").Call(d.heading, uintptr(unsafe.Pointer(&text[0])), uintptr(len(text)))
	user32.NewProc("DrawTextW").Call(dc, uintptr(unsafe.Pointer(&text[0])), ^uintptr(0), uintptr(unsafe.Pointer(&headingRect)), 0x0c10) // CALCRECT, NOPREFIX, WORDBREAK
	consentGDI.NewProc("SelectObject").Call(dc, old)
	user32.NewProc("ReleaseDC").Call(d.heading, dc)
	headingHeight := max(d.px(28), min(d.px(56), headingRect.Bottom))
	move(d.heading, margin, d.px(18), width, headingHeight)
	bodyTop := d.px(18) + headingHeight + d.px(12)
	move(d.body, margin, bodyTop, width, r.Bottom-bodyTop-footer)
	if d.disclosure != 0 {
		move(d.disclosure, margin, r.Bottom-d.px(56), d.px(92), d.px(40))
	}
	buttonWidth := min(d.px(112), (width-gap)/2)
	move(d.allow, margin+width-buttonWidth, r.Bottom-d.px(56), buttonWidth, d.px(40))
	move(d.cancel, margin+width-buttonWidth-gap-d.px(80), r.Bottom-d.px(56), d.px(80), d.px(40))
	// EDIT always retains keyboard scrolling, but do not show an unnecessary
	// native scrollbar on a short confirmation whose entire summary fits.
	var bodyRect = consentRect{Right: width - d.px(22)}
	dc, _, _ = user32.NewProc("GetDC").Call(d.body)
	old, _, _ = consentGDI.NewProc("SelectObject").Call(dc, d.fonts[0])
	var bodyText [8192]uint16
	user32.NewProc("GetWindowTextW").Call(d.body, uintptr(unsafe.Pointer(&bodyText[0])), uintptr(len(bodyText)))
	user32.NewProc("DrawTextW").Call(dc, uintptr(unsafe.Pointer(&bodyText[0])), ^uintptr(0), uintptr(unsafe.Pointer(&bodyRect)), 0x0c10)
	consentGDI.NewProc("SelectObject").Call(dc, old)
	user32.NewProc("ReleaseDC").Call(d.body, dc)
	showScroll := uintptr(0)
	if bodyRect.Bottom > r.Bottom-bodyTop-footer-d.px(4) {
		showScroll = 1
	}
	user32.NewProc("ShowScrollBar").Call(d.body, 1, showScroll)
}

func (d *nativeConsent) drawButton(pointer uintptr) {
	var item = (*struct {
		Type, ID, ItemID, Action, State uint32
		Window, DC                      uintptr
		Rect                            consentRect
		Data                            uintptr
	})(unsafe.Pointer(pointer))
	if item.Type != 4 {
		return
	}
	bg, fg := d.background, d.foreground
	if item.ID == 6 {
		bg, fg = d.primary, d.primaryText
	}
	brush, _, _ := consentGDI.NewProc("CreateSolidBrush").Call(uintptr(bg))
	border, stroke := d.border, int32(1)
	focused := item.State&0x0010 != 0 && item.State&0x0200 == 0
	if focused && item.ID != 6 {
		border, stroke = d.foreground, d.px(2)
	}
	pen, _, _ := consentGDI.NewProc("CreatePen").Call(0, uintptr(stroke), uintptr(border))
	oldBrush, _, _ := consentGDI.NewProc("SelectObject").Call(item.DC, brush)
	oldPen, _, _ := consentGDI.NewProc("SelectObject").Call(item.DC, pen)
	r := item.Rect
	user32.NewProc("FillRect").Call(item.DC, uintptr(unsafe.Pointer(&r)), d.brush)
	consentGDI.NewProc("RoundRect").Call(item.DC, uintptr(r.Left), uintptr(r.Top), uintptr(r.Right-1), uintptr(r.Bottom-1), uintptr(d.px(12)), uintptr(d.px(12)))
	consentGDI.NewProc("SetBkMode").Call(item.DC, 1)
	consentGDI.NewProc("SetTextColor").Call(item.DC, uintptr(fg))
	// OWNERDRAW buttons do not reliably return their assigned font through
	// WM_GETFONT. Select our owned Segoe UI font, never the DC's stock serif font.
	oldFont, _, _ := consentGDI.NewProc("SelectObject").Call(item.DC, d.fonts[2])
	var text [128]uint16
	user32.NewProc("GetWindowTextW").Call(item.Window, uintptr(unsafe.Pointer(&text[0])), 128)
	user32.NewProc("DrawTextW").Call(item.DC, uintptr(unsafe.Pointer(&text[0])), ^uintptr(0), uintptr(unsafe.Pointer(&r)), 0x0025) // single line, centered
	if focused && item.ID == 6 {                                                                                                   // solid rounded focus, not a dotted inset
		r.Left += d.px(4)
		r.Top += d.px(4)
		r.Right -= d.px(4)
		r.Bottom -= d.px(4)
		focusPen, _, _ := consentGDI.NewProc("CreatePen").Call(0, uintptr(d.px(2)), uintptr(d.primaryText))
		consentGDI.NewProc("SelectObject").Call(item.DC, focusPen)
		hollow, _, _ := consentGDI.NewProc("GetStockObject").Call(5)
		consentGDI.NewProc("SelectObject").Call(item.DC, hollow)
		consentGDI.NewProc("RoundRect").Call(item.DC, uintptr(r.Left), uintptr(r.Top), uintptr(r.Right), uintptr(r.Bottom), uintptr(d.px(8)), uintptr(d.px(8)))
		consentGDI.NewProc("SelectObject").Call(item.DC, pen)
		consentGDI.NewProc("DeleteObject").Call(focusPen)
	}
	consentGDI.NewProc("SelectObject").Call(item.DC, oldFont)
	consentGDI.NewProc("SelectObject").Call(item.DC, oldPen)
	consentGDI.NewProc("SelectObject").Call(item.DC, oldBrush)
	consentGDI.NewProc("DeleteObject").Call(pen)
	consentGDI.NewProc("DeleteObject").Call(brush)
}

func (d *nativeConsent) dispose() {
	for _, font := range d.fonts {
		consentGDI.NewProc("DeleteObject").Call(font)
	}
	if d.brush != 0 {
		consentGDI.NewProc("DeleteObject").Call(d.brush)
	}
}
