//go:build darwin && !cgo

package execution

// The native Darwin package requires cgo. A build without its ACL inspector
// cannot establish protected ancestry from POSIX mode bits alone.
func checkPrivateExtendedAuthority(string) error { return ErrUnsupported }
