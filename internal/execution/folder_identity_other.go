//go:build !linux && !darwin && !windows

package execution

import "os"

func folderIdentity(string) (string, error) { return "", ErrUnsupported }

func folderHandleIdentity(*os.File) (string, error) { return "", ErrUnsupported }
