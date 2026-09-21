//go:build windows && amd64

package computer

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

// The opt-in live fixture is a separate, owned Win32 process. Its edit value
// is checked through WM_GETTEXT as an independent oracle, not the driver's own
// claim. It never selects or modifies a user's existing document/application.
func TestNativeFixtureHost(t *testing.T) {
	if os.Getenv("OFFGRID_NATIVE_FIXTURE_CHILD") != "1" {
		t.Skip("owned helper only")
	}
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	create := user32.NewProc("CreateWindowExW")
	ptr := func(s string) uintptr { v, _ := windows.UTF16PtrFromString(s); return uintptr(unsafe.Pointer(v)) }
	root, _, _ := create.Call(0, ptr("STATIC"), ptr("OffGrid owned native fixture"), 0x10CF0000, 180, 180, 620, 280, 0, 0, 0, 0)
	if root == 0 {
		t.Fatal("fixture window unavailable")
	}
	edit, _, _ := create.Call(0, ptr("EDIT"), ptr("Original fixture text"), 0x50810080, 20, 40, 540, 45, root, 100, 0, 0)
	password, _, _ := create.Call(0, ptr("EDIT"), ptr("SYNTHETIC_TEST_SECRET"), 0x508100A0, 20, 100, 540, 45, root, 101, 0, 0)
	if edit == 0 || password == 0 {
		t.Fatal("fixture controls unavailable")
	}
	user32.NewProc("ShowWindow").Call(root, 5)
	user32.NewProc("SetForegroundWindow").Call(root)
	fmt.Printf("FIXTURE %d %d\n", root, edit)
	type message struct {
		Window  uintptr
		Message uint32
		WParam  uintptr
		LParam  uintptr
		Time    uint32
		X, Y    int32
		Private uint32
	}
	var msg message
	for {
		value, _, _ := user32.NewProc("GetMessageW").Call(uintptr(unsafe.Pointer(&msg)), 0, 0, 0)
		if int32(value) <= 0 {
			return
		}
		user32.NewProc("TranslateMessage").Call(uintptr(unsafe.Pointer(&msg)))
		user32.NewProc("DispatchMessageW").Call(uintptr(unsafe.Pointer(&msg)))
	}
}

