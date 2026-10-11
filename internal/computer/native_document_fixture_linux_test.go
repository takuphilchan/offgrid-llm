//go:build linux && cgo && offgrid_native

package computer

import "testing"

func TestNativeLinuxDocumentFixture(t *testing.T) {
	runUnixDocumentFixture(t, func() (NativeAdapter, error) { return NewNativeLinux() })
}
