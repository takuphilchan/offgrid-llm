package storage

import "fmt"

// BackupRetainedError identifies operation-owned temporary state that could not
// be confirmed removed. Callers can report its path without forwarding raw
// storage diagnostics or parsing user-controlled filenames out of Error().
type BackupRetainedError struct {
	Path      string
	Directory bool
	Cause     error
}

func (e *BackupRetainedError) Error() string {
	return fmt.Sprintf("temporary backup state retained at %s: %v", e.Path, e.Cause)
}
func (e *BackupRetainedError) Unwrap() error { return e.Cause }
