package storage

import (
	"archive/zip"
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"strconv"

	"filippo.io/age"
)

const (
	BackupEncryptionNone         = "none"
	BackupEncryptionAge          = "age-v1"
	MaxBackupKeys                = 32
	BackupScryptWorkFactor       = 18
	maxBackupArchiveBytes  int64 = maxBackupBytes + 1<<30
)

// Options contain ephemeral key objects, never serialized application settings.
// Callers parse only native age keys, or construct a single scrypt recipient.
type BackupWriteOptions struct{ Recipients []age.Recipient }
type BackupReadOptions struct {
	Identities        []age.Identity
	RequireEncryption bool
}

func (o BackupWriteOptions) validate() error {
	if len(o.Recipients) > MaxBackupKeys {
		return errors.New("too many backup recipients")
	}
	for _, recipient := range o.Recipients {
		if recipient == nil {
			return errors.New("missing backup recipient")
		}
	}
	return nil
}

type backupCipher struct {
	writer     io.Writer
	closer     io.Closer
	fileKey    []byte
	encryption string
}

func newBackupCipher(ctx context.Context, destination io.Writer, options BackupWriteOptions) (*backupCipher, error) {
	result := &backupCipher{writer: &contextBackupWriter{ctx, destination, maxBackupArchiveBytes}, encryption: BackupEncryptionNone}
	if len(options.Recipients) == 0 {
		return result, nil
	}
	recipients := make([]age.Recipient, len(options.Recipients))
	for i, recipient := range options.Recipients {
		recipients[i] = captureBackupRecipient{recipient, result}
	}
	writer, err := age.Encrypt(result.writer, recipients...)
	if err != nil {
		result.clearKey()
		return nil, errors.New("could not initialize backup encryption; check recipient compatibility")
	}
	result.writer, result.closer, result.encryption = writer, writer, BackupEncryptionAge
	return result, nil
}

func (c *backupCipher) clearKey() { clear(c.fileKey); c.fileKey = nil }
func (c *backupCipher) close() error {
	if c.closer != nil {
		return c.closer.Close()
	}
	return nil
}
func (c *backupCipher) verificationOptions() BackupReadOptions {
	if c.encryption == BackupEncryptionNone {
		return BackupReadOptions{}
	}
	return BackupReadOptions{RequireEncryption: true, Identities: []age.Identity{age.NewInjectedFileKeyIdentity(c.fileKey)}}
}

type captureBackupRecipient struct {
	recipient age.Recipient
	cipher    *backupCipher
}

func (r captureBackupRecipient) Wrap(key []byte) ([]*age.Stanza, error) {
	r.capture(key)
	return r.recipient.Wrap(key)
}
func (r captureBackupRecipient) capture(key []byte) {
	if r.cipher.fileKey == nil {
		r.cipher.fileKey = append([]byte(nil), key...)
	}
}
func (r captureBackupRecipient) WrapWithLabels(key []byte) ([]*age.Stanza, []string, error) {
	r.capture(key)
	if labelled, ok := r.recipient.(age.RecipientWithLabels); ok {
		return labelled.WrapWithLabels(key)
	}
	stanzas, err := r.recipient.Wrap(key)
	return stanzas, nil, err
}

type contextBackupWriter struct {
	ctx       context.Context
	writer    io.Writer
	remaining int64
}

func (w *contextBackupWriter) Write(p []byte) (int, error) {
	if err := w.ctx.Err(); err != nil {
		return 0, err
	}
	if int64(len(p)) > w.remaining {
		return 0, errors.New("backup archive exceeds size limit")
	}
	n, err := w.writer.Write(p)
	w.remaining -= int64(n)
	return n, err
}

type contextBackupReaderAt struct {
	ctx    context.Context
	reader io.ReaderAt
}

func (r contextBackupReaderAt) ReadAt(p []byte, offset int64) (int, error) {
	if err := r.ctx.Err(); err != nil {
		return 0, err
	}
	n, err := r.reader.ReadAt(p, offset)
	// ReaderAt may legally return EOF with a complete final read. The age
	// reader does so; bounds preflight and ZIP readers need those bytes.
	if n == len(p) && errors.Is(err, io.EOF) {
		err = nil
	}
	return n, err
}

// Enforce received header cost before any identity invokes its KDF. Public
// identities and the private post-write verification identity share this gate.
type boundedBackupIdentity struct{ identity age.Identity }

