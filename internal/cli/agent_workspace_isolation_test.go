package cli

import (
	"strings"
	"testing"
)

func TestWorkspaceIsolationRequiresExplicitPinnedChoice(t *testing.T) {
	image := "sha256:" + strings.Repeat("a", 64)
	base := []string{"test-folder", "--access", "trusted", "--backend", "isolated_linux", "--image", image}
	folder, access, p, err := workspaceStartOptions(base)
	if err != nil || folder != "test-folder" || access != "trusted" || p.Image != image || p.Network != "none" || p.UID != 0 || p.MemoryBytes != 512<<20 {
		t.Fatal(folder, access, p, err)
	}
	for _, bad := range [][]string{{"test-folder", "--image", image}, {"test-folder", "--backend", "isolated_linux"}, {"test-folder", "--access", "unknown"}, append(append([]string{}, base...), "--network", "host"), append(append([]string{}, base...), "--image", image), append(append([]string{}, base...), "--memory-mib", "1")} {
		if _, _, _, err := workspaceStartOptions(bad); err == nil {
			t.Fatal("accepted ambiguous/unsafe options", bad)
		}
	}
}
