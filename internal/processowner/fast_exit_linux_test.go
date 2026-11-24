package processowner

import (
	"context"
	"errors"
	"os/exec"
	"testing"
)

func TestNamespaceFastExitRetainsOriginalExitAndSettlement(t *testing.T) {
	launcher, _ := namespaceFixture(t)
	for range 30 {
		command := exec.Command("/bin/sh", "-c", "test $$ = 1 || exit 8; test $(readlink /proc/self) -gt 1 || exit 9; exit 7")
		command.Env = []string{"PATH=/usr/bin:/bin"}
		job, err := Start(context.Background(), command, launcher)
		if err != nil {
			t.Fatal(err)
		}
		var exit *exec.ExitError
		if err := job.Wait(); !errors.As(err, &exit) || exit.ExitCode() != 7 {
			t.Fatal("namespace lost exact command exit/proc view", err)
		}
		if err := job.Close(); err != nil {
			t.Fatal("fast exit lost cleanup ownership", err)
		}
	}
}
