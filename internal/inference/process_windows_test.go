package inference

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"testing"
	"time"

	"golang.org/x/sys/windows"
)

func TestOwnedRuntimeHelper(t *testing.T) {
	role := os.Getenv("OFFGRID_TEST_OWNERSHIP")
	if role == "" {
		return
	}
	if role == "owner" {
		cmd := exec.Command(os.Args[0], "-test.run=^TestOwnedRuntimeHelper$")
		cmd.Env = append(os.Environ(), "OFFGRID_TEST_OWNERSHIP=child")
		configureBackgroundProcess(cmd)
		if err := startOwnedProcess(cmd); err != nil {
			t.Fatal(err)
		}
		if !ownedProcessAlive(cmd.Process) {
			t.Fatal("live child reported dead")
		}
		fmt.Println(cmd.Process.Pid)
	}
	for {
		time.Sleep(time.Second)
	}
}

func TestOwnedRuntimeDiesWithWindowsService(t *testing.T) {
	owner := exec.Command(os.Args[0], "-test.run=^TestOwnedRuntimeHelper$")
	owner.Env = append(os.Environ(), "OFFGRID_TEST_OWNERSHIP=owner")
	configureBackgroundProcess(owner)
	stdout, err := owner.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := owner.Start(); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = owner.Process.Kill(); _ = owner.Wait() }()
	pidLine := make(chan string, 1)
	go func() {
		scanner := bufio.NewScanner(stdout)
		if scanner.Scan() {
			pidLine <- scanner.Text()
		} else {
			pidLine <- ""
		}
	}()
	var pid int
	select {
	case line := <-pidLine:
		pid, err = strconv.Atoi(strings.TrimSpace(line))
		if err != nil {
			t.Fatalf("runtime not started: %q", line)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("owner startup timed out")
	}
	handle, err := windows.OpenProcess(windows.SYNCHRONIZE|windows.PROCESS_TERMINATE, false, uint32(pid))
	if err != nil {
		t.Fatal(err)
	}
	defer windows.CloseHandle(handle)
	// Clean up the exact fixture even if the ownership regression fails.
	defer windows.TerminateProcess(handle, 1)
	if err := owner.Process.Kill(); err != nil {
		t.Fatal(err)
	}
	status, err := windows.WaitForSingleObject(handle, 5000)
	if err != nil || status != windows.WAIT_OBJECT_0 {
		t.Fatalf("orphan survived owner exit: status=%d err=%v", status, err)
	}
}
