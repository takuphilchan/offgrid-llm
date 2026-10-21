//go:build !windows

package resource

func detectNativeRAM() (totalMB, availableMB int64, ok bool) {
	return 0, 0, false
}
