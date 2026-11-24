package execution

import (
	"fmt"
	"os"
	"runtime"

	"golang.org/x/sys/unix"
)

func folderHandleIdentity(file *os.File) (string, error) {
	var st unix.Stat_t
	err := unix.Fstat(int(file.Fd()), &st)
	runtime.KeepAlive(file)
	if err != nil {
		return "", err
	}
	if st.Mode&unix.S_IFMT != unix.S_IFDIR {
		return "", ErrPath
	}
	return fmt.Sprintf("darwin:%x:%x:%x:%x", st.Dev, st.Ino, st.Btim.Sec, st.Btim.Nsec), nil
}

func folderIdentity(path string) (string, error) {
	var st unix.Stat_t
	if err := unix.Lstat(path, &st); err != nil {
		return "", err
	}
	if st.Mode&unix.S_IFMT != unix.S_IFDIR {
		return "", ErrPath
	}
	return fmt.Sprintf("darwin:%x:%x:%x:%x", st.Dev, st.Ino, st.Btim.Sec, st.Btim.Nsec), nil
}
