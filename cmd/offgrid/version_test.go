package main

import "testing"

func TestReleaseVersionUsesPackageIdentity(t *testing.T) {
	original := Version
	t.Cleanup(func() { Version = original })
	for _, tc := range []struct{ embedded, want string }{
		{"v0.4.5", "0.4.5"},
		{"0.4.5", "0.4.5"},
		{"v0.4.5-rc.1", "0.4.5-rc.1"},
		{"0.4.3-history-dev", "0.4.3-history-dev"},
		{"validation", "validation"},
		{"vulkan-preview", "vulkan-preview"},
	} {
		t.Run(tc.embedded, func(t *testing.T) {
			Version = tc.embedded
			if got := getVersion(); got != tc.want {
				t.Fatalf("getVersion() = %q; want %q", got, tc.want)
			}
		})
	}
}
