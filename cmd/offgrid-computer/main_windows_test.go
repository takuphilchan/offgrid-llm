//go:build windows && amd64

package main

import (
	"bufio"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
	"unsafe"

	"github.com/takuphilchan/offgrid-llm/internal/computer"
	"golang.org/x/sys/windows"
)

var fixtureUser32 = windows.NewLazySystemDLL("user32.dll")

// These helpers exist only in the test executable. Production has no test
// consent flag, environment override, or automatic-approval callback.
func TestNativeWorkerHost(t *testing.T) {
	if os.Getenv("OFFGRID_NATIVE_WORKER_CHILD") != "1" {
		t.Skip("owned worker helper")
	}
	flag.CommandLine = flag.NewFlagSet("native-worker", flag.ContinueOnError)
	os.Args = []string{os.Args[0], "--state", os.Getenv("OFFGRID_NATIVE_TEST_STATE")}
	if err := run(); err != nil {
		t.Fatal(err)
	}
}

func TestNativeDocumentHost(t *testing.T) {
	if os.Getenv("OFFGRID_NATIVE_DOCUMENT_CHILD") != "1" {
		t.Skip("owned document helper")
	}
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	create := fixtureUser32.NewProc("CreateWindowExW")
	class, _ := windows.UTF16PtrFromString("STATIC")
	title, _ := windows.UTF16PtrFromString("OffGrid IPC owned document")
	hwnd, _, _ := create.Call(0, uintptr(unsafe.Pointer(class)), uintptr(unsafe.Pointer(title)), 0x10CF0000, 180, 200, 620, 220, 0, 0, 0, 0)
	runtime.KeepAlive(class)
	runtime.KeepAlive(title)
	editClass, _ := windows.UTF16PtrFromString("EDIT")
	initial, _ := windows.UTF16PtrFromString("Original document")
	edit, _, _ := create.Call(0, uintptr(unsafe.Pointer(editClass)), uintptr(unsafe.Pointer(initial)), 0x50810080, 20, 40, 540, 45, hwnd, 1, 0, 0)
	runtime.KeepAlive(editClass)
	runtime.KeepAlive(initial)
	if hwnd == 0 || edit == 0 {
		t.Fatal("owned document unavailable")
	}
	fixtureUser32.NewProc("ShowWindow").Call(hwnd, 5)
	fmt.Printf("DOCUMENT %d %d\n", hwnd, edit)
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
		value, _, _ := fixtureUser32.NewProc("GetMessageW").Call(uintptr(unsafe.Pointer(&msg)), 0, 0, 0)
		if int32(value) <= 0 {
			return
		}
		fixtureUser32.NewProc("TranslateMessage").Call(uintptr(unsafe.Pointer(&msg)))
		fixtureUser32.NewProc("DispatchMessageW").Call(uintptr(unsafe.Pointer(&msg)))
	}
}

