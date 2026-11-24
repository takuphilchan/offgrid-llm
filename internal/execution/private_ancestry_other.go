//go:build !windows && !linux && !darwin

package execution

import "os"

func checkPrivateAncestor(string, os.FileInfo) error { return ErrUnsupported }
