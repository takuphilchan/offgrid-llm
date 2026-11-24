//go:build linux || darwin

package storage

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"filippo.io/age"
)

func TestEncryptedBackupAgePassphraseCLIInteroperability(t *testing.T) {
	binary := os.Getenv("OFFGRID_TEST_AGE_CLI")
	if binary == "" {
		t.Skip("set OFFGRID_TEST_AGE_CLI to a prepared reference executable")
	}
	if !filepath.IsAbs(binary) {
		t.Fatal("reference CLI path must be absolute")
	}
	python, err := exec.LookPath("python3")
	if err != nil {
		t.Skip("python3 required for isolated reference CLI PTY")
	}
	const passphrase = "isolated reference passphrase caf\u00e9"
	runReference := func(mode, source, target string) {
		t.Helper()
		ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
		defer cancel()
		command := exec.CommandContext(ctx, python, "-c", backupAgePassphrasePTY, binary, mode, source, target)
		command.Stdin = strings.NewReader(passphrase)
		if output, err := command.CombinedOutput(); err != nil {
			t.Fatalf("reference PTY failed: %v %s", err, output)
		}
	}
	root, data := t.TempDir(), t.TempDir()
	if err := os.WriteFile(filepath.Join(data, "private.txt"), []byte("isolated reference data"), 0600); err != nil {
		t.Fatal(err)
	}
	legacy := filepath.Join(root, "legacy.zip")
	if _, err := BackupWorkspace(context.Background(), data, legacy, "interop-v1"); err != nil {
		t.Fatal(err)
	}
	imported := filepath.Join(root, "reference.age")
	runReference("encrypt", legacy, imported)
	identity, err := age.NewScryptIdentity(passphrase)
	if err != nil {
		t.Fatal(err)
	}
	identity.SetMaxWorkFactor(BackupScryptWorkFactor)
	read := BackupReadOptions{Identities: []age.Identity{identity}, RequireEncryption: true}
	if _, err := VerifyBackupWithOptions(context.Background(), imported, read); err != nil {
		t.Fatal(err)
	}
	if _, err := RestoreWorkspaceWithOptions(context.Background(), imported, filepath.Join(root, "restored"), "interop-v1", read); err != nil {
		t.Fatal(err)
	}
	recipient, err := age.NewScryptRecipient(passphrase)
	if err != nil {
		t.Fatal(err)
	}
	recipient.SetWorkFactor(BackupScryptWorkFactor)
	exported := filepath.Join(root, "offgrid.age")
	if _, err := BackupWorkspaceWithOptions(context.Background(), data, exported, "interop-v1", BackupWriteOptions{Recipients: []age.Recipient{recipient}}); err != nil {
		t.Fatal(err)
	}
	// This is an explicit independent CLI plaintext export fixture. OffGrid's
	// encrypted operations above create no plaintext intermediate archive.
	decrypted := filepath.Join(root, "reference-export.zip")
	runReference("decrypt", exported, decrypted)
	if manifest, err := VerifyBackup(context.Background(), decrypted); err != nil || len(manifest.Files) != 1 {
		t.Fatal("reference passphrase export invalid", err)
	}
	t.Log("reference passphrase import/export passed at production scrypt logN18")
}

const backupAgePassphrasePTY = `
import fcntl, os, pty, select, signal, subprocess, sys, termios, time
phrase=sys.stdin.buffer.read(4097)
assert 0<len(phrase)<=4096 and b'\n' not in phrase
binary,mode,source,target=sys.argv[1:]
master,slave=pty.openpty()
def attach():
    os.setsid()
    fcntl.ioctl(0,termios.TIOCSCTTY,0)
args=[binary,'--passphrase' if mode=='encrypt' else '--decrypt','--output',target,source]
child=subprocess.Popen(args,stdin=slave,stdout=slave,stderr=slave,preexec_fn=attach)
seen=bytearray()
def until(marker):
    deadline=time.monotonic()+15
    while marker not in seen:
        if time.monotonic()>deadline or child.poll() is not None: raise AssertionError('reference passphrase prompt unavailable')
        if select.select([master],[],[],0.1)[0]: seen.extend(os.read(master,4096))
        if len(seen)>65536: raise AssertionError('reference output exceeded bound')
try:
    until(b'Enter passphrase')
    os.write(master,phrase+b'\n')
    if mode=='encrypt':
        until(b'Confirm passphrase')
        os.write(master,phrase+b'\n')
    deadline=time.monotonic()+15
    while child.poll() is None:
        if time.monotonic()>deadline: raise AssertionError('reference operation deadline')
        if select.select([master],[],[],0.1)[0]: seen.extend(os.read(master,4096))
        if len(seen)>65536: raise AssertionError('reference output exceeded bound')
    assert child.returncode==0, 'reference operation failed'
    assert phrase not in seen, 'reference echoed protected input'
finally:
    if child.poll() is None:
        os.killpg(child.pid,signal.SIGTERM)
        try: child.wait(timeout=2)
        except subprocess.TimeoutExpired:
            os.killpg(child.pid,signal.SIGKILL);child.wait()
    os.close(master);os.close(slave)
`
