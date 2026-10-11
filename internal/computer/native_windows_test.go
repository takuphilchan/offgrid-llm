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
	checkbox, _, _ := create.Call(0, ptr("BUTTON"), ptr("Include sources"), 0x50010003, 20, 160, 240, 35, root, 102, 0, 0)
	if edit == 0 || password == 0 || checkbox == 0 {
		t.Fatal("fixture controls unavailable")
	}
	user32.NewProc("ShowWindow").Call(root, 5)
	user32.NewProc("SetForegroundWindow").Call(root)
	fmt.Printf("FIXTURE %d %d %d\n", root, edit, checkbox)
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
		if msg.Message == 0x0100 && msg.WParam == 0x53 { // WM_KEYDOWN / S
			ctrl, _, _ := user32.NewProc("GetKeyState").Call(0x11)
			if int16(ctrl) < 0 {
				user32.NewProc("SetWindowTextW").Call(root, ptr("OffGrid shortcut saved"))
			}
		}
		if msg.Message == 0x8001 { // WM_APP+1: owned fixture focus request
			user32.NewProc("SetForegroundWindow").Call(root)
			continue
		}
		user32.NewProc("TranslateMessage").Call(uintptr(unsafe.Pointer(&msg)))
		user32.NewProc("DispatchMessageW").Call(uintptr(unsafe.Pointer(&msg)))
	}
}

