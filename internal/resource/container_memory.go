package resource

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// Respect limits exposed in the container's cgroup namespace, not just host
// MemAvailable. Unknown/unlimited cgroups do not invent a capacity measurement.
func limitContainerMemory(res *SystemResources, root string) {
	for _, names := range [][2]string{{"memory.max", "memory.current"}, {"memory/memory.limit_in_bytes", "memory/memory.usage_in_bytes"}} {
		read := func(name string) (uint64, error) {
			data, err := os.ReadFile(filepath.Join(root, name))
			if err != nil {
				return 0, err
			}
			return strconv.ParseUint(strings.TrimSpace(string(data)), 10, 64)
		}
		limit, err := read(names[0])
		if err != nil || limit == 0 || limit>>20 > uint64(res.TotalRAM) {
			continue
		}
		used, err := read(names[1])
		if err != nil {
			continue
		}
		remaining := uint64(0)
		if used < limit {
			remaining = limit - used
		}
		res.TotalRAM = min(res.TotalRAM, int64(limit>>20))
		res.AvailableRAM = min(res.AvailableRAM, int64(remaining>>20))
		return
	}
}
