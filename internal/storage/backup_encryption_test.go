package storage

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"filippo.io/age"
)

func backupTestKey(t *testing.T) *age.X25519Identity {
	t.Helper()
	key, err := age.GenerateX25519Identity()
	if err != nil {
		t.Fatal(err)
	}
	return key
}

func encryptedBackupBytes(t *testing.T, data []byte, recipients ...age.Recipient) []byte {
	t.Helper()
	var buffer bytes.Buffer
	writer, err := age.Encrypt(&buffer, recipients...)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := writer.Write(data); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	return buffer.Bytes()
}

func TestEncryptedBackupRoundTripAndPublication(t *testing.T) {
	key, other := backupTestKey(t), backupTestKey(t)
	hybrid, err := age.GenerateHybridIdentity()
	if err != nil {
		t.Fatal(err)
	}
	password, _ := age.NewScryptRecipient("isolated test passphrase")
	password.SetWorkFactor(1) // Resource-bound mechanics; CLI production cost is 18.
	passwordID, _ := age.NewScryptIdentity("isolated test passphrase")
	for _, tc := range []struct {
		name       string
		recipients []age.Recipient
		identities []age.Identity
	}{
		{"x25519", []age.Recipient{key.Recipient()}, []age.Identity{key}},
		{"multiple", []age.Recipient{key.Recipient(), other.Recipient()}, []age.Identity{other}},
		{"hybrid", []age.Recipient{hybrid.Recipient()}, []age.Identity{hybrid}},
		{"passphrase", []age.Recipient{password}, []age.Identity{passwordID}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root, data := t.TempDir(), t.TempDir()
			const content = "private credential and conversation fixture"
			if err := os.WriteFile(filepath.Join(data, "state.txt"), []byte(content), 0600); err != nil {
				t.Fatal(err)
			}
			archive := filepath.Join(root, "archive.any-extension")
			options := BackupWriteOptions{Recipients: tc.recipients}
			manifest, err := BackupWorkspaceWithOptions(context.Background(), data, archive, "test-1", options)
			if err != nil || manifest.Encryption != BackupEncryptionAge {
				t.Fatal(manifest, err)
			}
			raw, _ := os.ReadFile(archive)
			if !bytes.HasPrefix(raw, []byte("age-encryption.org/v1\n")) || bytes.Contains(raw, []byte(content)) || bytes.Contains(raw, []byte("state.txt")) {
				t.Fatal("plaintext archive or inventory leaked")
			}
			entries, _ := os.ReadDir(root)
			if len(entries) != 1 {
				t.Fatal("temporary archive retained", entries)
			}
			read := BackupReadOptions{Identities: tc.identities, RequireEncryption: true}
			if _, err := VerifyBackupWithOptions(context.Background(), archive, read); err != nil {
				t.Fatal(err)
			}
			if _, err := VerifyBackup(context.Background(), archive); err == nil {
				t.Fatal("encrypted backup accepted without key")
			}
			if _, err := VerifyBackupWithOptions(context.Background(), archive, BackupReadOptions{Identities: []age.Identity{backupTestKey(t)}}); err == nil {
				t.Fatal("wrong key accepted")
			}
			target := filepath.Join(root, "restored")
			if _, err := RestoreWorkspaceWithOptions(context.Background(), archive, target, "test-1", read); err != nil {
				t.Fatal(err)
			}
			got, _ := os.ReadFile(filepath.Join(target, "state.txt"))
			if string(got) != content {
				t.Fatal("restored content changed")
			}
			if _, err := RestoreWorkspaceWithOptions(context.Background(), archive, target, "test-1", read); err == nil {
				t.Fatal("overwrote existing restore")
			}
			if _, err := BackupWorkspaceWithOptions(context.Background(), data, archive, "test-1", options); err == nil {
				t.Fatal("overwrote existing archive")
			}
		})
	}
	if _, err := newBackupCipher(context.Background(), io.Discard, BackupWriteOptions{Recipients: []age.Recipient{key.Recipient(), hybrid.Recipient()}}); err == nil {
		t.Fatal("recipient labels were dropped")
	}
	if _, err := newBackupCipher(context.Background(), io.Discard, BackupWriteOptions{Recipients: []age.Recipient{key.Recipient(), password}}); err == nil {
		t.Fatal("passphrase labels were dropped")
	}
}

