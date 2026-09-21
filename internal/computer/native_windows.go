//go:build windows && amd64

package computer

// Windows UI Automation is called on one MTA thread in the owned companion,
// never in the service. These are target-addressed provider operations: there
// is deliberately no SendInput, shell, arbitrary COM or screenshot-click API.
// Vtable slots follow Microsoft's generated UIAutomationClient.h.
import (
	"context"
	"fmt"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"
	"unsafe"

	ole "github.com/go-ole/go-ole"
	"golang.org/x/sys/windows"
)

var _ NativeAdapter = (*NativeWindows)(nil)

type nativeSurface struct {
	target     NativeTarget
	hwnd       uintptr
	pid        uint32
	generation string
}
type nativeControl struct {
	pointer     *ole.IUnknown
	info        NativeElement
	runtimeID   string
	fingerprint string
}
type NativeWindows struct {
	automation *ole.IUnknown
	targets    map[string]nativeSurface
	controls   map[string]nativeControl
	selected   *nativeSurface
	view       NativeView
	sequence   uint64
	prepared   map[string]PreparedControlAction
}

var user32 = windows.NewLazySystemDLL("user32.dll")
var enumWindows = user32.NewProc("EnumWindows")
var isWindowVisible = user32.NewProc("IsWindowVisible")
var windowPID = user32.NewProc("GetWindowThreadProcessId")
var windowText = user32.NewProc("GetWindowTextW")
var foregroundWindow = user32.NewProc("GetForegroundWindow")
var inputDesktop = user32.NewProc("OpenInputDesktop")
var closeDesktop = user32.NewProc("CloseDesktop")
var userObjectInfo = user32.NewProc("GetUserObjectInformationW")
var nativeEnumerations sync.Map
var nativeEnumerationID atomic.Uintptr
var nativeEnumerationCallback = syscall.NewCallback(func(hwnd, key uintptr) uintptr {
	if item, ok := nativeEnumerations.Load(key); ok {
		return item.(func(uintptr) uintptr)(hwnd)
	}
	return 0
})

//go:uintptrescapes
func comCall(object *ole.IUnknown, slot int, args ...uintptr) error {
	if object == nil {
		return ErrControlScope
	}
	table := unsafe.Slice((*uintptr)(unsafe.Pointer(object.RawVTable)), slot+1)
	values := append([]uintptr{uintptr(unsafe.Pointer(object))}, args...)
	result, _, _ := syscall.SyscallN(table[slot], values...)
	runtime.KeepAlive(object)
	if int32(result) < 0 {
		return fmt.Errorf("computer_provider_unavailable: HRESULT %08x", uint32(result))
	}
	return nil
}
func comInt(object *ole.IUnknown, slot int) (int32, error) {
	var v int32
	err := comCall(object, slot, uintptr(unsafe.Pointer(&v)))
	return v, err
}
func comString(object *ole.IUnknown, slot int) (string, error) {
	var value *int16
	if err := comCall(object, slot, uintptr(unsafe.Pointer(&value))); err != nil {
		return "", err
	}
	if value == nil {
		return "", nil
	}
	defer ole.SysFreeString(value)
	return ole.BstrToString((*uint16)(unsafe.Pointer(value))), nil
}

// Caller must stay on the creating OS thread until Close. Worker isolation
// bounds hung providers; context cancellation cannot interrupt an in-flight COM
// call and must never be reported as proof that its side effect did not occur.
func NewNativeWindows() (*NativeWindows, error) {
	if err := ole.CoInitializeEx(0, ole.COINIT_MULTITHREADED); err != nil {
		return nil, err
	}
	automation, err := ole.CreateInstance(ole.NewGUID("{FF48DBA4-60EF-4201-AA87-54103EEF594E}"), ole.NewGUID("{30CBE57D-D9D0-452A-AB13-7AC5AC4825EE}"))
	if err != nil {
		ole.CoUninitialize()
		return nil, err
	}
	return &NativeWindows{automation: automation, targets: map[string]nativeSurface{}, controls: map[string]nativeControl{}, prepared: map[string]PreparedControlAction{}}, nil
}
func (driver *NativeWindows) clearControls() {
	for _, el := range driver.controls {
		el.pointer.Release()
	}
	driver.controls = map[string]nativeControl{}
	driver.prepared = map[string]PreparedControlAction{}
}
func (driver *NativeWindows) Close() {
	driver.clearControls()
	if driver.automation != nil {
		driver.automation.Release()
		driver.automation = nil
		ole.CoUninitialize()
	}
}

