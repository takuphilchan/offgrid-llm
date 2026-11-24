package execution

import (
	"fmt"
	"os"
	"runtime"

	"golang.org/x/sys/unix"
)

func folderHandleIdentity(file *os.File) (string, error) {
	var st unix.Statx_t
	err := unix.Statx(int(file.Fd()), "", unix.AT_EMPTY_PATH|unix.AT_SYMLINK_NOFOLLOW, unix.STATX_TYPE|unix.STATX_INO|unix.STATX_BTIME, &st)
	runtime.KeepAlive(file)
	if err != nil {
		return "", err
	}
	const required = unix.STATX_TYPE | unix.STATX_INO | unix.STATX_BTIME
	if st.Mode&unix.S_IFMT != unix.S_IFDIR || st.Mask&required != required {
		return "", ErrUnsupported
	}
	return fmt.Sprintf("linux:%x:%x:%x:%x:%x", st.Dev_major, st.Dev_minor, st.Ino, st.Btime.Sec, st.Btime.Nsec), nil
}

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
