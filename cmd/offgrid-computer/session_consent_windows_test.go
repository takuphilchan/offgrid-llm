//go:build windows && amd64

package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"syscall"
	"testing"
	"time"
	"unsafe"

	"github.com/takuphilchan/offgrid-llm/internal/computer"
	"golang.org/x/sys/windows"
)

// Only approves a dialog belonging to this fixture child and exact disposable
// workspace. No real app is launched, focused, read, or mutated.
func TestNativeWorkerSessionConsentReuse(t *testing.T) {
	if os.Getenv("OFFGRID_TEST_NATIVE_WINDOWS") != "1" {
		t.Skip("opt-in owned consent fixture")
	}
	if err := computer.NativeWindowsTestEnvironment(); err != nil {
		t.Skipf("unsupported runner: %v", err)
	}
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	child := exec.CommandContext(ctx, exe, "-test.run=^TestNativeWorkerHost$")
	child.Env = append(os.Environ(), "OFFGRID_NATIVE_WORKER_CHILD=1", "OFFGRID_NATIVE_TEST_PROTOCOL=3", "OFFGRID_NATIVE_TEST_STATE="+t.TempDir())
	child.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	child.Stderr = os.Stderr
	out, err := child.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	in, err := child.StdinPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err = child.Start(); err != nil {
		t.Fatal(err)
	}
	defer func() { cancel(); child.Wait() }()
	var startup struct {
		Protocol int             `json:"protocol"`
		ID       string          `json:"id"`
		Error    string          `json:"error"`
		Result   map[string]bool `json:"result"`
	}
	if err = computer.ReadControlFrame(out, &startup); err != nil || startup.Error != "" || !startup.Result["ready"] {
		t.Fatal("startup", startup, err)
	}
	count := 0
	call := func(kind string, fields map[string]any, into any) {
		t.Helper()
		count++
		id := fmt.Sprint(count)
		req := map[string]any{"protocol": 3, "id": id, "kind": kind}
		for k, v := range fields {
			req[k] = v
		}
		if err := computer.WriteControlFrame(in, req); err != nil {
			t.Fatal(err)
		}
		var reply struct {
			Protocol int             `json:"protocol"`
			ID       string          `json:"id"`
			Error    string          `json:"error"`
			Result   json.RawMessage `json:"result"`
		}
		if err := computer.ReadControlFrame(out, &reply); err != nil || reply.Error != "" || reply.ID != id {
			t.Fatalf("%s: %s %v", kind, reply.Error, err)
		}
		if into != nil {
			if err := json.Unmarshal(reply.Result, into); err != nil {
				t.Fatal(err)
			}
		}
	}
	var dialog uintptr
	visible := 0
	inspect := syscall.NewCallback(func(hwnd, _ uintptr) uintptr {
		var pid uint32
		fixtureUser32.NewProc("GetWindowThreadProcessId").Call(hwnd, uintptr(unsafe.Pointer(&pid)))
		if pid != uint32(child.Process.Pid) {
			return 1
		}
		shown, _, _ := fixtureUser32.NewProc("IsWindowVisible").Call(hwnd)
		if shown == 0 {
			return 1
		}
		visible++
		var title [256]uint16
		fixtureUser32.NewProc("GetWindowTextW").Call(hwnd, uintptr(unsafe.Pointer(&title[0])), 256)
		if windows.UTF16ToString(title[:]) != "OffGrid computer access" {
			return 1
		}
		label, _, _ := fixtureUser32.NewProc("GetDlgItem").Call(hwnd, 0xffff)
		var body [8192]uint16
		fixtureUser32.NewProc("SendMessageW").Call(label, 0x000d, 8192, uintptr(unsafe.Pointer(&body[0])))
		text := windows.UTF16ToString(body[:])
		heading, _, _ := fixtureUser32.NewProc("GetDlgItem").Call(hwnd, 102)
		var headingText [256]uint16
		fixtureUser32.NewProc("GetWindowTextW").Call(heading, uintptr(unsafe.Pointer(&headingText[0])), 256)
		text = windows.UTF16ToString(headingText[:]) + "\n" + text
		if strings.Contains(text, "Enable Full access?") && !strings.Contains(text, "owned-session-fixture") {
			// Expand only the PID-owned fixture's technical details before
			// validating its exact disposable workspace and approving it.
			details, _, _ := fixtureUser32.NewProc("GetDlgItem").Call(hwnd, 103)
			fixtureUser32.NewProc("SendMessageW").Call(details, 0x00f5, 0, 0)
			return 1
		}
		if strings.Contains(text, "Enable Full access?") && strings.Contains(text, "owned-session-fixture") {
			dialog = hwnd
		}
		return 1
	})
	fixtureUser32.NewProc("EnumWindows").Call(inspect, 0)
	if visible != 0 {
		t.Fatal("extra Stop window before consent")
	}
	consent := make(chan error, 1)
	go func() {
		for i := 0; i < 250; i++ {
			fixtureUser32.NewProc("EnumWindows").Call(inspect, 0)
			if dialog != 0 {
				button, _, _ := fixtureUser32.NewProc("GetDlgItem").Call(dialog, 6)
				fixtureUser32.NewProc("SendMessageW").Call(button, 0x00F5, 0, 0)
				consent <- nil
				return
			}
			time.Sleep(20 * time.Millisecond)
		}
		consent <- fmt.Errorf("owned consent was not shown")
		cancel()
	}()
	call("enable_desktop", map[string]any{"permission": computer.DesktopPermission{Service: "http://127.0.0.1:11611", Workspace: "owned-session-fixture", Actor: "fixture-owner"}}, nil)
	if err := <-consent; err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 2; i++ {
		var target computer.TargetIdentity
		call("desktop_identity", nil, &target)
		authority := computer.DesktopAuthority{Workspace: "owned-session-fixture", Binding: computer.ControlBinding{Actor: "fixture-owner", Task: fmt.Sprintf("fixture-task-%d", i), Companion: "fixture-host", Session: fmt.Sprintf("fixture-session-%d", i), Target: target}}
		call("grant_desktop", map[string]any{"desktop": authority, "service": "http://127.0.0.1:11611", "prompt": "Owned authority-only fixture", "approval_mode": "full_task"}, nil)
		call("release_desktop", nil, nil)
		visible = 0
		fixtureUser32.NewProc("EnumWindows").Call(inspect, 0)
		if visible != 0 {
			t.Fatal("extra consent or Stop popup after task")
		}
	}
	if err := computer.WriteControlFrame(in, map[string]any{"protocol": 3, "id": "stop", "kind": "stop"}); err != nil {
		t.Fatal(err)
	}
	if err := child.Wait(); err != nil {
		t.Fatal(err)
	}
	t.Log("one real owned consent, two separate task grants, no Stop popup, private Stop acknowledged by worker exit; no application input")
}