func TestEncryptedBackupAuthenticatesUnvisitedZIPBytes(t *testing.T) {
	filename := makeBackupFixture(t, "state.txt", []byte("fixture"), nil)
	plain, _ := os.ReadFile(filename)
	eocd := len(plain) - 22
	central := int(binary.LittleEndian.Uint32(plain[eocd+16 : eocd+20]))
	// The ZIP reader can skip this padding; complete age authentication cannot.
	gap := bytes.Repeat([]byte("unvisited padding"), 16000)
	withGap := append(append(append([]byte(nil), plain[:central]...), gap...), plain[central:]...)
	binary.LittleEndian.PutUint32(withGap[len(withGap)-6:len(withGap)-2], uint32(central+len(gap)))
	key := backupTestKey(t)
	ciphertext := encryptedBackupBytes(t, withGap, key.Recipient())
	ciphertext[len(ciphertext)/2] ^= 1
	reader, size, err := age.DecryptReaderAt(bytes.NewReader(ciphertext), int64(len(ciphertext)), key)
	if err != nil {
		t.Fatal("fixture unexpectedly touched corrupt middle chunk", err)
	}
	z, err := zip.NewReader(reader, size)
	if err != nil {
		t.Fatal(err)
	}
	for _, item := range z.File {
		file, err := item.Open()
		if err != nil {
			t.Fatal(err)
		}
		_, err = io.Copy(io.Discard, file)
		file.Close()
		if err != nil {
			t.Fatal("fixture ZIP extraction did not skip corrupt chunk", err)
		}
	}
	archive := filepath.Join(t.TempDir(), "corrupt.age")
	if err := os.WriteFile(archive, ciphertext, 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := VerifyBackupWithOptions(context.Background(), archive, BackupReadOptions{Identities: []age.Identity{key}}); err == nil || !strings.Contains(err.Error(), "authentication") {
		t.Fatal("unvisited payload corruption accepted", err)
	}
}

func TestEncryptedBackupRejectsCorruptionAndPlaintextFallback(t *testing.T) {
	plainFile := makeBackupFixture(t, "state.txt", bytes.Repeat([]byte("content"), 20000), nil)
	plain, _ := os.ReadFile(plainFile)
	key := backupTestKey(t)
	good := encryptedBackupBytes(t, plain, key.Recipient())
	mutate := func(position int) []byte { b := append([]byte(nil), good...); b[position] ^= 1; return b }
	for name, data := range map[string][]byte{
		"header": mutate(35), "payload": mutate(len(good) / 2), "final": mutate(len(good) - 1),
		"truncate-tag": good[:len(good)-1], "truncate-half": good[:len(good)/2],
		"append": append(append([]byte(nil), good...), 0), "concatenate": append(append([]byte(nil), good...), good...),
	} {
		t.Run(name, func(t *testing.T) {
			root := t.TempDir()
			archive := filepath.Join(root, "bad.age")
			if err := os.WriteFile(archive, data, 0600); err != nil {
				t.Fatal(err)
			}
			read := BackupReadOptions{Identities: []age.Identity{key}, RequireEncryption: true}
			if _, err := VerifyBackupWithOptions(context.Background(), archive, read); err == nil {
				t.Fatal("corruption verified")
			}
			if _, err := RestoreWorkspaceWithOptions(context.Background(), archive, filepath.Join(root, "restore"), "0.4.3", read); err == nil {
				t.Fatal("corruption restored")
			}
			entries, _ := os.ReadDir(root)
			if len(entries) != 1 {
				t.Fatal("unauthenticated restore state created", entries)
			}
		})
	}
	if _, err := VerifyBackupWithOptions(context.Background(), plainFile, BackupReadOptions{Identities: []age.Identity{key}}); err == nil {
		t.Fatal("plaintext fallback with identity")
	}
	if _, err := VerifyBackupWithOptions(context.Background(), plainFile, BackupReadOptions{RequireEncryption: true}); err == nil {
		t.Fatal("plaintext fallback when encryption required")
	}
	if manifest, err := VerifyBackup(context.Background(), plainFile); err != nil || manifest.Encryption != BackupEncryptionNone {
		t.Fatal("legacy verification changed", err)
	}
}

func TestEncryptedBackupBoundsAndCancellation(t *testing.T) {
	identity := &countingBackupIdentity{}
	_, err := (boundedBackupIdentity{identity}).Unwrap([]*age.Stanza{{Type: "scrypt", Args: []string{"salt", "30"}}})
	if err == nil || identity.calls != 0 {
		t.Fatal("excessive KDF reached identity")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	var buffer bytes.Buffer
	writer := &contextBackupWriter{ctx, &buffer, 10}
	if _, err := writer.Write([]byte("private")); !errors.Is(err, context.Canceled) || buffer.Len() != 0 {
		t.Fatal(err)
	}
	writer = &contextBackupWriter{context.Background(), &buffer, 3}
	if _, err := writer.Write([]byte("long")); err == nil || buffer.Len() != 0 {
		t.Fatal("writer limit ignored")
	}
	key := backupTestKey(t)
	root := t.TempDir()
	if _, err := BackupWorkspaceWithOptions(ctx, t.TempDir(), filepath.Join(root, "cancelled.age"), "v1", BackupWriteOptions{Recipients: []age.Recipient{key.Recipient()}}); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	entries, _ := os.ReadDir(root)
	if len(entries) != 0 {
		t.Fatal("cancelled backup created output")
	}
}

func TestEncryptedBackupInvalidOptionsFailBeforePublication(t *testing.T) {
	for _, recipients := range [][]age.Recipient{{nil}, make([]age.Recipient, MaxBackupKeys+1)} {
		output := t.TempDir()
		if _, err := BackupWorkspaceWithOptions(context.Background(), t.TempDir(), filepath.Join(output, "invalid.age"), "v1", BackupWriteOptions{Recipients: recipients}); err == nil {
			t.Fatal("invalid recipients accepted")
		}
		entries, err := os.ReadDir(output)
		if err != nil || len(entries) != 0 {
			t.Fatal("invalid recipients created archive state", entries, err)
		}
	}
	key := backupTestKey(t)
	archive := filepath.Join(t.TempDir(), "archive.age")
	if _, err := BackupWorkspaceWithOptions(context.Background(), t.TempDir(), archive, "v1", BackupWriteOptions{Recipients: []age.Recipient{key.Recipient()}}); err != nil {
		t.Fatal(err)
	}
	for _, identities := range [][]age.Identity{{nil}, make([]age.Identity, MaxBackupKeys+1)} {
		if _, err := VerifyBackupWithOptions(context.Background(), archive, BackupReadOptions{Identities: identities}); err == nil {
			t.Fatal("invalid identities accepted")
		}
	}
}

type countingBackupIdentity struct{ calls int }

func (i *countingBackupIdentity) Unwrap([]*age.Stanza) ([]byte, error) {
	i.calls++
	return nil, age.ErrIncorrectIdentity
}
