//go:build windows

package storage

import (
	"errors"
	"os"
	"time"

	"golang.org/x/sys/windows"
)

// A read-only status request or a scanner may briefly open the old snapshot
// without FILE_SHARE_DELETE. Retry only the atomic rename, never remove the old
// file or announce success before replacement. Permanent errors remain errors.
func replaceSnapshot(source, destination string) error {
	var err error
	for attempt := 0; attempt < 10; attempt++ {
		err = os.Rename(source, destination)
		if err == nil || (!errors.Is(err, windows.ERROR_SHARING_VIOLATION) && !errors.Is(err, windows.ERROR_LOCK_VIOLATION) && !errors.Is(err, windows.ERROR_ACCESS_DENIED)) {
			return err
		}
		if attempt < 9 {
			time.Sleep(25 * time.Millisecond)
		}
	}
	return err
}
