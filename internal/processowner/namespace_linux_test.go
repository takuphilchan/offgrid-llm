package processowner

import (
	"bufio"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"

	"golang.org/x/sys/unix"
)

func TestProviderNamespaceHelper(t *testing.T) {
	role := os.Getenv("OFFGRID_NAMESPACE_ROLE")
	if role == "" {
		return
	}
	if role == "owner" {
		command := namespaceFixtureCommand("parent")
		pin := Launcher{Path: os.Getenv("OFFGRID_NAMESPACE_LAUNCHER"), SHA256: os.Getenv("OFFGRID_NAMESPACE_DIGEST")}
		if _, err := Start(context.Background(), command, pin); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(2)
		}
	} else {
		if role == "parent" && os.Getpid() != 1 {
			os.Exit(3)
		}
		stat, err := os.ReadFile("/proc/self/stat")
		if err != nil || !strings.HasPrefix(string(stat), fmt.Sprint(os.Getpid())+" ") {
			os.Exit(4)
		}
		if role == "parent" || role == "child" {
			next := "child"
			if role == "child" {
				next = "grandchild"
			}
			child := namespaceFixtureCommand(next)
			child.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
			if child.Start() != nil {
				os.Exit(5)
			}
		}
		connection, err := net.DialUnix("unixgram", nil, &net.UnixAddr{Name: os.Getenv("OFFGRID_NAMESPACE_SOCKET"), Net: "unixgram"})
		if err != nil {
			os.Exit(6)
		}
		_, err = connection.Write([]byte(role))
		connection.Close()
		if err != nil {
			os.Exit(7)
		}
		if role == "parent" {
			scanner := bufio.NewScanner(os.Stdin)
			if scanner.Scan() && scanner.Text() == "exit" {
				os.Exit(0)
			}
		}
	}
	for {
		time.Sleep(time.Second)
	}
}

func namespaceFixtureCommand(role string) *exec.Cmd {
	command := exec.Command(os.Args[0], "-test.run=^TestProviderNamespaceHelper$")
	command.Env = append(os.Environ(), "OFFGRID_NAMESPACE_ROLE="+role)
	return command
}

func namespaceFixture(t *testing.T) (Launcher, *net.UnixConn) {
	t.Helper()
	path, err := exec.LookPath("unshare")
	if err != nil {
		t.Skip("Linux namespace qualification unavailable: util-linux unshare is not installed")
	}
	path, err = filepath.EvalSymlinks(path)
	if err != nil {
		t.Fatal(err)
	}
	probe := exec.Command(path, "--user", "--map-root-user", "--pid", "--fork", "--mount-proc", "true")
	if output, err := probe.CombinedOutput(); err != nil {
		t.Skipf("Linux unprivileged namespace qualification unavailable: %v: %s", err, output)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	digest := sha256.Sum256(raw)
	pin := Launcher{Path: path, SHA256: "sha256:" + hex.EncodeToString(digest[:])}
	socket := filepath.Join(t.TempDir(), "peer.sock")
	connection, err := net.ListenUnixgram("unixgram", &net.UnixAddr{Name: socket, Net: "unixgram"})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { connection.Close() })
	control, err := connection.SyscallConn()
	if err != nil {
		t.Fatal(err)
	}
	var optionErr error
	if err := control.Control(func(fd uintptr) { optionErr = unix.SetsockoptInt(int(fd), unix.SOL_SOCKET, unix.SO_PASSCRED, 1) }); err != nil || optionErr != nil {
		t.Fatal(err, optionErr)
	}
	t.Setenv("OFFGRID_NAMESPACE_SOCKET", socket)
	t.Setenv("OFFGRID_NAMESPACE_LAUNCHER", pin.Path)
	t.Setenv("OFFGRID_NAMESPACE_DIGEST", pin.SHA256)
	return pin, connection
}

func namespaceFixtureChildren(t *testing.T, connection *net.UnixConn) map[string]int {
	t.Helper()
	result := map[string]int{}
	connection.SetReadDeadline(time.Now().Add(5 * time.Second))
	for range 3 {
		data, control := make([]byte, 128), make([]byte, unix.CmsgSpace(unix.SizeofUcred))
		n, oobn, _, _, err := connection.ReadMsgUnix(data, control)
		if err != nil {
			t.Fatal("namespace descendants did not report credentials", err)
		}
		messages, err := unix.ParseSocketControlMessage(control[:oobn])
		if err != nil || len(messages) != 1 {
			t.Fatal("namespace credential framing", err)
		}
		credential, err := unix.ParseUnixCredentials(&messages[0])
		if err != nil {
			t.Fatal(err)
		}
		fd, err := unix.PidfdOpen(int(credential.Pid), 0)
		if err != nil {
			t.Fatal(err)
		}
		result[string(data[:n])] = fd
		t.Cleanup(func() { unix.PidfdSendSignal(fd, syscall.SIGKILL, nil, 0); unix.Close(fd) })
	}
	if len(result) != 3 {
		t.Fatal("namespace fixture identities duplicated", result)
	}
	return result
}