func unlockedDesktop() bool {
	handle, _, _ := inputDesktop.Call(0, 0, 1)
	if handle == 0 {
		return false
	}
	defer closeDesktop.Call(handle)
	var name [256]uint16
	var needed uint32
	ok, _, _ := userObjectInfo.Call(handle, 2, uintptr(unsafe.Pointer(&name[0])), uintptr(len(name)*2), uintptr(unsafe.Pointer(&needed)))
	return ok != 0 && windows.UTF16ToString(name[:]) == "Default"
}
func processGeneration(pid uint32) (string, string, error) {
	var session, selfSession uint32
	if windows.ProcessIdToSessionId(pid, &session) != nil || windows.ProcessIdToSessionId(windows.GetCurrentProcessId(), &selfSession) != nil || session != selfSession {
		return "", "", ErrControlScope
	}
	handle, err := windows.OpenProcess(windows.PROCESS_QUERY_LIMITED_INFORMATION, false, pid)
	if err != nil {
		return "", "", ErrControlScope
	}
	defer windows.CloseHandle(handle)
	var token windows.Token
	if windows.OpenProcessToken(handle, windows.TOKEN_QUERY, &token) != nil {
		return "", "", ErrControlScope
	}
	defer token.Close()
	var elevated, returned uint32
	if windows.GetTokenInformation(token, windows.TokenElevation, (*byte)(unsafe.Pointer(&elevated)), 4, &returned) != nil || elevated != 0 {
		return "", "", ErrControlScope
	}
	owner, err := token.GetTokenUser()
	if err != nil {
		return "", "", ErrControlScope
	}
	self, err := windows.GetCurrentProcessToken().GetTokenUser()
	if err != nil || !owner.User.Sid.Equals(self.User.Sid) {
		return "", "", ErrControlScope
	}
	var creation, exit, kernel, user windows.Filetime
	if windows.GetProcessTimes(handle, &creation, &exit, &kernel, &user) != nil {
		return "", "", ErrControlScope
	}
	return fmt.Sprintf("%d:%d:%d", pid, creation.HighDateTime, creation.LowDateTime), fmt.Sprintf("%d:Default", session), nil
}
func (driver *NativeWindows) Targets() ([]NativeTarget, error) {
	if !unlockedDesktop() {
		return nil, ErrControlScope
	}
	result := []NativeTarget{}
	driver.targets = map[string]nativeSurface{}
	key := nativeEnumerationID.Add(1)
	nativeEnumerations.Store(key, func(hwnd uintptr) uintptr {
		if len(result) >= 100 {
			return 0
		}
		visible, _, _ := isWindowVisible.Call(hwnd)
		if visible == 0 {
			return 1
		}
		var pid uint32
		windowPID.Call(hwnd, uintptr(unsafe.Pointer(&pid)))
		if pid == windows.GetCurrentProcessId() {
			return 1
		}
		generation, session, err := processGeneration(pid)
		if err != nil {
			return 1
		}
		var title [513]uint16
		n, _, _ := windowText.Call(hwnd, uintptr(unsafe.Pointer(&title[0])), 513)
		if n == 0 {
			return 1
		}
		target := NativeTarget{Identity: TargetIdentity{ID: nativeID(), OSSession: session, ProcessGeneration: generation, Surface: fmt.Sprintf("window:%x", hwnd), Driver: "windows-uia"}, Title: windows.UTF16ToString(title[:])}
		driver.targets[target.Identity.ID] = nativeSurface{target: target, hwnd: hwnd, pid: pid, generation: generation}
		result = append(result, target)
		return 1
	})
	defer nativeEnumerations.Delete(key)
	enumWindows.Call(nativeEnumerationCallback, key)
	return result, nil
}
func (driver *NativeWindows) Select(id string) (NativeTarget, error) {
	surface, ok := driver.targets[id]
	if !ok {
		return NativeTarget{}, ErrControlScope
	}
	driver.clearControls()
	driver.selected = &surface
	if err := driver.checkTarget(false); err != nil {
		driver.selected = nil
		return NativeTarget{}, err
	}
	return surface.target, nil
}
func (driver *NativeWindows) checkTarget(requireFocus bool) error {
	if driver.selected == nil || !unlockedDesktop() {
		return ErrControlScope
	}
	surface := driver.selected
	var pid uint32
	windowPID.Call(surface.hwnd, uintptr(unsafe.Pointer(&pid)))
	generation, session, err := processGeneration(pid)
	visible, _, _ := isWindowVisible.Call(surface.hwnd)
	if err != nil || visible == 0 || generation != surface.generation || session != surface.target.Identity.OSSession {
		return ErrControlScope
	}
	if requireFocus {
		current, _, _ := foregroundWindow.Call()
		if current != surface.hwnd {
			return ErrControlScope
		}
	}
	return nil
}
func (driver *NativeWindows) withinTarget(element *ole.IUnknown) bool {
	var root, walker *ole.IUnknown
	if comCall(driver.automation, 6, driver.selected.hwnd, uintptr(unsafe.Pointer(&root))) != nil || root == nil {
		return false
	}
	defer root.Release()
	if comCall(driver.automation, 14, uintptr(unsafe.Pointer(&walker))) != nil || walker == nil {
		return false
	}
	defer walker.Release()
	element.AddRef()
	current := element
	for depth := 0; depth < 128 && current != nil; depth++ {
		var equal int32
		if comCall(driver.automation, 3, uintptr(unsafe.Pointer(root)), uintptr(unsafe.Pointer(current)), uintptr(unsafe.Pointer(&equal))) != nil {
			current.Release()
			return false
		}
		if equal != 0 {
			current.Release()
			return true
		}
		var parent *ole.IUnknown
		err := comCall(walker, 3, uintptr(unsafe.Pointer(current)), uintptr(unsafe.Pointer(&parent)))
		current.Release()
		current = parent
		if err != nil {
			if current != nil {
				current.Release()
			}
			return false
		}
	}
	if current != nil {
		current.Release()
	}
	return false
}
func pattern(el *ole.IUnknown, id uintptr) *ole.IUnknown {
	var result *ole.IUnknown
	if comCall(el, 16, id, uintptr(unsafe.Pointer(&result))) != nil {
		return nil
	}
	return result
}
func runtimeIdentity(el *ole.IUnknown) (string, error) {
	var raw *ole.SafeArray
	if err := comCall(el, 4, uintptr(unsafe.Pointer(&raw))); err != nil || raw == nil {
		return "", ErrControlScope
	}
	array := ole.SafeArrayConversion{Array: raw}
	defer array.Release()
	count, err := array.TotalElements(1)
	if err != nil || count < 1 || count > 64 {
		return "", ErrControlScope
	}
	return nativeHash(array.ToValueArray()), nil
}
func inspectControl(el *ole.IUnknown, pid uint32) (nativeControl, error) {
	process, err := comInt(el, 20)
	if err != nil || uint32(process) != pid {
		return nativeControl{}, ErrControlScope
	}
	password, err := comInt(el, 35)
	if err != nil || password != 0 {
		return nativeControl{}, ErrControlScope
	}
	enabled, err := comInt(el, 28)
	if err != nil || enabled == 0 {
		return nativeControl{}, ErrControlScope
	}
	offscreen, err := comInt(el, 38)
	if err != nil || offscreen != 0 {
		return nativeControl{}, ErrControlScope
	}
	name, err := comString(el, 23)
	if err != nil {
		return nativeControl{}, err
	}
	lower := strings.ToLower(name)
	for _, sensitive := range []string{"password", "passcode", "credential", "credit card", "verification code", "security code", "token"} {
		if strings.Contains(lower, sensitive) {
			return nativeControl{}, ErrControlScope
		}
	}
	if len(name) > 1024 {
		name = name[:1024]
	}
	role, _ := comString(el, 22)
	rid, err := runtimeIdentity(el)
	if err != nil {
		return nativeControl{}, err
	}
	info := NativeElement{Name: name, Role: role}
	value := ""
	if p := pattern(el, 10002); p != nil {
		readOnly, e := comInt(p, 5)
		if e == nil && readOnly == 0 {
			info.Writable = true
		}
		value, err = comString(p, 4)
		p.Release()
		if err != nil {
			return nativeControl{}, err
		}
	}
	if p := pattern(el, 10000); p != nil {
		info.Invokable = true
		p.Release()
	}
	// Values are hashed locally for freshness; they never leave the worker.
	return nativeControl{pointer: el, info: info, runtimeID: rid, fingerprint: nativeHash([]any{rid, name, role, info.Writable, info.Invokable, value})}, nil
}
func (driver *NativeWindows) Observe(ctx context.Context) (NativeView, error) {
	if err := ctx.Err(); err != nil {
		return NativeView{}, err
	}
	if err := driver.checkTarget(false); err != nil {
		return NativeView{}, err
	}
	driver.clearControls()
	var root, condition, array *ole.IUnknown
	if err := comCall(driver.automation, 6, driver.selected.hwnd, uintptr(unsafe.Pointer(&root))); err != nil {
		return NativeView{}, err
	}
	defer root.Release()
	if err := comCall(driver.automation, 21, uintptr(unsafe.Pointer(&condition))); err != nil {
		return NativeView{}, err
	}
	defer condition.Release()
	if err := comCall(root, 6, 4, uintptr(unsafe.Pointer(condition)), uintptr(unsafe.Pointer(&array))); err != nil {
		return NativeView{}, err
	}
	defer array.Release()
	count, err := comInt(array, 3)
	if err != nil {
		return NativeView{}, err
	}
	view := NativeView{Elements: []NativeElement{}, Limited: count > 400}
	fingerprints := []string{}
	for i := int32(0); i < count && i < 400; i++ {
		if err := ctx.Err(); err != nil {
			return NativeView{}, err
		}
		var el *ole.IUnknown
		if comCall(array, 4, uintptr(i), uintptr(unsafe.Pointer(&el))) != nil || el == nil {
			continue
		}
		control, err := inspectControl(el, driver.selected.pid)
		if err != nil {
			el.Release()
			continue
		}
		control.info.ID = nativeID()
		driver.controls[control.info.ID] = control
		view.Elements = append(view.Elements, control.info)
		fingerprints = append(fingerprints, control.fingerprint)
	}
	driver.sequence++
	view.Observation = ControlObservation{ID: nativeID(), Target: driver.selected.target.Identity, Sequence: driver.sequence, CapturedAt: time.Now().UTC(), StateDigest: nativeHash(fingerprints)}
	driver.view = view
	return view, nil
}
func (driver *NativeWindows) Prepare(binding ControlBinding, op Operation, observation ControlObservation) (PreparedControlAction, error) {
	if err := driver.checkTarget(false); err != nil {
		return PreparedControlAction{}, err
	}
	if !binding.valid() || binding.Target != driver.selected.target.Identity || observation.ID != driver.view.Observation.ID || observation.Validate(binding.Target, time.Now()) != nil || op.Validate() != nil {
		return PreparedControlAction{}, ErrControlScope
	}
	if op.Kind != "replace_text" && op.Kind != "activate" {
		return PreparedControlAction{}, ErrInvalidControl
	}
	control, ok := driver.controls[op.Element]
	if !ok {
		return PreparedControlAction{}, ErrControlScope
	}
	if !driver.withinTarget(control.pointer) {
		return PreparedControlAction{}, ErrControlScope
	}
	fresh, err := inspectControl(control.pointer, driver.selected.pid)
	if err != nil || fresh.runtimeID != control.runtimeID || fresh.fingerprint != control.fingerprint {
		return PreparedControlAction{}, ErrStaleObservation
	}
	if op.Kind == "replace_text" && !fresh.info.Writable || op.Kind == "activate" && !fresh.info.Invokable {
		return PreparedControlAction{}, ErrInvalidControl
	}
	action := PreparedControlAction{ID: nativeID(), Operation: op, ControlIdentity: control.runtimeID, Precondition: control.fingerprint, ExpectedChange: nativeHash(op)}
	driver.prepared[action.ID] = action
	return action, nil
}

