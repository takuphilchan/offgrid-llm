//go:build linux

package models

import (
	"os"
	"syscall"
)

type capabilityFileStamp struct {
	Device, Inode, Links                                   uint64
	Size                                                   int64
	Mode, UID, GID                                         uint32
	ChangeSeconds, ChangeNanos, ModifySeconds, ModifyNanos int64
}

func capabilityChangeStamp(file *os.File) (capabilityFileStamp, bool) {
	var stat syscall.Stat_t
	var fs syscall.Statfs_t
	fd := int(file.Fd())
	if syscall.Fstat(fd, &stat) != nil || syscall.Fstatfs(fd, &fs) != nil || stat.Mode&syscall.S_IFMT != syscall.S_IFREG || stat.Size <= 0 {
		return capabilityFileStamp{}, false
	}
	// No reuse on network/Windows bridge filesystems with weaker change tracking.
	switch uint64(fs.Type) {
	case 0xef53, 0x58465342, 0x9123683e, 0x794c7630, 0x01021994: // ext, XFS, Btrfs, overlay, tmpfs
	default:
		return capabilityFileStamp{}, false
	}
	return capabilityFileStamp{uint64(stat.Dev), stat.Ino, uint64(stat.Nlink), stat.Size, stat.Mode, stat.Uid, stat.Gid,
		int64(stat.Ctim.Sec), int64(stat.Ctim.Nsec), int64(stat.Mtim.Sec), int64(stat.Mtim.Nsec)}, true
}
