package execution

import (
	"fmt"
	"golang.org/x/sys/unix"
)

func folderIdentity(path string) (string, error) {
	var st unix.Statx_t
	if err := unix.Statx(unix.AT_FDCWD, path, unix.AT_SYMLINK_NOFOLLOW, unix.STATX_TYPE|unix.STATX_INO|unix.STATX_BTIME, &st); err != nil {
		return "", err
	}
	if st.Mode&unix.S_IFMT != unix.S_IFDIR || st.Mask&unix.STATX_BTIME == 0 {
		return "", ErrUnsupported
	}
	return fmt.Sprintf("linux:%x:%x:%x:%x:%x", st.Dev_major, st.Dev_minor, st.Ino, st.Btime.Sec, st.Btime.Nsec), nil
}
