package inference

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"testing"
	"time"

	"golang.org/x/sys/windows"
)

func TestProviderProcessTreeHelper(t *testing.T) {
	role := os.Getenv("OFFGRID_PROVIDER_TREE_TEST")
	if role == "" {
		return
	}
	if role == "owner" {
		cmd := providerTreeCommand("parent")
		cmd.Stdout = os.Stdout
		if _, err := startProviderProcess(cmd); err != nil {
			t.Fatal(err)
		}
	} else {
		if role == "parent" || role == "child" {
			next := "child"
			if role == "child" {
				next = "grandchild"
			}
			cmd := providerTreeCommand(next)
			cmd.Stdout = os.Stdout
			if err := cmd.Start(); err != nil {
				t.Fatal(err)
			}
		}
		fmt.Printf("%s:%d\n", role, os.Getpid())
	}
	for {
		time.Sleep(time.Second)
	}
}

func providerTreeCommand(role string) *exec.Cmd {
	cmd := exec.Command(os.Args[0], "-test.run=^TestProviderProcessTreeHelper$")
	cmd.Env = append(os.Environ(), "OFFGRID_PROVIDER_TREE_TEST="+role)
	configureBackgroundProcess(cmd)
	return cmd
}

func readProviderTreeHandles(t *testing.T, scanner *bufio.Scanner, count int) map[string]windows.Handle {
	t.Helper()
	lines := make(chan string, count)
	go func() {
		defer close(lines)
		for range count {
			if !scanner.Scan() {
				return
			}
			lines <- scanner.Text()
		}
	}()
	handles := map[string]windows.Handle{}
	for range count {
		select {
		case line := <-lines:
			role, raw, ok := strings.Cut(line, ":")
			pid, err := strconv.Atoi(raw)
			if !ok || err != nil || pid <= 0 {
				t.Fatalf("invalid provider fixture identity: %q", line)
			}
			handle, err := windows.OpenProcess(windows.SYNCHRONIZE|windows.PROCESS_TERMINATE, false, uint32(pid))
			if err != nil {
				t.Fatal(err)
			}
			handles[role] = handle
			t.Cleanup(func() { _ = windows.TerminateProcess(handle, 1); _ = windows.CloseHandle(handle) })
		case <-time.After(10 * time.Second):
			t.Fatal("owned provider descendant startup timed out")
		}
	}
	return handles
}

func TestProviderProcessTreeRetiresDescendantsAfterParentExitAndLeavesOtherRuntime(t *testing.T) {
	first := providerTreeCommand("parent")
	stdout, err := first.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	stopFirst, err := startProviderProcess(first)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = stopFirst(first) })
	children := readProviderTreeHandles(t, bufio.NewScanner(stdout), 3)
	second := providerTreeCommand("independent")
	otherOutput, err := second.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	stopSecond, err := startProviderProcess(second)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = stopSecond(second) })
	other := readProviderTreeHandles(t, bufio.NewScanner(otherOutput), 1)["independent"]
	if err := first.Process.Kill(); err != nil {
		t.Fatal(err)
	}
	if status, err := windows.WaitForSingleObject(children["parent"], 3000); err != nil || status != windows.WAIT_OBJECT_0 {
		t.Fatal("fixture parent did not exit")
	}
	for _, name := range []string{"child", "grandchild"} {
		if status, _ := windows.WaitForSingleObject(children[name], 0); status != uint32(windows.WAIT_TIMEOUT) {
			t.Fatal("fixture descendant did not survive parent", name)
		}
	}
	closed := make(chan error, 2)
	for range 2 {
		go func() { closed <- stopFirst(first) }()
	}
	for range 2 {
		if err := <-closed; err != nil {
			t.Fatal(err)
		}
	}
	for role, handle := range children {
		if status, err := windows.WaitForSingleObject(handle, 0); err != nil || status != windows.WAIT_OBJECT_0 {
			t.Fatal("released provider retained descendant", role, status, err)
		}
	}
	if status, err := windows.WaitForSingleObject(other, 0); err != nil || status != uint32(windows.WAIT_TIMEOUT) {
		t.Fatal("retiring one provider killed another", status, err)
	}
}

func TestProviderProcessTreeDiesWithServiceOwner(t *testing.T) {
	owner := providerTreeCommand("owner")
	stdout, err := owner.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err = owner.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = owner.Process.Kill(); _ = owner.Wait() })
	children := readProviderTreeHandles(t, bufio.NewScanner(stdout), 3)
	if err := owner.Process.Kill(); err != nil {
		t.Fatal(err)
	}
	for role, handle := range children {
		if status, err := windows.WaitForSingleObject(handle, 5000); err != nil || status != windows.WAIT_OBJECT_0 {
			t.Fatal("provider tree survived service exit", role, status, err)
		}
	}
}

func TestProviderProcessTreeUnknownExitCannotReportCleanRelease(t *testing.T) {
	job := &providerProcessJob{handle: windows.InvalidHandle}
	if err := job.close(); !errors.Is(err, errProviderStopUncertain) {
		t.Fatalf("invalid ownership handle reported release: %v", err)
	}
	if err := job.close(); !errors.Is(err, errProviderStopUncertain) {
		t.Fatalf("repeat close hid uncertain release: %v", err)
	}
}
