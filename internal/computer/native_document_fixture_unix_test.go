//go:build (darwin && cgo) || (linux && cgo && offgrid_native)

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
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
)

func runUnixDocumentFixture(t *testing.T, create func() (NativeAdapter, error)) {
	t.Helper()
	fixture := os.Getenv("OFFGRID_NATIVE_DOCUMENT_FIXTURE")
	if fixture == "" {
		t.Skip("owned document fixture not enabled")
	}
	if os.Getenv("OFFGRID_NATIVE_ISOLATED_DESKTOP") != "1" || os.Getuid() == 0 {
		t.Fatal("refusing native fixture outside explicit unprivileged isolation")
	}
	account, err := user.Current()
	if err != nil {
		t.Fatal(err)
	}
	if runtime.GOOS == "darwin" && !strings.HasPrefix(strings.ToLower(account.Username), "offgridtest-") {
		t.Fatal("AX fixture requires a disposable OffGridTest-* account")
	}
	if runtime.GOOS == "linux" && (os.Getenv("DISPLAY") == "" || os.Getenv("DBUS_SESSION_BUS_ADDRESS") == "" || os.Getenv("XDG_SESSION_TYPE") != "x11") {
		t.Fatal("GTK fixture requires the isolated X11 runner; it does not qualify Wayland")
	}
	directory := os.Getenv("OFFGRID_NATIVE_TEST_DIRECTORY")
	root, err := filepath.EvalSymlinks(directory)
	if err != nil || !filepath.IsAbs(root) || !strings.HasPrefix(filepath.Base(root), "offgrid-native-") {
		t.Fatal("missing isolated runner directory")
	}
	info, err := os.Stat(root)
	if err != nil || !info.IsDir() || info.Mode().Perm() != 0700 || info.Sys().(*syscall.Stat_t).Uid != uint32(os.Getuid()) {
		t.Fatal("unowned fixture directory")
	}
	executable, err := filepath.EvalSymlinks(fixture)
	if err != nil || filepath.Dir(executable) != root || filepath.Base(executable) != "owned-documents" {
		t.Fatal("fixture must be the runner-owned executable")
	}
	info, err = os.Stat(executable)
	if err != nil || !info.Mode().IsRegular() || info.Mode().Perm()&0022 != 0 || info.Sys().(*syscall.Stat_t).Uid != uint32(os.Getuid()) {
		t.Fatal("unsafe fixture executable")
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
	child.Stderr = os.Stderr
	if err := child.Start(); err != nil {
		t.Fatal(err)
	}
	defer func() { input.Close(); child.Process.Kill(); child.Wait() }()
	// Do not reap this child until cleanup. PID+provider generation/OS session
	// bind every check; commands go only to its private pipe, never a global port.
	replies := make(chan unixDocumentFixtureReply, 1)
	done := make(chan struct{})
	defer close(done)
	go func() {
		defer close(replies)
		scanner := bufio.NewScanner(output)
		scanner.Buffer(make([]byte, 4096), 1<<20)
		for scanner.Scan() {
			var reply unixDocumentFixtureReply
			if json.Unmarshal(scanner.Bytes(), &reply) != nil {
				return
			}
			select {
			case replies <- reply:
			case <-done:
				return
			}
		}
	}()
	read := func() unixDocumentFixtureReply {
		t.Helper()
		select {
		case reply, ok := <-replies:
			if !ok {
				t.Fatal("owned fixture exited")
			}
			return reply
		case <-time.After(10 * time.Second):
			t.Fatal("owned fixture timed out")
			return unixDocumentFixtureReply{}
		}
	}
	ready := read()
	driver, err := create()
	if err != nil {
		t.Fatal("native provider/permission unavailable:", err)
	}
	defer driver.Close()
	var target NativeTarget
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		targets, err := driver.Targets()
		if err != nil {
			t.Fatal("native discovery unavailable:", err)
		}
		for _, candidate := range targets {
			if strings.HasPrefix(candidate.Identity.ProcessGeneration, strconv.Itoa(child.Process.Pid)+":") {
				if target.Identity.ID != "" {
					t.Fatal("ambiguous owned fixture surface")
				}
				target = candidate
			}
		}
		if target.Identity.ID != "" {
			break
		}
		time.Sleep(30 * time.Millisecond)
	}
	if target.Identity.ID == "" {
		t.Fatal("owned fixture not exposed by native provider")
	}
	if validateUnixDocumentFixture(ready, child.Process.Pid, target.Identity) != nil {
		t.Fatal("unowned initial fixture state")
	}
	selected, err := driver.Select(target.Identity.ID)
	if err != nil || selected.Identity != target.Identity {
		t.Fatal("owned native selection failed:", err)
	}
	check := func() {
		t.Helper()
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		view, err := driver.Observe(ctx)
		if err != nil || view.Observation.Target != target.Identity || view.Observation.Validate(target.Identity, time.Now()) != nil {
			t.Fatal("owned native scope lost:", err)
		}
	}
	send := func(request map[string]any) unixDocumentFixtureReply {
		t.Helper()
		check()
		if err := json.NewEncoder(input).Encode(request); err != nil {
			t.Fatal(err)
		}
		reply := read()
		if validateUnixDocumentFixture(reply, child.Process.Pid, target.Identity) != nil || reply.Kind != request["kind"] {
			t.Fatal("invalid owned fixture reply")
		}
		check()
		return reply
	}
	if ready.Kind != "ready" || len(ready.Documents) != 2 || ready.Selected != 0 || ready.Documents[0].Text != "Synthetic lifecycle content" || ready.Documents[1].Text != "Untouched — café 日本語 12345" {
		t.Fatal("unexpected baseline")
	}
	send(map[string]any{"kind": "delay", "delay_ms": 200})
	created := send(map[string]any{"kind": "new"})
	if len(created.Documents) != 2 {
		t.Fatal("creation was not delayed")
	}
	deadline = time.Now().Add(3 * time.Second)
	for len(created.Documents) == 2 && time.Now().Before(deadline) {
		time.Sleep(30 * time.Millisecond)
		created = send(map[string]any{"kind": "state"})
	}
	if len(created.Documents) != 3 || created.Selected != 2 || created.Documents[2].ID == ready.Documents[0].ID || created.Documents[2].ID == ready.Documents[1].ID || created.Documents[2].Text != "" || created.Editor != ready.Editor {
		t.Fatal("distinct blank tab in reused editor missing")
	}
	text := "OffGrid test — café 日本語 12345 😀 e\u0301 مرحبا"
	edited := send(map[string]any{"kind": "text", "text": text})
	if edited.Documents[2].Text != text {
		t.Fatal("Unicode fixture oracle mismatch")
	}
	for i := 0; i < 2; i++ {
		if edited.Documents[i] != ready.Documents[i] {
			t.Fatal("unrelated document changed")
		}
	}
	selectedReply := send(map[string]any{"kind": "select", "index": 0})
	if selectedReply.Selected != 0 || selectedReply.Documents[0] != ready.Documents[0] {
		t.Fatal("selection did not preserve baseline")
	}
	selection := send(map[string]any{"kind": "selection", "start": 1, "end": 3})
	if selection.Documents[0].Start != 1 || selection.Documents[0].End != 3 {
		t.Fatal("selection mismatch")
	}
	send(map[string]any{"kind": "select", "index": 2})
	send(map[string]any{"kind": "selection", "start": 0, "end": 0})
	send(map[string]any{"kind": "partial_input", "index": 1})
	partial := send(map[string]any{"kind": "text", "text": "XYZ"})
	if partial.Documents[2].Text != "X"+text {
		t.Fatal("partial-input fault did not preserve prefix/suffix")
	}
	send(map[string]any{"kind": "focus_loss"})
	t.Log("Owned private fixture lifecycle and native read scope passed; not a native edit or installed-application qualification")
}
