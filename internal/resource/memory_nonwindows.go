//go:build !windows

package resource

func detectNativeMemory(res *SystemResources) bool {
	return false
}
