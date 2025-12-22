//go:build linux
// +build linux

package inference

import (
	"log"
	"syscall"
)

// canUseMlock checks if the system has sufficient RLIMIT_MEMLOCK for mlock to work
// Returns false if RLIMIT_MEMLOCK is too low (common in containers/WSL/default Linux)
func canUseMlock() bool {
	// Use raw syscall for RLIMIT_MEMLOCK (8 on Linux)
	const RLIMIT_MEMLOCK = 8
	var rlimit syscall.Rlimit
	err := syscall.Getrlimit(RLIMIT_MEMLOCK, &rlimit)
	if err != nil {
		log.Printf("Cannot check RLIMIT_MEMLOCK: %v, disabling mlock", err)
		return false
	}

	// If limit is less than 1GB, mlock will likely fail for models
	// Most models need at least a few hundred MB
	const minMlockBytes uint64 = 1024 * 1024 * 1024 // 1GB
	if rlimit.Cur < minMlockBytes {
		log.Printf("RLIMIT_MEMLOCK too low (%d bytes), disabling mlock. Run 'ulimit -l unlimited' as root to enable.", rlimit.Cur)
		return false
	}

	return true
}