// ApprovalSummary is generated from local prepared controls, not a model's
// description of what its action supposedly does. Quoting preserves exact
// whitespace and makes hidden control characters visible in the local prompt.
func (driver *NativeWindows) ApprovalSummary(step BoundedStep) (string, error) {
	if err := driver.checkTarget(false); err != nil {
		return "", err
	}
	if _, err := step.Digest(); err != nil {
		return "", err
	}
	if step.Binding.Target != driver.selected.target.Identity {
		return "", ErrControlScope
	}
	var text strings.Builder
	text.WriteString("Approve these exact changes in " + strconv.Quote(driver.selected.target.Title) + "?\nOffGrid will focus this application.\n\n")
	for index, action := range step.Actions {
		prepared, ok := driver.prepared[action.ID]
		if !ok || nativeHash(prepared) != nativeHash(action) {
			return "", ErrControlApproval
		}
		control, ok := driver.controls[action.Operation.Element]
		if !ok {
			return "", ErrControlScope
		}
		label := control.info.Name
		if label == "" {
			label = "unnamed " + control.info.Role
		}
		text.WriteString(fmt.Sprintf("%d. ", index+1))
		switch action.Operation.Kind {
		case "replace_text":
			text.WriteString("Replace all text in " + strconv.Quote(label) + " with:\n" + strconv.Quote(*action.Operation.Text) + "\n\n")
		case "activate":
			text.WriteString("Activate " + strconv.Quote(label) + ". This may submit or change information; successful activation alone does not verify the outcome.\n\n")
		default:
			return "", ErrInvalidControl
		}
	}
	text.WriteString("Only these changes are allowed. No additional actions are included.\nUse the local Stop window or Ctrl+Alt+Shift+F12 to revoke access.")
	return text.String(), nil
}

