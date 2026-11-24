package storage

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"filippo.io/age"
)

type failingBackupWriter struct {
	bytes.Buffer
	fail error
}

func (w *failingBackupWriter) Write(p []byte) (int, error) {
	if w.fail != nil {
		return 0, w.fail
	}
	return w.Buffer.Write(p)
}

func TestEncryptedBackupCipherPropagatesWriteAndFinalCloseFailure(t *testing.T) {
	for _, final := range []bool{false, true} {
		t.Run(map[bool]string{false: "write", true: "final-tag"}[final], func(t *testing.T) {
			output := &failingBackupWriter{}
			cipher, err := newBackupCipher(context.Background(), output, BackupWriteOptions{Recipients: []age.Recipient{backupTestKey(t).Recipient()}})
			if err != nil {
				t.Fatal(err)
			}
			defer cipher.clearKey()
			cause := errors.New("isolated destination failure")
			if final {
				if _, err := cipher.writer.Write([]byte("buffered final plaintext")); err != nil {
					t.Fatal(err)
				}
				output.fail = cause
				err = cipher.close()
			} else {
				output.fail = cause
				_, err = cipher.writer.Write(make([]byte, 2<<16))
			}
			if !errors.Is(err, cause) {
				t.Fatal("destination error lost", err)
			}
		})
	}
}

func TestEncryptedBackupRejectsReorderedChunksAndInvalidZIP(t *testing.T) {
	key := backupTestKey(t)
	// Uncompressed padding creates multiple full age chunks independently of ZIP
	// compression. A valid final chunk lets ReaderAt open without visiting them.
	good := encryptedBackupBytes(t, make([]byte, 4<<16), key.Recipient())
	header := bytes.Index(good, []byte("--- "))
	if header < 0 {
		t.Fatal("age header missing")
	}
	payload := header + bytes.IndexByte(good[header:], '\n') + 1 + 16
	const chunk = 65536 + 16
	reordered := append([]byte(nil), good...)
	copy(reordered[payload:payload+chunk], good[payload+chunk:payload+2*chunk])
	copy(reordered[payload+chunk:payload+2*chunk], good[payload:payload+chunk])
	for name, ciphertext := range map[string][]byte{
		"reordered":                 reordered,
		"authenticated-invalid-zip": encryptedBackupBytes(t, bytes.Repeat([]byte("not a ZIP"), 20), key.Recipient()),
	} {
		t.Run(name, func(t *testing.T) {
			root := t.TempDir()
			archive := filepath.Join(root, "bad.age")
			if err := os.WriteFile(archive, ciphertext, 0600); err != nil {
				t.Fatal(err)
			}
			read := BackupReadOptions{Identities: []age.Identity{key}, RequireEncryption: true}
			if _, err := VerifyBackupWithOptions(context.Background(), archive, read); err == nil || name == "reordered" && !strings.Contains(err.Error(), "authentication") {
				t.Fatal("invalid archive verified")
			}
			if _, err := RestoreWorkspaceWithOptions(context.Background(), archive, filepath.Join(root, "restore"), "v1", read); err == nil {
				t.Fatal("invalid archive restored")
			}
			entries, err := os.ReadDir(root)
			if err != nil || len(entries) != 1 {
				t.Fatal("invalid archive created restore state", entries, err)
			}
		})
	}
}
