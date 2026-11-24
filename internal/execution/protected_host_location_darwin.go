package execution

import (
	"bytes"
	"os"
	"path/filepath"
	"runtime"
	"unicode/utf8"
	"unsafe"

	"golang.org/x/sys/unix"
)

// EvalSymlinks preserves lookup spelling on case-insensitive volumes. Ask the
// opened vnode for its filesystem pathname instead; lexical case folding would
// conflate distinct directories on case-sensitive volumes.
func canonicalPrivateLocation(path string) (string, error) {
	fd, err := unix.Open(path, unix.O_RDONLY|unix.O_DIRECTORY|unix.O_CLOEXEC|unix.O_NONBLOCK, 0)
	if err != nil {
		// Missing components must remain distinguishable for pending roots.
		return "", &os.PathError{Op: "open", Path: path, Err: err}
	}
	file := os.NewFile(uintptr(fd), path)
	defer file.Close()
	identity, err := folderHandleIdentity(file)
	if err != nil {
		return "", ErrPath
	}
	buffer := new([unix.PathMax]byte)
	// FcntlInt carries the native pointer as an integer. Pin explicitly across
	// its Go wrappers, including any stack movement or garbage collection.
	var pin runtime.Pinner
	pin.Pin(buffer)
	defer pin.Unpin()
	_, err = unix.FcntlInt(file.Fd(), unix.F_GETPATH, int(uintptr(unsafe.Pointer(&buffer[0]))))
	runtime.KeepAlive(file)
	if err != nil {
		return "", ErrPath
	}
	end := bytes.IndexByte(buffer[:], 0)
	if end <= 0 || !utf8.Valid(buffer[:end]) {
		return "", ErrPath
	}
	canonical := string(buffer[:end])
	if !filepath.IsAbs(canonical) || filepath.Clean(canonical) != canonical {
		return "", ErrPath
	}
	actual, err := folderIdentity(canonical)
	if err != nil || actual != identity {
		return "", ErrPath
	}
	pinned, err := file.Stat()
	if err != nil {
		return "", ErrPath
	}
	current, err := os.Stat(path)
	if err != nil || !os.SameFile(pinned, current) {
		return "", ErrPath
	}
	return canonical, nil
}
