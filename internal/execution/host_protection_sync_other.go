//go:build !windows && !linux && !darwin

package execution

func syncHostProtectionDirectory(string) error { return ErrUnsupported }
