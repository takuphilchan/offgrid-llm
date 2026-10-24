package resource

import (
	"os"
	"path/filepath"
	"testing"
)

func TestContainerMemoryBounds(t *testing.T) {
	for _, tc := range []struct {
		limit, used string
		total, free int64
	}{{"4294967296", "3221225472", 4096, 1024}, {"max", "0", 16000, 12000}, {"4294967296", "5000000000", 4096, 0}} {
		dir := t.TempDir()
		for k, v := range map[string]string{"memory.max": tc.limit, "memory.current": tc.used} {
			if err := os.WriteFile(filepath.Join(dir, k), []byte(v), 0600); err != nil {
				t.Fatal(err)
			}
		}
		res := &SystemResources{TotalRAM: 16000, AvailableRAM: 12000}
		limitContainerMemory(res, dir)
		if res.TotalRAM != tc.total || res.AvailableRAM != tc.free {
			t.Fatalf("wrong limit: %+v", res)
		}
	}
}
