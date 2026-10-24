//go:build linux || darwin

package main

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"reflect"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/x/term"
	"golang.org/x/sys/unix"
)

// Real PTYs catch SIGTTIN/SIGTTOU and restoration bugs that pipe-based process
// mocks and oneshot smoke checks cannot observe. No installed agent/config used.
func TestIntegrationForegroundTerminal(t *testing.T) {
	python, err := exec.LookPath("python3")
	if err != nil {
		t.Skip("python3 required for isolated PTY fixture")
	}
	self, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	for _, mode := range []string{"input", "interrupt", "eof", "raw", "failed-start", "cancel"} {
		t.Run(mode, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
			defer cancel()
			cmd := exec.CommandContext(ctx, python, "-c", integrationPTYProbe, self, mode)
			out, err := cmd.CombinedOutput()
			if err != nil || !strings.Contains(string(out), "RESTORED") {
				t.Fatalf("PTY %s: %v\n%s", mode, err, out)
			}
		})
	}
}

func TestIntegrationTerminalHelper(t *testing.T) {
	mode := os.Getenv("OFFGRID_TTY_FIXTURE")
	if mode == "" {
		return
	}
	if os.Getenv("OFFGRID_TTY_CHILD") == "1" {
		if mode == "raw" {
			if _, err := term.MakeRaw(os.Stdin.Fd()); err != nil {
				t.Fatal(err)
			}
			return
		}
		fmt.Println("CHILD_READY")
		line, err := bufio.NewReader(os.Stdin).ReadString('\n')
		if mode == "eof" {
			if !errors.Is(err, io.EOF) {
				t.Fatal(err)
			}
			return
		}
		if mode == "input" && (err != nil || line != "hello\n") {
			t.Fatalf("input=%q error=%v", line, err)
		}
		return
	}
	fd := int(os.Stdin.Fd())
	owner, err := unix.IoctlGetInt(fd, unix.TIOCGPGRP)
	if err != nil {
		t.Fatal(err)
	}
	before, err := term.GetState(os.Stdin.Fd())
	if err != nil {
		t.Fatal(err)
	}
	self, _ := os.Executable()
	if mode == "failed-start" {
		self = "/offgrid-nonexistent-test-executable"
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if mode == "cancel" {
		ctx, cancel = context.WithTimeout(context.Background(), 300*time.Millisecond)
		defer cancel()
	}
	err = externalCommand(ctx, hermesCommand, self, []string{"-test.run=^TestIntegrationTerminalHelper$"}, append(os.Environ(), "OFFGRID_TTY_CHILD=1"), os.Stdin, os.Stdout, os.Stderr)
	switch mode {
	case "interrupt":
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("interrupt: %v", err)
		}
	case "cancel":
		if !errors.Is(err, context.DeadlineExceeded) {
			t.Fatalf("cancel: %v", err)
		}
	case "failed-start":
		if err == nil {
			t.Fatal("unexpected successful start")
		}
	default:
		if err != nil {
			t.Fatal(err)
		}
	}
	afterOwner, err := unix.IoctlGetInt(fd, unix.TIOCGPGRP)
	if err != nil || owner != afterOwner {
		t.Fatalf("terminal owner not restored: %d -> %d: %v", owner, afterOwner, err)
	}
	after, err := term.GetState(os.Stdin.Fd())
	same := err == nil && reflect.DeepEqual(before, after)
	if err == nil && runtime.GOOS == "darwin" {
		// Darwin sets PENDIN when returning from raw to canonical input. It is
		// transient kernel bookkeeping, not a lost terminal setting. Compare
		// all defined settings while excluding that pending-input marker.
		a, b := before.Termios, after.Termios
		same = a.Iflag == b.Iflag && a.Oflag == b.Oflag && a.Cflag == b.Cflag && a.Lflag&^unix.PENDIN == b.Lflag&^unix.PENDIN && a.Cc == b.Cc && a.Ispeed == b.Ispeed && a.Ospeed == b.Ospeed
	}
	if !same {
		t.Fatalf("terminal settings not restored: %v; before=%+v after=%+v", err, before, after)
	}
	fmt.Println("RESTORED")
}

const integrationPTYProbe = `
import os,pty,select,signal,sys,time
pid,fd=pty.fork()
if pid==0:
    env=dict(os.environ,OFFGRID_TTY_FIXTURE=sys.argv[2],TERM='dumb',NO_COLOR='1')
    os.execve(sys.argv[1],[sys.argv[1],'-test.run=^TestIntegrationTerminalHelper$'],env)
data=b'';sent=False;finished=False
try:
    deadline=time.monotonic()+10
    while time.monotonic()<deadline:
        ready,_,_=select.select([fd],[],[],.1)
        if ready:
            try:chunk=os.read(fd,65536)
            except OSError:break
            if not chunk:break
            data+=chunk
            if b'CHILD_READY' in data and not sent:
                payload={'input':b'hello\n','interrupt':b'\x03','eof':b'\x04'}.get(sys.argv[2],b'')
                if payload:os.write(fd,payload)
                sent=True
        found,status=os.waitpid(pid,os.WNOHANG)
        if found:
            finished=True
            break
    while not finished and time.monotonic()<deadline:
        found,status=os.waitpid(pid,os.WNOHANG)
        finished=bool(found)
        if not finished:time.sleep(.01)
    print(data.decode(errors='replace'))
    if not finished or not os.WIFEXITED(status) or os.WEXITSTATUS(status)!=0 or b'RESTORED' not in data:
        raise SystemExit(1)
finally:
    if not finished:
        os.kill(pid,signal.SIGKILL);os.waitpid(pid,0)
    os.close(fd)
`
