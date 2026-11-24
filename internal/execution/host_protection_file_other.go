//go:build !linux && !windows

package execution

func protectionRegularFileIdentity(string) (string, error) { return "", ErrUnsupported }
