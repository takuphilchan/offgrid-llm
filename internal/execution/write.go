package execution

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

// MutationError preserves uncertainty after dispatch; callers must not turn it
// into a safe retry just because they did not receive a success response.
type MutationError struct {
	Cause   error
	Effects string
}

func (e *MutationError) Error() string {
	return fmt.Sprintf("execution_write_%s: %v", e.Effects, e.Cause)
}
func (e *MutationError) Unwrap() error { return e.Cause }

type WriteResult struct {
	Location
	Revision string `json:"revision"`
	Bytes    int    `json:"bytes"`
	Created  bool   `json:"created"`
	Verified bool   `json:"verified"`
}

// Write creates exclusively if expected is empty; replacing requires a complete
// preimage digest. The caller supplies bytes, never line-numbered renderings.
func (s *Session) Write(ctx context.Context, binding Binding, path string, data []byte, expected string) (WriteResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.authorize(ctx, binding); err != nil {
		return WriteResult{}, err
	}
	if !s.config.Permissions.Write {
		return WriteResult{}, ErrAuthority
	}
	if len(data) > MaxFileBytes {
		return WriteResult{}, ErrLimit
	}
	name, err := s.resolve(path, s.config.Directory)
	if err != nil {
		return WriteResult{}, err
	}
	if expected == "" {
		f, err := s.root.OpenFile(name, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
		if err != nil {
			return WriteResult{}, err
		}
		err = writeAndSync(f, data)
		if err != nil {
			return WriteResult{}, &MutationError{Cause: err, Effects: "partial"}
		}
	} else {
		if len(expected) != 64 {
			return WriteResult{}, ErrRevision
		}
		if _, err := hex.DecodeString(expected); err != nil {
			return WriteResult{}, ErrRevision
		}
		info, err := s.root.Lstat(name)
		if err != nil {
			return WriteResult{}, err
		}
		if !info.Mode().IsRegular() {
			return WriteResult{}, ErrUnsupported
		}
		before, err := s.read(ctx, name)
		if err != nil {
			return WriteResult{}, err
		}
		if digest(before) != expected {
			return WriteResult{}, ErrRevision
		}
		var nonce [16]byte
		if _, err = rand.Read(nonce[:]); err != nil {
			return WriteResult{}, err
		}
		staged := filepath.Join(filepath.Dir(name), ".offgrid-edit-"+hex.EncodeToString(nonce[:]))
		f, err := s.root.OpenFile(staged, os.O_CREATE|os.O_EXCL|os.O_WRONLY, info.Mode().Perm())
		if err != nil {
			return WriteResult{}, err
		}
		defer s.root.Remove(staged)
		if err = writeAndSync(f, data); err != nil {
			return WriteResult{}, &MutationError{Cause: err, Effects: "partial"}
		}
		current, err := s.read(ctx, name)
		if err != nil {
			return WriteResult{}, err
		}
		if digest(current) != expected {
			return WriteResult{}, ErrRevision
		}
		if err = s.authorize(ctx, binding); err != nil {
			return WriteResult{}, err
		}
		if err = s.root.Rename(staged, name); err != nil {
			return WriteResult{}, err
		}
	}
	// The write already happened. Cancellation/revocation must not be reported
	// as proof of no effects or cause another attempt through a different route.
	observed, err := s.read(context.WithoutCancel(ctx), name)
	if err != nil {
		return WriteResult{}, &MutationError{Cause: err, Effects: "unknown"}
	}
	if !bytes.Equal(observed, data) {
		return WriteResult{}, &MutationError{Cause: ErrRevision, Effects: "unknown"}
	}
	return WriteResult{Location: s.location(name), Revision: digest(observed), Bytes: len(observed), Created: expected == "", Verified: true}, nil
}

func writeAndSync(f *os.File, data []byte) error {
	_, writeErr := f.Write(data)
	if writeErr == nil {
		writeErr = f.Sync()
	}
	return errors.Join(writeErr, f.Close())
}
