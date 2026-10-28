package main

import "testing"

func TestExecutorRequiresExplicitPrivateSupervisor(t *testing.T) {
	for _, args := range [][]string{nil, {"--stdio"}, {"--stdio", "--grant-file", "relative", "--state-dir", "relative"}} {
		if err := run(args); err == nil {
			t.Fatal("started without explicit supervisor", args)
		}
	}
}
