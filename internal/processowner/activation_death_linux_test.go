package processowner

import (
	"bytes"
	"context"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"golang.org/x/sys/unix"
)

func TestNamespaceEarlyOwnerHelper(t *testing.T) {
	if os.Getenv("OFFGRID_EARLY_OWNER") != "1" {
		return
	}
	command := exec.Command("/bin/sh", "-c", `printf effect > "$1"`, "fixture", os.Getenv("OFFGRID_EARLY_MARKER"))
	command.Env = []string{"PATH=/usr/bin:/bin"}
	launcher := Launcher{Path: os.Getenv("OFFGRID_NAMESPACE_LAUNCHER"), SHA256: os.Getenv("OFFGRID_NAMESPACE_DIGEST")}
	job, activation, err := prepareNamespace(context.Background(), command, launcher, 0)
	if err != nil {
		fmt.Fprintln(os.Stderr, "prepare:", err)
		os.Exit(2)
	}
	connection, err := net.ListenUnixgram("unixgram", &net.UnixAddr{Name: "@", Net: "unixgram"})
	if err != nil {
		fmt.Fprintln(os.Stderr, "connect:", err)
		activation.Close()
		job.Close()
		os.Exit(3)
	}
	_, _, err = connection.WriteMsgUnix([]byte("prepared-init"), unix.UnixRights(job.initFD), &net.UnixAddr{Name: os.Getenv("OFFGRID_NAMESPACE_SOCKET"), Net: "unixgram"})
	connection.Close()
	if err != nil {
		fmt.Fprintln(os.Stderr, "send:", err)
		activation.Close()
		job.Close()
		os.Exit(4)
	}
	// Retain activation without sending it. The parent test kills this owner
	// while PDEATHSIG is deliberately absent, reproducing the pre-arm boundary.
	for {
		time.Sleep(time.Second)
		runtime.KeepAlive(activation)
	}
}

func TestNamespacePreActivationOwnerDeath(t *testing.T) {
	_, connection := namespaceFixture(t)
	marker := filepath.Join(t.TempDir(), "must-not-run")
	owner := exec.Command(os.Args[0], "-test.run=^TestNamespaceEarlyOwnerHelper$")
	var output bytes.Buffer
	owner.Stderr = &output
	owner.Env = append(os.Environ(), "OFFGRID_EARLY_OWNER=1", "OFFGRID_EARLY_MARKER="+marker)
	if err := owner.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { owner.Process.Kill(); owner.Wait() })
	connection.SetReadDeadline(time.Now().Add(5 * time.Second))
	data, ancillary := make([]byte, 64), make([]byte, unix.CmsgSpace(unix.SizeofUcred)+unix.CmsgSpace(4))
	n, size, flags, _, err := connection.ReadMsgUnix(data, ancillary)
	if err != nil || flags&unix.MSG_CTRUNC != 0 || string(data[:n]) != "prepared-init" {
		owner.Process.Kill()
		owner.Wait()
		t.Fatal(n, flags, err, output.String())
	}
	messages, err := unix.ParseSocketControlMessage(ancillary[:size])
	if err != nil {
		t.Fatal(err)
	}
	fd := -1
	var credential *unix.Ucred
	for _, message := range messages {
		switch message.Header.Type {
		case unix.SCM_CREDENTIALS:
			credential, err = unix.ParseUnixCredentials(&message)
		case unix.SCM_RIGHTS:
			var descriptors []int
			descriptors, err = unix.ParseUnixRights(&message)
			for _, received := range descriptors {
				if fd >= 0 {
					unix.Close(received)
					t.Error("unexpected extra owner descriptor")
					continue
				}
				fd = received
				unix.CloseOnExec(fd)
				t.Cleanup(func() { unix.PidfdSendSignal(received, unix.SIGKILL, nil, 0); unix.Close(received) })
			}
		}
		if err != nil {
			t.Fatal(err)
		}
	}
	if fd < 0 || credential == nil || credential.Pid != int32(owner.Process.Pid) || credential.Uid != uint32(os.Geteuid()) {
		t.Fatal("foreign/unidentified owner evidence", credential, fd)
	}
	if _, err := os.Stat(marker); !os.IsNotExist(err) {
		t.Fatal("unactivated command ran", err)
	}
	if exited, err := pidfdExited(fd); err != nil || exited {
		t.Fatal("namespace init exited before owner death", exited, err)
	}
	if err := owner.Process.Kill(); err != nil {
		t.Fatal(err)
	}
	assertNamespaceChildrenExited(t, map[string]int{"unactivated-init": fd})
	if _, err := os.Stat(marker); !os.IsNotExist(err) {
		t.Fatal("owner death allowed command execution", err)
	}
}
