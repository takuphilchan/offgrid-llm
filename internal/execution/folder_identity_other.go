//go:build !linux && !darwin && !windows

package execution

func folderIdentity(string) (string, error) { return "", ErrUnsupported }
