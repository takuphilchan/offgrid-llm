package execution

import (
	"context"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"syscall"
	"testing"
	"time"

	"golang.org/x/sys/unix"
)

func TestWorkingCopyEscapeFixture(t *testing.T) {
	if len(os.Args) < 5 || os.Args[len(os.Args)-4] != "offgrid-copy-escape" {
		return
	}
	mode, path, socket := os.Args[len(os.Args)-3], os.Args[len(os.Args)-2], os.Args[len(os.Args)-1]
	if mode == "child" {
		if err := os.WriteFile(path, []byte(strconv.Itoa(os.Getpid())), 0600); err != nil {
			os.Exit(3)
		}
		connection, err := net.DialUnix("unixgram", nil, &net.UnixAddr{Name: socket, Net: "unixgram"})
		if err != nil {
			os.Exit(6)
		}
		if _, err := connection.Write([]byte("escaped-child")); err != nil {
			os.Exit(7)
		}
		connection.Close()
		time.Sleep(20 * time.Second)
		os.Exit(0)
	}
	child := exec.Command(os.Args[0], "-test.run=^TestWorkingCopyEscapeFixture$", "--", "offgrid-copy-escape", "child", path, socket)
	child.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	if err := child.Start(); err != nil {
		os.Exit(4)
	}
	for deadline := time.Now().Add(5 * time.Second); time.Now().Before(deadline); {
		if _, err := os.Stat(path + ".release"); err == nil {
			os.Exit(0)
		}
		time.Sleep(5 * time.Millisecond)
	}
	os.Exit(5)
}

func TestWorkingCopyLinuxNamespaceRetiresEscapedDescendantAndReleases(t *testing.T) {
	endpoint, store, binding, _ := workingCopyFixture(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	child, err := endpoint.PrepareWorker(ctx, workingCopyRequest(binding, "escaped-descendant"))
	if err != nil {
		t.Fatal(err)
	}
	token, err := opaqueToken()
	if err != nil {
		t.Fatal(err)
	}
	socket := "@offgrid-copy-" + token
	connection, err := net.ListenUnixgram("unixgram", &net.UnixAddr{Name: socket, Net: "unixgram"})
	if err != nil {
		t.Fatal(err)
	}
	defer connection.Close()
	control, err := connection.SyscallConn()
	if err != nil {
		t.Fatal(err)
	}
	var optionErr error
	if err := control.Control(func(fd uintptr) { optionErr = unix.SetsockoptInt(int(fd), unix.SOL_SOCKET, unix.SO_PASSCRED, 1) }); err != nil || optionErr != nil {
		t.Fatal(err, optionErr)
	}
	pidPath := filepath.Join(t.TempDir(), "fixture-child-pid")
	manager := child.Endpoint.Worker.Processes
	process, err := manager.Start(ctx, child.Context.Binding, ProcessSpec{Program: os.Args[0], Arguments: []string{"-test.run=^TestWorkingCopyEscapeFixture$", "--", "offgrid-copy-escape", "parent", pidPath, socket}})
	if err != nil {
		t.Fatal(err)
	}
	// Kernel credentials translate the child's PID into the observer namespace.
	// Never interpret its written namespace-local PID as a host process identity.
	connection.SetReadDeadline(time.Now().Add(5 * time.Second))
	data, ancillary := make([]byte, 64), make([]byte, unix.CmsgSpace(unix.SizeofUcred))
	n, size, _, _, err := connection.ReadMsgUnix(data, ancillary)
	if err != nil || string(data[:n]) != "escaped-child" {
		t.Fatal(n, err)
	}
	messages, err := unix.ParseSocketControlMessage(ancillary[:size])
	if err != nil || len(messages) != 1 {
		t.Fatal(messages, err)
	}
	credential, err := unix.ParseUnixCredentials(&messages[0])
	if err != nil || credential.Uid != uint32(os.Geteuid()) {
		t.Fatal(credential, err)
	}
	pidfd, err := unix.PidfdOpen(int(credential.Pid), 0)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		unix.PidfdSendSignal(pidfd, unix.SIGKILL, nil, 0)
		unix.Poll([]unix.PollFd{{Fd: int32(pidfd), Events: unix.POLLIN}}, 5000)
		unix.Close(pidfd)
	})
	if ready, err := unix.Poll([]unix.PollFd{{Fd: int32(pidfd), Events: unix.POLLIN}}, 0); err != nil || ready != 0 {
		t.Fatal("fixture child was not alive", ready, err)
	}
	if err := os.WriteFile(pidPath+".release", []byte("exit parent"), 0600); err != nil {
		t.Fatal(err)
	}
	result, err := manager.Wait(ctx, child.Context.Binding, process.Handle)
	if err != nil || result.State != "exited" || !ProcessDescendantsSettled(result) {
		t.Fatal(result, err)
	}
	if ready, err := unix.Poll([]unix.PollFd{{Fd: int32(pidfd), Events: unix.POLLIN}}, 0); err != nil || ready != 1 {
		t.Fatal("setsid child survived namespace init exit", ready, err)
	}
	if err := child.Close(); err != nil || !manager.cleanupConfirmed() {
		t.Fatal("worker close could not prove namespace settlement", err)
	}
	if err := child.Release(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(child.Context.Root); !os.IsNotExist(err) {
		t.Fatal("settled private copy was not released", err)
	}
	record, err := store.Inspect(ctx, child.Record.ID, binding)
	if err != nil || record.State != "released" || record.ReservedBytes != 0 {
		t.Fatal(record, err)
	}
}