// Dispatch is called ONLY after local supervision durably claims the action.
// It consumes preparation before the provider call, even if COM fails.
func (driver *NativeWindows) Dispatch(ctx context.Context, action PreparedControlAction) (ControlResult, error) {
	result := ControlResult{ActionID: action.ID, Outcome: "failed"}
	if err := ctx.Err(); err != nil {
		return result, err
	}
	if err := driver.checkTarget(true); err != nil {
		return result, err
	}
	prepared, ok := driver.prepared[action.ID]
	if !ok || nativeHash(prepared) != nativeHash(action) {
		return result, ErrControlApproval
	}
	delete(driver.prepared, action.ID)
	control, ok := driver.controls[action.Operation.Element]
	if !ok {
		return result, ErrControlScope
	}
	if !driver.withinTarget(control.pointer) {
		return result, ErrControlScope
	}
	fresh, err := inspectControl(control.pointer, driver.selected.pid)
	if err != nil || fresh.fingerprint != action.Precondition || fresh.runtimeID != action.ControlIdentity {
		return result, ErrStaleObservation
	}
	driver.view = NativeView{}
	if action.Operation.Kind == "replace_text" {
		p := pattern(control.pointer, 10002)
		if p == nil {
			return result, ErrControlScope
		}
		defer p.Release()
		text, err := windows.UTF16PtrFromString(*action.Operation.Text)
		if err != nil {
			return result, ErrInvalidControl
		}
		result.Outcome = "uncertain"
		result.Uncertain = true
		if err := comCall(p, 3, uintptr(unsafe.Pointer(text))); err != nil {
			return result, err
		}
		runtime.KeepAlive(text)
		current, err := comString(p, 4)
		if err != nil || current != *action.Operation.Text {
			return result, ErrUncertain
		}
		result.Outcome = "verified"
		result.Uncertain = false
		result.EvidenceID = nativeHash(current)
		return result, nil
	}
	p := pattern(control.pointer, 10000)
	if p == nil {
		return result, ErrControlScope
	}
	defer p.Release()
	result.Outcome = "uncertain"
	result.Uncertain = true
	if err := comCall(p, 3); err != nil {
		return result, err
	}
	// Invoke success proves dispatch, not the user's requested outcome.
	result.Outcome = "dispatched"
	result.Uncertain = false
	return result, nil
}

