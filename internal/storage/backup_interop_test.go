package storage

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/sha256"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"filippo.io/age"
)

// Opt-in external reference tool. The normal suite never downloads or discovers
// an executable; qualification records its source version and actual binary hash.
func TestEncryptedBackupAgeCLIInteroperability(t *testing.T) {
	binary := os.Getenv("OFFGRID_TEST_AGE_CLI")
	if binary == "" {
		t.Skip("set OFFGRID_TEST_AGE_CLI to an explicitly prepared age reference executable")
	}
	if !filepath.IsAbs(binary) {
		t.Fatal("reference CLI path must be absolute")
	}
	binaryBytes, err := os.ReadFile(binary)
	if err != nil {
		t.Fatal(err)
	}
	version, err := exec.Command(binary, "--version").CombinedOutput()
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("reference CLI %s sha256=%x", bytes.TrimSpace(version), sha256.Sum256(binaryBytes))
	x := backupTestKey(t)
	h, err := age.GenerateHybridIdentity()
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name, private, public string
		identity              age.Identity
		recipient             age.Recipient
	}{
		{"x25519", x.String(), x.Recipient().String(), x, x.Recipient()},
		{"hybrid", h.String(), h.Recipient().String(), h, h.Recipient()},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			data := filepath.Join(root, "data")
			if err := os.Mkdir(data, 0700); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(data, "private.txt"), []byte("isolated interop content"), 0600); err != nil {
				t.Fatal(err)
			}
			wal := exec.Command(os.Args[0], "-test.run=^TestBackupWALCrashHelper$")
			wal.Env = append(os.Environ(), "OFFGRID_BACKUP_WAL_TEST="+filepath.Join(data, "state.db"))
			if out, err := wal.CombinedOutput(); err != nil {
				t.Fatalf("WAL fixture: %v %s", err, out)
			}
			legacy := filepath.Join(root, "legacy.zip")
			if _, err := BackupWorkspace(context.Background(), data, legacy, "interop-v1"); err != nil {
				t.Fatal(err)
			}
			imported := filepath.Join(root, "reference.age")
			if out, err := exec.Command(binary, "--encrypt", "--recipient", tc.public, "--output", imported, legacy).CombinedOutput(); err != nil {
				t.Fatalf("reference encrypt: %v %s", err, out)
			}
			read := BackupReadOptions{Identities: []age.Identity{tc.identity}, RequireEncryption: true}
			if _, err := VerifyBackupWithOptions(context.Background(), imported, read); err != nil {
				t.Fatal(err)
			}
			target := filepath.Join(root, "restored")
			if _, err := RestoreWorkspaceWithOptions(context.Background(), imported, target, "interop-v1", read); err != nil {
				t.Fatal(err)
			}
			db, err := OpenSQLite(filepath.Join(target, "state.db"))
			if err != nil {
				t.Fatal(err)
			}
			var value string
			err = db.QueryRow("SELECT value FROM committed").Scan(&value)
			db.Close()
			if err != nil || value != "acknowledged before crash" {
				t.Fatal("reference archive lost WAL state", value, err)
			}
			exported := filepath.Join(root, "offgrid.age")
			if _, err := BackupWorkspaceWithOptions(context.Background(), data, exported, "interop-v1", BackupWriteOptions{Recipients: []age.Recipient{tc.recipient}}); err != nil {
				t.Fatal(err)
			}
			identityFile := filepath.Join(root, "identity.txt")
			if err := os.WriteFile(identityFile, []byte(tc.private), 0600); err != nil {
				t.Fatal(err)
			}
			// Reference decryption stays in memory, not a plaintext temp archive.
			plaintext, err := exec.Command(binary, "--decrypt", "--identity", identityFile, exported).Output()
			if err != nil {
				t.Fatal("reference decrypt", err)
			}
			archive, err := zip.NewReader(bytes.NewReader(plaintext), int64(len(plaintext)))
			if err != nil {
				t.Fatal(err)
			}
			manifest, files, err := readBackupManifest(archive)
			if err != nil {
				t.Fatal(err)
			}
			for _, item := range manifest.Files {
				if err := copyBackupFile(context.Background(), files[item.Path], item, io.Discard); err != nil {
					t.Fatal(err)
				}
			}
			t.Log(fmt.Sprintf("reference imported and exported %d files / %d bytes with committed WAL", len(manifest.Files), manifest.Bytes))
		})
	}
}
