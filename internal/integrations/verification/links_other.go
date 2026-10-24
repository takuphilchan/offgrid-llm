//go:build !windows && !linux && !darwin

package verification

import "os"

func openRead(root *os.Root, name string) (*os.File, error) { return root.Open(name) }

func singleLink(*os.File) bool { return false }
