package storage

import (
	"errors"
	"time"

	"golang.org/x/sys/windows"
)

func replaceDurableSnapshot(source, target string) (bool, error) {
	from, err := windows.UTF16PtrFromString(source)
	if err != nil {
		return false, err
	}
	to, err := windows.UTF16PtrFromString(target)
	if err != nil {
		return false, err
	}
	for attempt := 0; attempt < 10; attempt++ {
		err = windows.MoveFileEx(from, to, windows.MOVEFILE_REPLACE_EXISTING|windows.MOVEFILE_WRITE_THROUGH)
		if err == nil || (!errors.Is(err, windows.ERROR_SHARING_VIOLATION) && !errors.Is(err, windows.ERROR_LOCK_VIOLATION) && !errors.Is(err, windows.ERROR_ACCESS_DENIED)) {
			return err == nil, err
		}
		if attempt < 9 {
			time.Sleep(25 * time.Millisecond)
		}
	}
	return false, err
}
