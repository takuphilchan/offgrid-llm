package models

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
)

// MeasureCapabilityFile hashes a caller-owned, authorized regular-file handle.
// It does not resolve model output as a path or trust a catalog checksum. The
// caller must hold the model installation/lifecycle lease through measurement
// and evaluation: stat checks are race detection, not a filesystem write lock.
// No stat-only cache is used, including for equal-size, equal-mtime replacement.
// Reading with ReadAt preserves the caller's file offset.
func MeasureCapabilityFile(ctx context.Context, file *os.File) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	if file == nil {
		return "", fmt.Errorf("missing capability artifact")
	}
	before, err := file.Stat()
	if err != nil {
		return "", err
	}
	if !before.Mode().IsRegular() || before.Size() <= 0 {
		return "", fmt.Errorf("capability artifact must be a nonempty regular file")
	}
	hash := sha256.New()
	reader := io.NewSectionReader(file, 0, before.Size())
	n, err := io.Copy(hash, contextPackageReader{ctx, reader})
	if err != nil {
		return "", err
	}
	if err := ctx.Err(); err != nil {
		return "", err
	}
	after, err := file.Stat()
	if err != nil {
		return "", err
	}
	if n != before.Size() || after.Size() != before.Size() || !after.ModTime().Equal(before.ModTime()) || after.Mode() != before.Mode() {
		return "", fmt.Errorf("capability artifact changed during measurement")
	}
	return hex.EncodeToString(hash.Sum(nil)), nil
}
