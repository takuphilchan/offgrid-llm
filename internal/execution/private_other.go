//go:build !windows && !linux && !darwin

package execution

func ProtectPrivatePath(string) error { return ErrUnsupported }
func CheckPrivatePath(string) error   { return ErrUnsupported }
