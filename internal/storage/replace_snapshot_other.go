//go:build !windows

package storage

import "os"

func replaceSnapshot(source, destination string) error { return os.Rename(source, destination) }