func TestNativeWorkerPrivatePipeConsentAndRealEdit(t *testing.T) {
	if os.Getenv("OFFGRID_TEST_NATIVE_WINDOWS") != "1" {
		t.Skip("opt-in owned native application test")
	}
	if err := computer.NativeWindowsTestEnvironment(); err != nil {
		t.Skipf("production policy refuses this elevated or isolated runner: %v", err)
	}
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	start := func(name, env string) (*exec.Cmd, io.ReadCloser, io.WriteCloser) {
		child := exec.Command(executable, "-test.run=^"+name+"$")
		child.Env = append(os.Environ(), env, "OFFGRID_NATIVE_TEST_STATE="+t.TempDir())
		child.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
		out, e := child.StdoutPipe()
		if e != nil {
			t.Fatal(e)
		}
		in, e := child.StdinPipe()
		if e != nil {
			t.Fatal(e)
		}
		child.Stderr = os.Stderr
		if e = child.Start(); e != nil {
			t.Fatal(e)
		}
		t.Cleanup(func() { child.Process.Kill(); child.Wait() })
		return child, out, in
	}
	_, documentOut, _ := start("TestNativeDocumentHost", "OFFGRID_NATIVE_DOCUMENT_CHILD=1")
	scanner := bufio.NewScanner(documentOut)
	if !scanner.Scan() {
		t.Fatal("document fixture failed")
	}
	parts := strings.Fields(scanner.Text())
	if len(parts) != 3 || parts[0] != "DOCUMENT" {
		t.Fatal("document fixture identity missing")
	}
	hwnd, _ := strconv.ParseUint(parts[1], 10, 64)
	edit, _ := strconv.ParseUint(parts[2], 10, 64)
	child, out, in := start("TestNativeWorkerHost", "OFFGRID_NATIVE_WORKER_CHILD=1")
	var startup struct {
		Protocol int             `json:"protocol"`
		ID       string          `json:"id"`
		Result   map[string]bool `json:"result"`
		Error    string          `json:"error"`
	}
	if err := computer.ReadControlFrame(out, &startup); err != nil || startup.ID != "startup" || startup.Error != "" || !startup.Result["ready"] {
		t.Fatal("worker startup failed", startup.Error, err)
	}
	var count int
	call := func(kind string, fields map[string]any, into any) {
		t.Helper()
		count++
		id := fmt.Sprintf("request-%d", count)
		message := map[string]any{"protocol": 2, "id": id, "kind": kind}
		for k, v := range fields {
			message[k] = v
		}
		if e := computer.WriteControlFrame(in, message); e != nil {
			t.Fatal(e)
		}
		var response struct {
			Protocol int             `json:"protocol"`
			ID       string          `json:"id"`
			Result   json.RawMessage `json:"result"`
			Error    string          `json:"error"`
		}
		if e := computer.ReadControlFrame(out, &response); e != nil {
			t.Fatal(e)
		}
		if response.Protocol != 2 || response.ID != id || response.Error != "" {
			t.Fatalf("worker %s failed: %s", kind, response.Error)
		}
		if into != nil {
			if e := json.Unmarshal(response.Result, into); e != nil {
				t.Fatal(e)
			}
		}
	}
	var targets []computer.NativeTarget
	call("targets", nil, &targets)
	var target computer.NativeTarget
	for _, candidate := range targets {
		if candidate.Identity.Surface == fmt.Sprintf("window:%x", hwnd) {
			target = candidate
			break
		}
	}
	if target.Identity.ID == "" {
		t.Fatal("owned native document not selectable")
	}
	binding := computer.ControlBinding{Actor: "fixture-owner", Task: "fixture-task", Companion: "fixture-companion", Session: "fixture-session", Target: target.Identity}
	// Exercise actual OS Yes/No dialogs, never a production auto-approve flag.
	// The harness sends Yes only to the worker child it owns, for the exact
	// fixture document; no unrelated app or user's task is approved.
	approve := func(expected string) <-chan error {
		done := make(chan error, 1)
		go func() {
			deadline := time.Now().Add(10 * time.Second)
			var dialog uintptr
			callback := syscall.NewCallback(func(window, unused uintptr) uintptr {
				var pid uint32
				fixtureUser32.NewProc("GetWindowThreadProcessId").Call(window, uintptr(unsafe.Pointer(&pid)))
				if pid != uint32(child.Process.Pid) {
					return 1
				}
				var title [256]uint16
				fixtureUser32.NewProc("GetWindowTextW").Call(window, uintptr(unsafe.Pointer(&title[0])), 256)
				if windows.UTF16ToString(title[:]) != "OffGrid computer access" {
					return 1
				}
				label, _, _ := fixtureUser32.NewProc("GetDlgItem").Call(window, 0xffff)
				var text [8192]uint16
				fixtureUser32.NewProc("GetWindowTextW").Call(label, uintptr(unsafe.Pointer(&text[0])), 8192)
				content := windows.UTF16ToString(text[:])
				if strings.Contains(content, expected) && strings.Contains(content, "OffGrid IPC owned document") {
					dialog = window
					return 0
				}
				return 1
			})
			for time.Now().Before(deadline) {
				fixtureUser32.NewProc("EnumWindows").Call(callback, 0)
				if dialog != 0 {
					button, _, _ := fixtureUser32.NewProc("GetDlgItem").Call(dialog, 6)
					fixtureUser32.NewProc("SendMessageW").Call(button, 0x00F5, 0, 0)
					done <- nil
					return
				}
				time.Sleep(20 * time.Millisecond)
			}
			done <- fmt.Errorf("owned exact consent dialog not found")
			child.Process.Kill() // unblock the pipe; own helper only
		}()
		return done
	}
	consented := approve("Allow OffGrid to inspect")
	call("select", map[string]any{"target": target.Identity.ID, "binding": binding}, &target)
	if err := <-consented; err != nil {
		t.Fatal(err)
	}
	var view computer.NativeView
	call("observe", nil, &view)
	var element string
	for _, entry := range view.Elements {
		if entry.Writable {
			element = entry.ID
			break
		}
	}
	if element == "" {
		t.Fatal("native field unavailable")
	}
	text := "Zimbabwe research — final (2026)"
	var action computer.PreparedControlAction
	call("prepare", map[string]any{"operation": computer.Operation{Kind: "replace_text", Element: element, Text: &text}, "observation": view.Observation}, &action)
	step := computer.BoundedStep{ID: "exact-edit", Binding: binding, Actions: []computer.PreparedControlAction{action}}
	var grant computer.StepGrant
	consented = approve("Approve these exact changes")
	call("approve", map[string]any{"step": step}, &grant)
	if err := <-consented; err != nil {
		t.Fatal(err)
	}
	var results []computer.ControlResult
	call("execute", map[string]any{"step": step, "grant": grant.ID}, &results)
	if len(results) != 1 || results[0].Outcome != "verified" {
		t.Fatal("native outcome not verified")
	}
	var actual [1024]uint16
	fixtureUser32.NewProc("SendMessageW").Call(uintptr(edit), 0x000D, 1024, uintptr(unsafe.Pointer(&actual[0])))
	if windows.UTF16ToString(actual[:]) != text {
		t.Fatal("independent document oracle differs")
	}
	if err := computer.WriteControlFrame(in, map[string]any{"protocol": 2, "id": "stop", "kind": "stop"}); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	exited := make(chan error, 1)
	go func() { exited <- child.Wait() }()
	select {
	case err := <-exited:
		if err != nil {
			t.Fatal(err)
		}
	case <-ctx.Done():
		t.Fatal("owned worker failed to stop")
	}
	t.Log("private framed IPC, real local consent dialogs, approved Unicode edit, independent verification and worker exit passed")
}