func (i boundedBackupIdentity) Unwrap(stanzas []*age.Stanza) ([]byte, error) {
	if len(stanzas) > MaxBackupKeys {
		return nil, errors.New("backup has too many recipients")
	}
	for _, stanza := range stanzas {
		if stanza.Type != "scrypt" {
			continue
		}
		if len(stanzas) != 1 || len(stanza.Args) != 2 {
			return nil, errors.New("invalid backup passphrase header")
		}
		cost, err := strconv.Atoi(stanza.Args[1])
		if err != nil || cost < 1 || cost > BackupScryptWorkFactor {
			return nil, errors.New("backup passphrase work factor exceeds supported limit")
		}
	}
	return i.identity.Unwrap(stanzas)
}

type backupArchive struct {
	*zip.Reader
	file       *os.File
	encryption string
}

func (a *backupArchive) Close() error { return a.file.Close() }

func openBackupArchive(ctx context.Context, filename string, options BackupReadOptions) (_ *backupArchive, resultErr error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if len(options.Identities) > MaxBackupKeys {
		return nil, errors.New("too many backup identities")
	}
	before, err := os.Stat(filename)
	if err != nil {
		return nil, err
	}
	if !before.Mode().IsRegular() || before.Size() < 22 || before.Size() > maxBackupArchiveBytes {
		return nil, errors.New("backup must be a bounded regular archive file")
	}
	if err := backupBoundary(ctx, "archive_source_observed", filename); err != nil {
		return nil, err
	}
	file, err := openBackupInputFile(filename)
	if err != nil {
		return nil, err
	}
	defer func() {
		if resultErr != nil {
			_ = file.Close()
		}
	}()
	info, err := file.Stat()
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() || !os.SameFile(before, info) || info.Size() < 22 || info.Size() > maxBackupArchiveBytes {
		return nil, errors.New("backup must be a bounded regular archive file")
	}
	var reader io.ReaderAt = contextBackupReaderAt{ctx, file}
	size, encryption := info.Size(), BackupEncryptionNone
	magic := make([]byte, len("age-encryption.org/v1\n"))
	if _, err := reader.ReadAt(magic, 0); err != nil {
		return nil, err
	}
	if bytes.Equal(magic, []byte("age-encryption.org/v1\n")) {
		if len(options.Identities) == 0 {
			return nil, errors.New("encrypted backup requires an identity or passphrase")
		}
		identities := make([]age.Identity, len(options.Identities))
		for i, identity := range options.Identities {
			if identity == nil {
				return nil, errors.New("missing backup identity")
			}
			identities[i] = boundedBackupIdentity{identity}
		}
		reader, size, err = age.DecryptReaderAt(reader, size, identities...)
		if err != nil {
			return nil, backupAuthenticationError(ctx)
		}
		if size < 22 || size > maxBackupArchiveBytes {
			return nil, errors.New("decrypted backup exceeds archive limits")
		}
		reader = contextBackupReaderAt{ctx, reader}
		// ZIP may not visit gaps, comments or every encrypted payload chunk.
		// Authenticate the complete range before declaring the archive valid.
		if _, err := io.Copy(io.Discard, io.NewSectionReader(reader, 0, size)); err != nil {
			return nil, backupAuthenticationError(ctx)
		}
		encryption = BackupEncryptionAge
	} else if options.RequireEncryption || len(options.Identities) != 0 {
		return nil, errors.New("backup is not encrypted; no plaintext fallback was used")
	}
	if err := validateBackupZIPDirectory(reader, size); err != nil {
		return nil, err
	}
	archive, err := zip.NewReader(reader, size)
	if err != nil {
		return nil, err
	}
	return &backupArchive{Reader: archive, file: file, encryption: encryption}, nil
}

func backupAuthenticationError(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	return errors.New("backup authentication failed: incorrect key, unsupported passphrase cost, or corrupt archive")
}

func backupPathIdentity(path string, expected os.FileInfo) error {
	observed, err := os.Lstat(path)
	if err != nil {
		return err
	}
	if expected == nil || !os.SameFile(expected, observed) || observed.Mode().Type() != expected.Mode().Type() {
		return errors.New("backup temporary path identity changed; replacement was preserved")
	}
	return nil
}

func cleanupBackupPath(path string, directory bool, expected os.FileInfo, resultErr *error) {
	if err := backupPathIdentity(path, expected); err != nil {
		if !os.IsNotExist(err) {
			*resultErr = errors.Join(*resultErr, &BackupRetainedError{Path: path, Directory: directory, Cause: err})
		}
		return
	}
	var err error
	if directory {
		err = os.RemoveAll(path)
	} else {
		err = os.Remove(path)
	}
	if err != nil && !os.IsNotExist(err) {
		*resultErr = errors.Join(*resultErr, &BackupRetainedError{Path: path, Directory: directory, Cause: err})
	}
}