func TestNativeWindowsLiveUIA(t *testing.T) {
	if os.Getenv("OFFGRID_TEST_NATIVE_WINDOWS") != "1" {
		t.Skip("set OFFGRID_TEST_NATIVE_WINDOWS=1 for isolated native UIA test")
	}
	if err := NativeWindowsTestEnvironment(); err != nil {
		t.Skipf("production policy refuses this elevated or isolated runner: %v", err)
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
	if len(fields) != 4 {
		t.Fatal("fixture missing")
	}
	root, _ := strconv.ParseUint(fields[1], 10, 64)
	edit, _ := strconv.ParseUint(fields[2], 10, 64)
	checkbox, _ := strconv.ParseUint(fields[3], 10, 64)
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
	// V3 readiness validates the actual owned process image and can reuse its
	// existing window. This fixture never launches or examines user documents.
	desktop, err := NewDesktopController(driver)
	if err != nil {
		t.Fatal(err)
	}
	fixtureInfo, err := os.Stat(executable)
	if err != nil {
		t.Fatal(err)
	}
	desktop.apps = map[string]installedApplication{"owned-fixture": {NativeApplication: NativeApplication{ID: "owned-fixture", Name: "Owned native fixture"}, path: executable, identity: fixtureInfo}}
	for attempt := 0; attempt < 2; attempt++ {
		evidence, err := desktop.VerifyApplication(context.Background(), "owned-fixture")
		if err != nil || !evidence.Verified || evidence.Target.ProcessGeneration != target.Identity.ProcessGeneration {
			t.Fatal("v3 owned process readback", evidence, err)
		}
	}
	// Rediscovery issues new target IDs. Reacquire the same owned surface for
	// the existing v2 mutation/independent Win32 oracle checks below.
	for _, surface := range driver.targets {
		if surface.hwnd == uintptr(root) {
			target = surface.target
		}
	}
	focusSelected := func() error {
		if err := driver.FocusSelected(); err == nil {
			return nil
		}
		// Windows may withdraw foreground permission from a long-running test.
		// Ask only the owned fixture process to focus itself, then re-run the
		// production target/focus validation. No production bypass is added.
		user32.NewProc("PostMessageW").Call(uintptr(root), 0x8001, 0, 0)
		for attempt := 0; attempt < 25; attempt++ {
			current, _, _ := foregroundWindow.Call()
			if current == uintptr(root) {
				return driver.FocusSelected()
			}
			time.Sleep(20 * time.Millisecond)
		}
		return ErrControlScope
	}
	if _, err = driver.Select(target.Identity.ID); err != nil {
		t.Fatal(err)
	}
	view, err := driver.Observe(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	var field NativeElement
	var check NativeElement
	for _, element := range view.Elements {
		if strings.Contains(element.Name, "SYNTHETIC_TEST_SECRET") {
			t.Fatal("password content exposed")
		}
		if element.Writable && field.ID == "" {
			field = element
		}
		if element.Checkable && element.Name == "Include sources" {
			check = element
		}
	}
	if field.ID == "" || check.ID == "" || check.Checked == nil || *check.Checked {
		t.Fatalf("native edit not discovered: %d controls", len(view.Elements))
	}
	initialEvidenceIdentity := field.Identity
	if initialEvidenceIdentity == "" {
		t.Fatal("owned UIA control lacks evidence identity")
	}
	step, _, consent, _ := controlFixture()
	step.Binding.Target = target.Identity
	consent.Binding = step.Binding
	consent.IssuedAt = time.Now().Add(-time.Second)
	consent.ExpiresAt = time.Now().Add(time.Minute)
	text := "Zimbabwe research — final (2026)"
	// Simulate slow inference without sleeping. V2 still rejects old evidence;
	// v3 must freshly inspect the owned fixture and retain only identical controls.
	view.Observation.CapturedAt = time.Now().Add(-3 * time.Minute)
	driver.view.Observation = view.Observation
	if _, err := driver.Prepare(step.Binding, Operation{Kind: "replace_text", Element: field.ID, Text: &text}, view.Observation); err == nil {
		t.Fatal("v2 accepted expired observation")
	}
	forged := view.Observation
	forged.CapturedAt = time.Now()
	if _, err := driver.RefreshObservation(context.Background(), step.Binding, forged); err != ErrStaleObservation {
		t.Fatal("forged freshness accepted", err)
	}
	refreshed, err := driver.RefreshObservation(context.Background(), step.Binding, view.Observation)
	if err != nil || refreshed.ID == view.Observation.ID || refreshed.Validate(step.Binding.Target, time.Now()) != nil {
		t.Fatal("delayed owned view not revalidated", err)
	}
	view = driver.view
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
	if err := focusSelected(); err != nil {
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
	oldInputID := field.ID
	view, err = driver.Observe(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	for _, element := range view.Elements {
		if element.Writable {
			field = element
			if field.ID == oldInputID || field.Identity != initialEvidenceIdentity {
				t.Fatal("fresh input ID did not preserve owned OS evidence identity")
			}
			break
		}
	}
	action, err = driver.Prepare(step.Binding, Operation{Kind: "shortcut", Element: field.ID, Shortcut: "save"}, view.Observation)
	if err != nil {
		t.Fatal(err)
	}
	step.Actions = []PreparedControlAction{action}
	grant, err = supervisor.Approve(step, func(proposed BoundedStep) bool { return proposed.Actions[0].Operation.Shortcut == "save" })
	if err != nil {
		t.Fatal(err)
	}
	if err = focusSelected(); err != nil {
		t.Fatal(err)
	}
	results, err = supervisor.Execute(step, grant.ID)
	if err != nil || len(results) != 1 || results[0].Outcome != "dispatched" {
		t.Fatal(results, err)
	}
	var title [128]uint16
	user32.NewProc("GetWindowTextW").Call(uintptr(root), uintptr(unsafe.Pointer(&title[0])), 128)
	if got := windows.UTF16ToString(title[:]); got != "OffGrid shortcut saved" {
		t.Fatalf("bound save shortcut was not delivered: %q", got)
	}
	if _, err := supervisor.Execute(step, grant.ID); err == nil {
		t.Fatal("approved mutation repeated")
	}
	view, err = driver.Observe(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	for _, element := range view.Elements {
		if element.Checkable && element.Name == "Include sources" {
			check = element
		}
	}
	wantChecked := true
	action, err = driver.Prepare(step.Binding, Operation{Kind: "set_checked", Element: check.ID, Checked: &wantChecked}, view.Observation)
	if err != nil {
		t.Fatal(err)
	}
	step.Actions = []PreparedControlAction{action}
	grant, err = supervisor.Approve(step, func(proposed BoundedStep) bool {
		return proposed.Actions[0].Operation.Checked != nil && *proposed.Actions[0].Operation.Checked
	})
	if err != nil {
		t.Fatal(err)
	}
	if err = focusSelected(); err != nil {
		t.Fatal(err)
	}
	results, err = supervisor.Execute(step, grant.ID)
	if err != nil || len(results) != 1 || results[0].Outcome != "verified" {
		t.Fatal(results, err)
	}
	state, _, _ := user32.NewProc("SendMessageW").Call(uintptr(checkbox), 0x00F0, 0, 0) // BM_GETCHECK
	if state != 1 {
		t.Fatal("independent Win32 checkbox oracle mismatch")
	}
	view, err = driver.Observe(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	for _, element := range view.Elements {
		if element.Checkable && element.Name == "Include sources" {
			check = element
		}
	}
	view.Observation.CapturedAt = time.Now().Add(-3 * time.Minute)
	driver.view.Observation = view.Observation
	refreshed, err = driver.RefreshObservation(context.Background(), step.Binding, view.Observation)
	if err != nil {
		t.Fatal("delayed checkbox readback", err)
	}
	verified, err := driver.VerifyChecked(step.Binding, check.ID, true, refreshed)
	if err != nil || !verified {
		t.Fatal("checkbox verification failed", err)
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
	if err := focusSelected(); err != nil {
		t.Fatal(err)
	}
	if _, err := driver.Dispatch(context.Background(), action); err != ErrStaleObservation {
		t.Fatalf("stale native element accepted: %v", err)
	}
	view.Observation.CapturedAt = time.Now().Add(-3 * time.Minute)
	driver.view.Observation = view.Observation
	if _, err := driver.RefreshObservation(context.Background(), step.Binding, view.Observation); err != ErrStaleObservation {
		t.Fatal("manual edit was silently remapped after slow inference", err)
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
	for _, input := range []string{"button", "shortcut", "close", "session-lock", "suspend"} {
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
			case "session-lock":
				user32.NewProc("PostMessageW").Call(surface.window, 0x02B1, 7, 0)
			case "suspend":
				user32.NewProc("PostMessageW").Call(surface.window, 0x0218, 4, 0)
			}
			select {
			case <-stopped:
			case <-time.After(time.Second):
				t.Fatal("local stop did not revoke")
			}
		})
	}
}

func TestNativeWindowsHiddenStopMonitor(t *testing.T) {
	if os.Getenv("OFFGRID_TEST_NATIVE_WINDOWS") != "1" {
		t.Skip("opt-in owned native monitor")
	}
	for _, event := range []struct {
		name    string
		message uint32
		value   uintptr
	}{
		{"shortcut", 0x0312, 1}, {"session-lock", 0x02B1, 7}, {"suspend", 0x0218, 4},
	} {
		t.Run(event.name, func(t *testing.T) {
			stopped := make(chan struct{}, 1)
			surface, err := StartNativeStopMonitor(func() {
				select {
				case stopped <- struct{}{}:
				default:
				}
			})
			if err != nil {
				t.Fatal(err)
			}
			defer surface.Close()
			visible, _, _ := user32.NewProc("IsWindowVisible").Call(surface.window)
			if visible != 0 {
				t.Fatal("desktop-owned monitor created another popup")
			}
			user32.NewProc("PostMessageW").Call(surface.window, uintptr(event.message), event.value, 0)
			select {
			case <-stopped:
			case <-time.After(time.Second):
				t.Fatal("hidden monitor did not revoke")
			}
		})
	}
}
