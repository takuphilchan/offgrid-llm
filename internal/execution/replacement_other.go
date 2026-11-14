//go:build !windows && !linux

package execution

import "os"

func pinReplacementDirectory(root *os.Root, name string) (*replacementDirectory, error) {
	return nil, ErrUnsupported
}
func replacePinned(target *replacementDirectory, name string, private *replacementDirectory, staged, backup string) (string, error) {
	return "", ErrUnsupported
}
func syncReplacement(target, private *replacementDirectory) error          { return ErrUnsupported }
func qualifyReplacementVolume(target, private *replacementDirectory) error { return ErrUnsupported }
func movePinnedExclusive(from *replacementDirectory, name string, to *replacementDirectory, destination string) error {
	return ErrUnsupported
}
