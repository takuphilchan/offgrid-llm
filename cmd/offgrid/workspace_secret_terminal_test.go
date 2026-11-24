//go:build linux || darwin

package main

import (
	"context"
	"os"
	"os/exec"
	"testing"
	"time"

	"golang.org/x/sys/unix"
)

func TestWorkspaceSecretInvalidDescriptorReturnsError(t *testing.T) {
	// A non-existent descriptor exercises POLLNVAL -> Read returning -1/EBADF
	// without closing or racing any real process-owned descriptor.
	file := os.NewFile(1<<30, "invalid-secret-fixture")
	defer file.Close()
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	data, err := workspaceTerminalInput(ctx, file)()
	if len(data) != 0 || err != unix.EBADF {
		t.Fatalf("invalid descriptor: %d bytes, %v", len(data), err)
	}
}

func TestWorkspaceSecretRealTerminal(t *testing.T) {
	python, err := exec.LookPath("python3")
	if err != nil {
		t.Skip("python3 required for isolated PTY test")
	}
	self, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	for _, mode := range []string{"confirm", "mismatch", "interrupt", "cancel", "oversize", "oversize-suffix"} {
		t.Run(mode, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			command := exec.CommandContext(ctx, python, "-c", workspaceSecretPTY, self, mode)
			if out, err := command.CombinedOutput(); err != nil {
				t.Fatalf("isolated PTY: %v\n%s", err, out)
			}
		})
	}
}

const workspaceSecretPTY = `
import os, pty, select, subprocess, sys, termios, time
master, slave = pty.openpty()
original = termios.tcgetattr(slave)
env = dict(os.environ, OFFGRID_WORKSPACE_SECRET_PTY=sys.argv[2])
child = subprocess.Popen([sys.argv[1], '-test.run=^TestWorkspaceSecretTerminalHelper$'], stdin=slave, stdout=slave, stderr=slave, env=env)
seen = bytearray()
def until(marker):
    limit=time.monotonic()+5
    while marker not in seen:
        if time.monotonic()>limit: raise AssertionError('protected prompt deadline')
        if select.select([master],[],[],0.1)[0]: seen.extend(os.read(master,4096))
until(b'Backup passphrase: ')
mode=sys.argv[2]
phrase='isolated café 日本語 😀'.encode()
if mode=='cancel': pass
elif mode=='interrupt': os.write(master,b'\x03')
elif mode=='oversize': os.write(master,b'x'*4097)
elif mode=='oversize-suffix': os.write(master,b'x'*4097+b'private-pasted-suffix\r')
else:
    os.write(master,phrase+b'\r')
    until(b'Confirm backup passphrase: ')
    os.write(master,(phrase if mode=='confirm' else b'different secret')+b'\r')
until(b'PROTECTED_INPUT_DONE')
assert child.wait(timeout=3)==0
assert termios.tcgetattr(slave)==original, 'terminal mode was not restored'
if mode=='oversize-suffix':
    assert not select.select([slave],[],[],0)[0], 'unread secret suffix escaped to the next terminal consumer'
assert phrase not in seen and b'different secret' not in seen and b'x'*30 not in seen and b'private-pasted-suffix' not in seen, 'input was echoed'
os.close(master);os.close(slave)
`
