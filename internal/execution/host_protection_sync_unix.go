//go:build linux || darwin

package execution

import (
	"errors"
	"os"
)

func syncHostProtectionDirectory(path string) error {
	directory, err := os.Open(path)
	if err != nil {
		return err
	}
	return errors.Join(directory.Sync(), directory.Close())
}
