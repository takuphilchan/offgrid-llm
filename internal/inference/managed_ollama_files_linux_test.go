package inference

import (
	"os"
	"path/filepath"
	"testing"

	"golang.org/x/sys/unix"
)

func TestManagedOllamaLinuxStateLeaseAndUnsafeInputs(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("requires unprivileged service fixture")
	}
	state := t.TempDir()
	if err := os.Chmod(state, 0700); err != nil {
		t.Fatal(err)
	}
	first, second := &managedOllamaFiles{}, &managedOllamaFiles{}
	t.Cleanup(first.close)
	t.Cleanup(second.close)
	// Test the private opened-directory/lock boundary separately. Full factory
	// admission additionally requires root-owned ancestors, which this ordinary
	// temporary fixture intentionally lacks.
	open := func(lease *managedOllamaFiles) error {
		before, err := os.Lstat(state)
		if err != nil {
			return err
		}
		file, err := openManagedLinuxState(state, before)
		if err != nil {
			return err
		}
		lease.files = append(lease.files, file)
		return lease.acquireLinuxStateLock(file, state)
	}
	if err := open(first); err != nil {
		t.Fatal(err)
	}
	if err := open(second); err == nil {
		t.Fatal("accepted concurrent state owner")
	}
	first.close()
	second.close()
	if err := open(second); err != nil {
		t.Fatal(err)
	}
	second.close()
	if err := os.Chmod(state, 0755); err != nil {
		t.Fatal(err)
	}
	if err := open(first); err == nil {
		t.Fatal("accepted non-private state")
	}
	if err := os.Chmod(state, 0700); err != nil {
		t.Fatal(err)
	}
	lock := filepath.Join(state, ".offgrid-managed-ollama.lock")
	if err := os.Remove(lock); err != nil {
		t.Fatal(err)
	}
	if err := unix.Mkfifo(lock, 0600); err != nil {
		t.Fatal(err)
	}
	if err := open(first); err == nil {
		t.Fatal("accepted special lock object")
	}
}

func TestManagedOllamaLinuxRequiresAdministratorProvisionedArtifacts(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("requires unprivileged service fixture")
	}
	if err := managedLinuxUnprivileged(); err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(t.TempDir(), "runtime")
	if err := os.WriteFile(file, []byte("fixture"), 0500); err != nil {
		t.Fatal(err)
	}
	lease := &managedOllamaFiles{}
	defer lease.close()
	if _, err := lease.open(file, false); err == nil {
		t.Fatal("accepted user-writable ownership as immutable")
	}
	if err := lease.openState(filepath.Dir(file)); err == nil {
		t.Fatal("accepted service-replaceable state ancestor")
	}
	if err := managedLinuxRootPath("/", true); err != nil {
		t.Fatal(err)
	}
	if err := managedLinuxRootPath("/tmp", true); err == nil {
		t.Fatal("accepted writable artifact ancestor")
	}
	link := filepath.Join(filepath.Dir(file), "link")
	if err := os.Symlink(file, link); err != nil {
		t.Fatal(err)
	}
	if _, err := lease.open(link, false); err == nil {
		t.Fatal("accepted symbolic link")
	}
}

func TestManagedOllamaLinuxStateReplacementRejected(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state")
	if err := os.Mkdir(path, 0700); err != nil {
		t.Fatal(err)
	}
	before, err := os.Lstat(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(path, path+"-original"); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(path, 0700); err != nil {
		t.Fatal(err)
	}
	file, err := openManagedLinuxState(path, before)
	if file != nil {
		file.Close()
	}
	if err == nil {
		t.Fatal("accepted substituted state directory")
	}
}
