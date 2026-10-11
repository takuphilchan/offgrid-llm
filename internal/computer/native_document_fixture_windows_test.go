//go:build windows && amd64

package computer

import (
	"bufio"
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
	"unsafe"

	ole "github.com/go-ole/go-ole"
	"golang.org/x/sys/windows"
)

type documentFixtureReply struct {
	Kind      string  `json:"kind"`
	PID       uint32  `json:"pid"`
	Window    uintptr `json:"window"`
	Editor    uintptr `json:"editor"`
	Selected  int     `json:"selected"`
	Documents []struct {
		ID    int    `json:"id"`
		Text  string `json:"text"`
		Start uint32 `json:"start"`
		End   uint32 `json:"end"`
	} `json:"documents"`
}

// Validate the independently spawned child and both HWNDs before every fixture
// command. A matching title or basename cannot qualify an unrelated target.
func validateDocumentFixtureOwner(reply documentFixtureReply, pid uint32, generation, session, executable string) error {
	if pid == 0 || reply.PID != pid || reply.Window == 0 || reply.Editor == 0 || generation == "" || session == "" || executable == "" {
		return ErrControlScope
	}
	for _, hwnd := range []uintptr{reply.Window, reply.Editor} {
		var owner uint32
		windowPID.Call(hwnd, uintptr(unsafe.Pointer(&owner)))
		if owner != pid {
			return ErrControlScope
		}
	}
	child, _, _ := user32.NewProc("IsChild").Call(reply.Window, reply.Editor)
	if child == 0 {
		return ErrControlScope
	}
	actualGeneration, actualSession, err := processGeneration(pid)
	if err != nil || actualGeneration != generation || actualSession != session {
		return ErrControlScope
	}
	process, err := windows.OpenProcess(windows.PROCESS_QUERY_LIMITED_INFORMATION, false, pid)
	if err != nil {
		return ErrControlScope
	}
	defer windows.CloseHandle(process)
	name := make([]uint16, 32768)
	size := uint32(len(name))
	if windows.QueryFullProcessImageName(process, 0, &name[0], &size) != nil {
		return ErrControlScope
	}
	expected, err := os.Stat(executable)
	if err != nil {
		return ErrControlScope
	}
	actual, err := os.Stat(windows.UTF16ToString(name[:size]))
	if err != nil || !os.SameFile(expected, actual) {
		return ErrControlScope
	}
	return nil
}

func TestDocumentFixtureOwnershipRefusesUnknownWindows(t *testing.T) {
	for _, reply := range []documentFixtureReply{{}, {PID: 2, Window: 1, Editor: 2}, {PID: 1, Window: 0, Editor: 2}, {PID: 1, Window: 1}} {
		if err := validateDocumentFixtureOwner(reply, 1, "generation", "session", "not-a-fixture.exe"); err == nil {
			t.Fatal("unrelated fixture accepted")
		}
	}
}

