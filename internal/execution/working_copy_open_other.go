//go:build !linux && !darwin && !windows

package execution

import "os"

func qualifyWorkingCopyDirectory(_ *os.File) error { return ErrUnsupported }

// These platforms have no qualified no-follow worker-copy opener. Keep the
// proposal/admission error explicit rather than silently following links.
func openWorkingCopyRooted(_ *os.Root, _ string, _ bool) (*os.File, error) {
	return nil, ErrUnsupported
}
