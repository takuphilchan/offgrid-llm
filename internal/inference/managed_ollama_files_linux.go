package inference

import (
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"golang.org/x/sys/unix"
)

// This restricted mode relies on administrator-provisioned DAC, not advisory
// read locks: the service and runtime cannot write any artifact or ancestor.
// Privileged administrator mutation and the trusted base OS are out of scope.
func managedLinuxUnprivileged() error {
	if os.Geteuid() == 0 || os.Getuid() != os.Geteuid() {
		return errors.New("managed Linux Ollama requires an unprivileged service account")
	}
	raw, err := providerProcRead("/proc/self/status")
	if err != nil {
		return err
	}
	count := 0
	for _, line := range strings.Split(string(raw), "\n") {
		fields := strings.Fields(line)
		if len(fields) != 2 {
			continue
		}
		switch fields[0] {
		case "CapInh:", "CapPrm:", "CapEff:", "CapAmb:":
			value, err := strconv.ParseUint(fields[1], 16, 64)
			if err != nil || value != 0 {
				return errors.New("managed Linux Ollama requires a service without process capabilities")
			}
			count++
		}
	}
	if count != 4 {
		return errors.New("managed Linux Ollama cannot verify service capabilities")
	}
	return nil
}

func managedLinuxRootPath(path string, directory bool) error {
	if !filepath.IsAbs(path) || filepath.Clean(path) != path || len(path) > 2048 {
		return errors.New("managed Linux artifact path is invalid")
	}
	for depth, current := 0, path; ; depth++ {
		if depth > 128 {
			return errors.New("managed Linux artifact path is too deep")
		}
		var stat unix.Stat_t
		if err := unix.Lstat(current, &stat); err != nil {
			return err
		}
		wantDirectory := depth > 0 || directory
		if stat.Uid != 0 || stat.Mode&0022 != 0 || stat.Mode&(unix.S_ISUID|unix.S_ISGID) != 0 || wantDirectory && stat.Mode&unix.S_IFMT != unix.S_IFDIR || !wantDirectory && (stat.Mode&unix.S_IFMT != unix.S_IFREG || stat.Nlink != 1) {
			return errors.New("managed Linux artifacts and ancestors require root ownership without group/other writes or links")
		}
		if current == "/" {
			return nil
		}
		current = filepath.Dir(current)
	}
}

func (lease *managedOllamaFiles) open(path string, directory bool) (*os.File, error) {
	if err := managedLinuxRootPath(path, directory); err != nil {
		return nil, err
	}
	before, err := os.Lstat(path)
	if err != nil {
		return nil, err
	}
	flags := unix.O_RDONLY | unix.O_CLOEXEC | unix.O_NOFOLLOW | unix.O_NONBLOCK
	if directory {
		flags |= unix.O_DIRECTORY
	}
	fd, err := unix.Open(path, flags, 0)
	if err != nil {
		return nil, err
	}
	file := os.NewFile(uintptr(fd), path)
	info, err := file.Stat()
	if err != nil || !os.SameFile(before, info) || managedLinuxRootPath(path, directory) != nil {
		file.Close()
		return nil, errors.New("managed Linux artifact identity changed")
	}
	lease.files = append(lease.files, file)
	return file, nil
}

func (lease *managedOllamaFiles) openState(path string) error {
	if !filepath.IsAbs(path) || filepath.Clean(path) != path {
		return errors.New("managed Linux state path is invalid")
	}
	// Unlike the writable contents, the state directory's parent is immutable
	// to the runtime: it cannot rename its HOME or substitute another state tree.
	if err := managedLinuxRootPath(filepath.Dir(path), true); err != nil {
		return err
	}
	before, err := os.Lstat(path)
	if err != nil {
		return err
	}
	file, err := openManagedLinuxState(path, before)
	if err != nil {
		return err
	}
	if err := managedLinuxRootPath(filepath.Dir(path), true); err != nil {
		file.Close()
		return err
	}
	lease.files = append(lease.files, file)
	return lease.acquireLinuxStateLock(file, path)
}

func openManagedLinuxState(path string, before os.FileInfo) (*os.File, error) {
	fd, err := unix.Open(path, unix.O_RDONLY|unix.O_DIRECTORY|unix.O_NOFOLLOW|unix.O_CLOEXEC, 0)
	if err != nil {
		return nil, err
	}
	file := os.NewFile(uintptr(fd), path)
	info, err := file.Stat()
	var stat unix.Stat_t
	if err != nil || unix.Fstat(fd, &stat) != nil || !os.SameFile(before, info) || stat.Mode&unix.S_IFMT != unix.S_IFDIR || stat.Uid != uint32(os.Geteuid()) || stat.Mode&0077 != 0 || stat.Mode&(unix.S_ISUID|unix.S_ISGID) != 0 {
		file.Close()
		return nil, errors.New("managed Linux state identity or private ownership changed")
	}
	return file, nil
}

func (lease *managedOllamaFiles) acquireLinuxStateLock(directory *os.File, path string) error {
	lockPath := filepath.Join(path, ".offgrid-managed-ollama.lock")
	lock, err := unix.Openat(int(directory.Fd()), ".offgrid-managed-ollama.lock", unix.O_RDWR|unix.O_CREAT|unix.O_NOFOLLOW|unix.O_NONBLOCK|unix.O_CLOEXEC, 0600)
	if err != nil {
		return err
	}
	file := os.NewFile(uintptr(lock), lockPath)
	var stat unix.Stat_t
	if unix.Fstat(lock, &stat) != nil || stat.Mode&unix.S_IFMT != unix.S_IFREG || stat.Nlink != 1 || stat.Uid != uint32(os.Geteuid()) || stat.Mode&0077 != 0 || unix.Flock(lock, unix.LOCK_EX|unix.LOCK_NB) != nil {
		file.Close()
		return errors.New("managed Linux state is already leased or unsafe")
	}
	// This is a cooperative lease, not protection against hostile same-UID code
	// unlinking its own lock. Leave the inode in place so cooperating openers
	// cannot lease a replacement inode during normal release/reacquire.
	lease.files = append(lease.files, file)
	return nil
}
