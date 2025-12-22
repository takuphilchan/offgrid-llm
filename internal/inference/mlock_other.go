//go:build !linux
// +build !linux

package inference

// canUseMlock returns false on non-Linux platforms
// mlock is primarily useful on Linux; on macOS/Windows we skip the check
func canUseMlock() bool {
	return false
}