func TestNativeWindowsLiveUIA(t *testing.T) {
	if os.Getenv("OFFGRID_TEST_NATIVE_WINDOWS") != "1" {
		t.Skip("set OFFGRID_TEST_NATIVE_WINDOWS=1 for isolated native UIA test")
	}
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	child := exec.Command(executable, "-test.run=^TestNativeFixtureHost$", "-test.v")
	child.Env = append(os.Environ(), "OFFGRID_NATIVE_FIXTURE_CHILD=1")
	child.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	output, err := child.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	child.Stderr = os.Stderr
	if err := child.Start(); err != nil {
		t.Fatal(err)
	}
	defer func() { child.Process.Kill(); child.Wait() }()
	ready := make(chan string, 1)
	go func() {
		scanner := bufio.NewScanner(output)
		for scanner.Scan() {
			if strings.HasPrefix(scanner.Text(), "FIXTURE ") {
				ready <- scanner.Text()
				return
			}
		}
		ready <- ""
	}()
	var line string
	select {
	case line = <-ready:
	case <-time.After(10 * time.Second):
		t.Fatal("fixture startup timed out")
	}
	fields := strings.Fields(line)
	if len(fields) != 3 {
		t.Fatal("fixture missing")
	}
	root, _ := strconv.ParseUint(fields[1], 10, 64)
	edit, _ := strconv.ParseUint(fields[2], 10, 64)
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	driver, err := NewNativeWindows()
	if err != nil {
		t.Fatal(err)
	}
	defer driver.Close()
	targets, err := driver.Targets()
	if err != nil {
		t.Fatal(err)
	}
	var target NativeTarget
	for _, candidate := range targets {
		if candidate.Identity.Surface == fmt.Sprintf("window:%x", root) {
			target = candidate
			break
		}
	}
	if target.Identity.ID == "" {
		var pid uint32
		windowPID.Call(uintptr(root), uintptr(unsafe.Pointer(&pid)))
		visible, _, _ := isWindowVisible.Call(uintptr(root))
		generation, session, e := processGeneration(pid)
		t.Fatalf("owned native target not found: visible=%d pid=%d generation=%s session=%s policy=%v", visible, pid, generation, session, e)
	}
	if _, err = driver.Select(target.Identity.ID); err != nil {
		t.Fatal(err)
	}
	view, err := driver.Observe(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	var field NativeElement
	for _, element := range view.Elements {
		if strings.Contains(element.Name, "SYNTHETIC_TEST_SECRET") {
			t.Fatal("password content exposed")
		}
		if element.Writable && field.ID == "" {
			field = element
		}
	}
	if field.ID == "" {
		t.Fatalf("native edit not discovered: %d controls", len(view.Elements))
	}
	step, _, consent, _ := controlFixture()
	step.Binding.Target = target.Identity
	consent.Binding = step.Binding
	consent.IssuedAt = time.Now().Add(-time.Second)
	consent.ExpiresAt = time.Now().Add(time.Minute)
	text := "Zimbabwe research — final (2026)"
	action, err := driver.Prepare(step.Binding, Operation{Kind: "replace_text", Element: field.ID, Text: &text}, view.Observation)
	if err != nil {
		t.Fatal(err)
	}
	step.Actions = []PreparedControlAction{action}
	journal, err := OpenNativeJournal(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer journal.Close()
	supervisor := NewNativeSupervisor(journal, consent, driver.Dispatch)
	defer supervisor.Stop()
	grant, err := supervisor.Approve(step, func(proposed BoundedStep) bool {
		return proposed.Binding.Target == target.Identity && *proposed.Actions[0].Operation.Text == text
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := driver.FocusSelected(); err != nil {
		t.Fatal(err)
	}
	results, err := supervisor.Execute(step, grant.ID)
	if err != nil || len(results) != 1 || results[0].Outcome != "verified" {
		t.Fatal(results, err)
	}
	var buffer [512]uint16
	user32.NewProc("SendMessageW").Call(uintptr(edit), 0x000D, 512, uintptr(unsafe.Pointer(&buffer[0])))
	if got := windows.UTF16ToString(buffer[:]); got != text {
		t.Fatalf("independent Win32 oracle mismatch: %q", got)
	}
	if _, err := supervisor.Execute(step, grant.ID); err == nil {
		t.Fatal("approved mutation repeated")
	}
	// A manual/silent edit after observation must invalidate the prepared action.
	view, err = driver.Observe(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	for _, element := range view.Elements {
		if element.Writable {
			field = element
			break
		}
	}
	action, err = driver.Prepare(step.Binding, Operation{Kind: "replace_text", Element: field.ID, Text: &text}, view.Observation)
	if err != nil {
		t.Fatal(err)
	}
	replacement, _ := windows.UTF16PtrFromString("Manual fixture edit")
	user32.NewProc("SendMessageW").Call(uintptr(edit), 0x000C, 0, uintptr(unsafe.Pointer(replacement)))
	if err := driver.FocusSelected(); err != nil {
		t.Fatal(err)
	}
	if _, err := driver.Dispatch(context.Background(), action); err != ErrStaleObservation {
		t.Fatalf("stale native element accepted: %v", err)
	}
	supervisor.Stop()
	if err := supervisor.Check(); err != ErrControlScope {
		t.Fatal("scope not revoked")
	}
	t.Log("real UIA selection, password exclusion, approved Unicode edit, independent Win32 verification, replay and stale-action rejection passed")
}

func TestNativeWindowsLocalStopSurface(t *testing.T) {
	if os.Getenv("OFFGRID_TEST_NATIVE_WINDOWS") != "1" {
		t.Skip("opt-in owned native windows")
	}
	for _, input := range []string{"button", "shortcut", "close"} {
		t.Run(input, func(t *testing.T) {
			stopped := make(chan struct{}, 1)
			surface, err := StartNativeStopSurface(func() {
				select {
				case stopped <- struct{}{}:
				default:
				}
			})
			if err != nil {
				t.Fatal(err)
			}
			defer surface.Close()
			switch input {
			case "button":
				user32.NewProc("SendMessageW").Call(surface.button, 0x00F5, 0, 0) // BM_CLICK: real built-in button
			case "shortcut":
				user32.NewProc("PostMessageW").Call(surface.window, 0x0312, 1, 0)
			case "close":
				user32.NewProc("PostMessageW").Call(surface.window, 0x0010, 0, 0)
			}
			select {
			case <-stopped:
			case <-time.After(time.Second):
				t.Fatal("local stop did not revoke")
			}
		})
	}
}
