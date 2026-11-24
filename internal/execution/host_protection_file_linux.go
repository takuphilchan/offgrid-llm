package execution

import (
	"fmt"
	"golang.org/x/sys/unix"
)

func protectionRegularFileIdentity(path string) (string, error) {
	var stat unix.Statx_t
	const required = unix.STATX_TYPE | unix.STATX_INO | unix.STATX_BTIME
	if err := unix.Statx(unix.AT_FDCWD, path, unix.AT_SYMLINK_NOFOLLOW, required, &stat); err != nil {
		return "", err
	}
	if stat.Mode&unix.S_IFMT != unix.S_IFREG || stat.Mask&required != required {
		return "", ErrUnsupported
	}
	return fmt.Sprintf("linux-file:%x:%x:%x:%x:%x", stat.Dev_major, stat.Dev_minor, stat.Ino, stat.Btime.Sec, stat.Btime.Nsec), nil
}
