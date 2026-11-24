package execution

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"golang.org/x/sys/windows"
)

func TestProcessCleanupJobFixture(t *testing.T) {
	if len(os.Args) < 4 || os.Args[len(os.Args)-3] != "offgrid-cleanup-job" {
		return
	}
	mode, path := os.Args[len(os.Args)-2], os.Args[len(os.Args)-1]
	if mode == "child" {
		if err := os.WriteFile(path, []byte(strconv.Itoa(os.Getpid())), 0600); err != nil {
			os.Exit(3)
		}
		time.Sleep(30 * time.Second)
		os.Exit(0)
	}
	if mode == "quick" {
		os.Exit(0)
	}
	if mode == "parent-fast" {
		child := exec.Command(os.Args[0], "-test.run=^TestProcessCleanupJobFixture$", "--", "offgrid-cleanup-job", "quick", path)
		if err := child.Run(); err != nil {
			os.Exit(6)
		}
		os.Exit(0)
	}
	child := exec.Command(os.Args[0], "-test.run=^TestProcessCleanupJobFixture$", "--", "offgrid-cleanup-job", "child", path)
	if err := child.Start(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(4)
	}
	for deadline := time.Now().Add(5 * time.Second); time.Now().Before(deadline); {
		if _, err := os.Stat(path); err == nil {
			os.Exit(0)
		}
		time.Sleep(5 * time.Millisecond)
	}
	os.Exit(5)
}

func TestProcessCleanupWindowsUnobservedDescendantRemainsUncertain(t *testing.T) {
	command := exec.Command(os.Args[0], "-test.run=^TestProcessCleanupJobFixture$", "--", "offgrid-cleanup-job", "parent-fast", filepath.Join(t.TempDir(), "unused"))
	// Suppress discovery deliberately: Windows notifications are not guaranteed.
	// A missing hint must remain uncertain even when the real child has exited.
	stop, wait, release, err := startOwnedJob(command, false)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { stop(); release() })
	if err = wait(); err != nil {
		t.Fatal(err)
	}
	if err = release(); err == nil {
		t.Fatal("lifetime process without retained handle became cleanup proof")
	}
	if err = release(); err == nil {
		t.Fatal("later empty Job list erased missing proof")
	}
}

func TestProcessCleanupWindowsCapturesShortLivedDescendant(t *testing.T) {
	for attempt := 0; attempt < 3; attempt++ {
		command := exec.Command(os.Args[0], "-test.run=^TestProcessCleanupJobFixture$", "--", "offgrid-cleanup-job", "parent-fast", filepath.Join(t.TempDir(), "unused"))
		stop, wait, release, err := startOwned(command)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { stop(); release() })
		if err = wait(); err != nil {
			t.Fatal(err)
		}
		if err = release(); err != nil {
			t.Fatalf("short-lived child was not owned through exit: %v", err)
		}
	}
}

func TestProcessCleanupWindowsJobIncludesLivingDescendant(t *testing.T) {
	path := filepath.Join(t.TempDir(), "child-pid")
	command := exec.Command(os.Args[0], "-test.run=^TestProcessCleanupJobFixture$", "--", "offgrid-cleanup-job", "parent", path)
	stop, wait, release, err := startOwned(command)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { stop(); release() })
	if err = wait(); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	pid, err := strconv.ParseUint(string(data), 10, 32)
	if err != nil {
		t.Fatal(err)
	}
	// The exact fixture-created child is still alive after its leader exits.
	handle, err := windows.OpenProcess(windows.SYNCHRONIZE|windows.PROCESS_QUERY_LIMITED_INFORMATION, false, uint32(pid))
	if err != nil {
		t.Fatal(err)
	}
	defer windows.CloseHandle(handle)
	if state, err := windows.WaitForSingleObject(handle, 0); err != nil || state != uint32(windows.WAIT_TIMEOUT) {
		t.Fatal("descendant already settled before release", state, err)
	}
	if err = release(); err != nil {
		t.Fatal(err)
	}
	if state, err := windows.WaitForSingleObject(handle, 0); err != nil || state != windows.WAIT_OBJECT_0 {
		t.Fatal("cleanup acknowledged a living Job descendant", state, err)
	}
	if err = release(); err != nil {
		t.Fatal("settlement not idempotent", err)
	}
}
