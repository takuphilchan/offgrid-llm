package models

import (
	"context"
	"errors"
	"fmt"
	"os"
	"time"
)

// ErrFinalizeDownload means transferred bytes are retained in the partial file.
// Retrying can promote that file without transferring the model again.
var ErrFinalizeDownload = errors.New("failed to finalize download")

func finalizeDownload(ctx context.Context, temporary, destination string) error {
	return renameDownload(ctx, temporary, destination, os.Rename, 3*time.Second)
}

func renameDownload(ctx context.Context, temporary, destination string, rename func(string, string) error, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		err := rename(temporary, destination)
		if err == nil {
			return nil
		}
		// Antivirus/indexers can briefly hold a closed file on Windows. Never
		// delete a destination, disable scanning, or redownload on a sharing lock.
		if !isDownloadSharingError(err) || time.Now().After(deadline) {
			return fmt.Errorf("%w: %w", ErrFinalizeDownload, err)
		}
		timer := time.NewTimer(min(100*time.Millisecond, time.Until(deadline)))
		select {
		case <-ctx.Done():
			timer.Stop()
			return ctx.Err()
		case <-timer.C:
		}
	}
}