func assertNamespaceChildrenExited(t *testing.T, children map[string]int) {
	t.Helper()
	for role, fd := range children {
		deadline := time.Now().Add(5 * time.Second)
		for {
			fds := []unix.PollFd{{Fd: int32(fd), Events: unix.POLLIN}}
			remaining := time.Until(deadline)
			if remaining <= 0 {
				t.Fatal("owned namespace descendant survived", role)
			}
			n, err := unix.Poll(fds, int(remaining.Milliseconds())+1)
			if errors.Is(err, unix.EINTR) {
				continue
			}
			if err != nil || n != 1 || fds[0].Revents&(unix.POLLIN|unix.POLLHUP) == 0 {
				t.Fatal("owned namespace descendant survived", role, n, err)
			}
			break
		}
	}
}

func TestProviderNamespaceRetiresSetsidDescendantsAndOtherRuntimeSurvives(t *testing.T) {
	pin, connection := namespaceFixture(t)
	first := namespaceFixtureCommand("parent")
	job, err := Start(context.Background(), first, pin)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { job.Close() })
	children := namespaceFixtureChildren(t, connection)
	second := namespaceFixtureCommand("parent")
	other, err := Start(context.Background(), second, pin)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { other.Close() })
	independent := namespaceFixtureChildren(t, connection)
	closed := make(chan error, 2)
	for range 2 {
		go func() { closed <- job.Close() }()
	}
	for range 2 {
		if err := <-closed; err != nil {
			t.Fatal(err)
		}
	}
	assertNamespaceChildrenExited(t, children)
	for role, fd := range independent {
		if exited, err := pidfdExited(fd); err != nil || exited {
			t.Fatal("unrelated runtime was terminated", role, err)
		}
	}
}

func TestProviderNamespaceInitExitKillsDescendants(t *testing.T) {
	pin, connection := namespaceFixture(t)
	command := namespaceFixtureCommand("parent")
	input, err := command.StdinPipe()
	if err != nil {
		t.Fatal(err)
	}
	defer input.Close()
	job, err := Start(context.Background(), command, pin)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { job.Close() })
	children := namespaceFixtureChildren(t, connection)
	if _, err := fmt.Fprintln(input, "exit"); err != nil {
		t.Fatal(err)
	}
	assertNamespaceChildrenExited(t, children)
	if err := job.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestProviderNamespaceServiceOwnerDeathKillsDescendants(t *testing.T) {
	_, connection := namespaceFixture(t)
	owner := namespaceFixtureCommand("owner")
	if err := owner.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { owner.Process.Kill(); owner.Wait() })
	children := namespaceFixtureChildren(t, connection)
	if err := owner.Process.Kill(); err != nil {
		t.Fatal(err)
	}
	assertNamespaceChildrenExited(t, children)
}

func TestNamespaceInitPidfdKillRetiresDescendants(t *testing.T) {
	pin, connection := namespaceFixture(t)
	command := namespaceFixtureCommand("parent")
	job, err := Start(context.Background(), command, pin)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { job.Close() })
	children := namespaceFixtureChildren(t, connection)
	if err := unix.PidfdSendSignal(job.initFD, syscall.SIGKILL, nil, 0); err != nil {
		t.Fatal(err)
	}
	assertNamespaceChildrenExited(t, children)
	if err := job.Close(); err != nil {
		t.Fatal("confirmed namespace shutdown could not retire", err)
	}
}

func TestProviderNamespaceMissingExecutableAndPinFailWithoutFallback(t *testing.T) {
	pin, _ := namespaceFixture(t)
	bad := pin
	bad.SHA256 = "sha256:" + strings.Repeat("a", 64)
	command := namespaceFixtureCommand("parent")
	if job, err := Start(context.Background(), command, bad); err == nil || job != nil || command.Process != nil {
		t.Fatal("invalid launcher pin started process")
	}
	command = exec.Command(filepath.Join(t.TempDir(), "missing"))
	command.Env = []string{"PATH=/usr/bin"}
	if job, err := Start(context.Background(), command, pin); err == nil || job != nil {
		t.Fatal("missing runtime was accepted")
	}
}

func TestProviderNamespaceLauncherFIFOReplacementCannotBlockOpen(t *testing.T) {
	path := filepath.Join(t.TempDir(), "launcher")
	data := []byte("isolated executable identity fixture")
	if err := os.WriteFile(path, data, 0700); err != nil {
		t.Fatal(err)
	}
	before, err := os.Lstat(path)
	if err != nil {
		t.Fatal(err)
	}
	digest := sha256.Sum256(data)
	pin := Launcher{Path: path, SHA256: "sha256:" + hex.EncodeToString(digest[:])}
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	if err := unix.Mkfifo(path, 0600); err != nil {
		t.Fatal(err)
	}
	result := make(chan error, 1)
	go func() {
		file, err := openLauncherFile(context.Background(), pin, before)
		if file != nil {
			file.Close()
		}
		result <- err
	}()
	select {
	case err := <-result:
		if err == nil {
			t.Fatal("replacement FIFO accepted")
		}
	case <-time.After(time.Second):
		// Unblock a broken implementation so this regression does not leak a
		// goroutine or make test cleanup hang; it still fails the deadline.
		fd, _ := unix.Open(path, unix.O_WRONLY|unix.O_NONBLOCK, 0)
		if fd >= 0 {
			unix.Close(fd)
		}
		t.Fatal("replacement FIFO blocked launcher open")
	}
}
