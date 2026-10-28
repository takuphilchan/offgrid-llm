package execution

import (
	"fmt"
	"golang.org/x/sys/unix"
)

func folderIdentity(path string) (string, error) {
	var st unix.Stat_t
	if err := unix.Lstat(path, &st); err != nil {
		return "", err
	}
	if st.Mode&unix.S_IFMT != unix.S_IFDIR {
		return "", ErrPath
	}
	return fmt.Sprintf("darwin:%x:%x:%x:%x", st.Dev, st.Ino, st.Birthtimespec.Sec, st.Birthtimespec.Nsec), nil
}