func TestNativeWindowsDocumentFixture(t *testing.T) {
	if os.Getenv("OFFGRID_TEST_NATIVE_WINDOWS") != "1" || os.Getenv("OFFGRID_NATIVE_ISOLATED_DESKTOP") != "1" {
		t.Skip("document fixture requires both native opt-in and an explicitly isolated desktop")
	}
	account, err := user.Current()
	if err != nil {
		t.Fatal(err)
	}
	name := account.Username
	if i := strings.LastIndex(name, `\`); i >= 0 {
		name = name[i+1:]
	}
	if os.Getenv("OFFGRID_NATIVE_ISOLATED_DESKTOP") != "1" || !strings.HasPrefix(strings.ToLower(name), "offgridtest-") {
		t.Fatal("refusing document input outside a disposable OffGridTest-* account")
	}
	if err := NativeWindowsTestEnvironment(); err != nil {
		t.Fatal("native fixture unavailable:", err)
	}
	executable := filepath.Join(t.TempDir(), "owned-documents.exe")
	if output, err := exec.Command("go", "build", "-ldflags=-H=windowsgui", "-o", executable, "./testdata/nativeapp").CombinedOutput(); err != nil {
		t.Fatal(string(output), err)
	}
	child := exec.Command(executable)
	input, err := child.StdinPipe()
	if err != nil {
		t.Fatal(err)
	}
	output, err := child.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := child.Start(); err != nil {
		t.Fatal(err)
	}
	defer func() { input.Close(); child.Process.Kill(); child.Wait() }() // exact owned child only
	pid := uint32(child.Process.Pid)
	generation, session, err := processGeneration(pid)
	if err != nil {
		t.Fatal(err)
	}
	replies := make(chan documentFixtureReply, 16)
	go func() {
		defer close(replies)
		scanner := bufio.NewScanner(output)
		scanner.Buffer(make([]byte, 4096), 1024*1024)
		for scanner.Scan() {
			var reply documentFixtureReply
			if json.Unmarshal(scanner.Bytes(), &reply) != nil {
				return
			}
			replies <- reply
		}
	}()
	read := func() documentFixtureReply {
		t.Helper()
		select {
		case reply, ok := <-replies:
			if !ok {
				t.Fatal("owned fixture exited")
			}
			if err := validateDocumentFixtureOwner(reply, pid, generation, session, executable); err != nil {
				t.Fatal("fixture ownership lost:", err)
			}
			return reply
		case <-time.After(10 * time.Second):
			t.Fatal("owned fixture response timed out")
			return documentFixtureReply{}
		}
	}
	ready := read()
	send := func(command map[string]any) documentFixtureReply {
		t.Helper()
		if err := validateDocumentFixtureOwner(ready, pid, generation, session, executable); err != nil {
			t.Fatal("refusing fixture command:", err)
		}
		if err := json.NewEncoder(input).Encode(command); err != nil {
			t.Fatal(err)
		}
		return read()
	}
	if ready.Kind != "ready" || len(ready.Documents) != 2 || ready.Selected != 0 || ready.Documents[0].Text != "Synthetic lifecycle content" || ready.Documents[1].Text != "Untouched — café 日本語 12345" {
		t.Fatal("unexpected untouched-document baseline")
	}
	send(map[string]any{"kind": "delay", "delay_ms": 80})
	created := send(map[string]any{"kind": "new"})
	if len(created.Documents) != 2 {
		t.Fatal("delayed creation was not delayed")
	}
	deadline := time.Now().Add(3 * time.Second)
	for len(created.Documents) == 2 && time.Now().Before(deadline) {
		time.Sleep(20 * time.Millisecond)
		created = send(map[string]any{"kind": "state"})
	}
	if len(created.Documents) != 3 || created.Selected != 2 || created.Documents[2].ID == ready.Documents[0].ID || created.Documents[2].ID == ready.Documents[1].ID || created.Documents[2].Text != "" || created.Editor != ready.Editor {
		t.Fatal("creation did not yield distinct blank document in reused editor")
	}
	text := "OffGrid test — café 日本語 12345 😀 e\u0301 مرحبا"
	edited := send(map[string]any{"kind": "text", "text": text})
	if len(edited.Documents) != 3 || edited.Documents[2].Text != text {
		t.Fatal("fixture Unicode oracle mismatch")
	}
	for i := 0; i < 2; i++ {
		if edited.Documents[i] != ready.Documents[i] {
			t.Fatal("unrelated document changed")
		}
	}
	// Inspect the real TextPattern and select through UIA, not fixture commands.
	// The fixture pipe remains only the independent expected-state oracle.
	func() {
		runtime.LockOSThread()
		defer runtime.UnlockOSThread()
		driver, err := NewNativeWindows()
		if err != nil {
			t.Fatal(err)
		}
		defer driver.Close()
		if err := validateDocumentFixtureOwner(ready, pid, generation, session, executable); err != nil {
			t.Fatal(err)
		}
		var editor *ole.IUnknown
		if err := comCall(driver.automation, 6, ready.Editor, uintptr(unsafe.Pointer(&editor))); err != nil || editor == nil {
			t.Fatal("owned editor UIA unavailable", err)
		}
		defer editor.Release()
		actual, _, _, err := readUIADocument(editor)
		if err != nil || actual != text {
			t.Fatalf("independent TextPattern mismatch: %v", err)
		}
		pattern, document, err := uiaDocumentRange(editor)
		if err != nil {
			t.Fatal(err)
		}
		defer pattern.Release()
		defer document.Release()
		selection := DocumentTextRange{Start: 0, End: 7}
		rangeValue, err := uiaExactRange(context.Background(), document, text, selection)
		if err != nil {
			t.Fatal(err)
		}
		defer rangeValue.Release()
		if err := validateDocumentFixtureOwner(ready, pid, generation, session, executable); err != nil {
			t.Fatal(err)
		}
		if err := comCall(rangeValue, 16); err != nil {
			t.Fatal(err)
		}
		actual, selected, _, err := readUIADocument(editor)
		if err != nil || actual != text || selected == nil || *selected != selection {
			t.Fatal("UIA selection read-back failed", err)
		}
		oracle := send(map[string]any{"kind": "state"})
		if oracle.Documents[2].Text != text || oracle.Documents[2].Start != 0 || oracle.Documents[2].End != 7 {
			t.Fatal("UIA selection differs from independent oracle")
		}
		for i := 0; i < 2; i++ {
			if oracle.Documents[i] != ready.Documents[i] {
				t.Fatal("UIA selection changed unrelated document")
			}
		}
	}()
	selected := send(map[string]any{"kind": "select", "index": 0})
	if selected.Editor != ready.Editor || selected.Selected != 0 || len(selected.Documents) != 3 || selected.Documents[0].Text != ready.Documents[0].Text {
		t.Fatal("tab selection lost document binding")
	}
	selected = send(map[string]any{"kind": "selection", "start": 1, "end": 3})
	if len(selected.Documents) != 3 || selected.Documents[0].Start != 1 || selected.Documents[0].End != 3 {
		t.Fatal("selection oracle mismatch")
	}
	send(map[string]any{"kind": "partial_input", "index": 1})
	send(map[string]any{"kind": "focus_loss"})
	t.Logf("owned fixture pid=%d verified: duplicate tab titles, reused editor, Unicode, selection and delayed creation; native-driver edit qualification is separate", pid)
}
