package main

import (
	"fmt"
	"os"
	"path/filepath"
)

// Capture only directory identity, not its contents or a grant. Resolve a
// relative CLI option now so later conversation turns cannot follow a new cwd.
func captureNativeLaunchFolder(folder string) (string, func() error, error) {
	root, err := filepath.Abs(folder)
	if err != nil {
		return "", nil, err
	}
	root, err = filepath.EvalSymlinks(root)
	if err != nil {
		return "", nil, err
	}
	initial, err := os.Stat(root)
	if err != nil {
		return "", nil, err
	}
	if !initial.IsDir() {
		return "", nil, fmt.Errorf("Workspace selection is not a folder")
	}
	check := func() error {
		current, err := os.Lstat(root)
		if err != nil || !current.IsDir() || !os.SameFile(initial, current) {
			return fmt.Errorf("The launch folder changed while this task was running. No workspace access was granted; select the intended folder again")
		}
		return nil
	}
	return root, check, nil
}
