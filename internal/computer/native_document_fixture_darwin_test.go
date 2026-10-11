//go:build darwin && cgo

package computer

import "testing"

func TestNativeDarwinDocumentFixture(t *testing.T) {
	runUnixDocumentFixture(t, func() (NativeAdapter, error) { return NewNativeDarwin() })
}