// The prompt is local OS UI and never accepts a service/model-supplied boolean
// as consent. UIA work remains on its MTA thread; dialogs use a separate thread.
func NativeConfirm(message string) bool {
	result := make(chan bool, 1)
	go func() {
		runtime.LockOSThread()
		defer runtime.UnlockOSThread()
		text, err := windows.UTF16PtrFromString(message)
		if err != nil {
			result <- false
			return
		}
		title, _ := windows.UTF16PtrFromString("OffGrid computer access")
		value, _, _ := user32.NewProc("MessageBoxW").Call(0, uintptr(unsafe.Pointer(text)), uintptr(unsafe.Pointer(title)), 0x00000004|0x00000020|0x00000100|0x00010000)
		result <- value == 6
	}()
	return <-result
}
func (driver *NativeWindows) FocusSelected() error {
	if err := driver.checkTarget(false); err != nil {
		return err
	}
	user32.NewProc("SetForegroundWindow").Call(driver.selected.hwnd)
	// Cross-input-queue activation is asynchronous. Observe its completion;
	// never repeatedly request focus or bypass Windows foreground restrictions.
	for attempt := 0; attempt < 10; attempt++ {
		if err := driver.checkTarget(false); err != nil {
			return err
		}
		if driver.checkTarget(true) == nil {
			return nil
		}
		time.Sleep(20 * time.Millisecond)
	}
	return ErrControlScope
}
func AcquireNativeInputOwner() (func(), error) {
	token, err := windows.GetCurrentProcessToken().GetTokenUser()
	if err != nil {
		return nil, err
	}
	name, _ := windows.UTF16PtrFromString("Local\\OffGridComputerInput-" + nativeHash(token.User.Sid.String()))
	handle, err := windows.CreateMutex(nil, false, name)
	if err != nil {
		if handle != 0 {
			windows.CloseHandle(handle)
		}
		return nil, fmt.Errorf("computer_input_in_use")
	}
	return func() { windows.CloseHandle(handle) }, nil
}
