package processowner

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestNamespaceActivationEOFBarsCommandWithoutParentDeathSignal(t *testing.T) {
	launcher, _ := namespaceFixture(t)
	marker := filepath.Join(t.TempDir(), "must-not-run")
	command := exec.Command("/bin/sh", "-c", `printf effect > "$1"`, "fixture", marker)
	command.Env = []string{"PATH=/usr/bin:/bin"}
	// Disable PDEATHSIG only for this fixture. EOF must independently cover
	// parent loss before the kernel signal is armed, with no command effects.
	job, activation, err := prepareNamespace(context.Background(), command, launcher, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer job.Close()
	if _, err := os.Stat(marker); !os.IsNotExist(err) {
		t.Fatal("command ran before activation", err)
	}
	if exited, err := pidfdExited(job.initFD); err != nil || exited {
		t.Fatal("namespace init exited before activation EOF", exited, err)
	}
	if err := activation.Close(); err != nil {
		t.Fatal(err)
	}
	var exit *exec.ExitError
	if err := job.Wait(); !errors.As(err, &exit) || exit.ExitCode() != 125 {
		t.Fatal("EOF did not terminate activation gate", err)
	}
	if _, err := os.Stat(marker); !os.IsNotExist(err) {
		t.Fatal("parent-loss window executed command", err)
	}
	if err := job.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestNamespaceLiteralArgumentFixture(t *testing.T) {
	if len(os.Args) > 2 && os.Args[len(os.Args)-2] == "offgrid-namespace-literal" {
		fmt.Print(os.Args[len(os.Args)-1])
		os.Exit(0)
	}
}

func TestNamespaceActivationPreservesLiteralArguments(t *testing.T) {
	launcher, _ := namespaceFixture(t)
	marker := filepath.Join(t.TempDir(), "must-not-evaluate")
	literal := `$(touch "` + marker + `"); 'single' "double" $HOME` + "\nsecond line"
	command := exec.Command(os.Args[0], "-test.run=^TestNamespaceLiteralArgumentFixture$", "--", "offgrid-namespace-literal", literal)
	command.Env = []string{"PATH=/usr/bin:/bin"}
	var output bytes.Buffer
	command.Stdout = &output
	job, err := Start(context.Background(), command, launcher)
	if err != nil {
		t.Fatal(err)
	}
	defer job.Close()
	if err := job.Wait(); err != nil || output.String() != literal {
		t.Fatal("activation changed/evaluated arguments", output.String(), err)
	}
	if _, err := os.Stat(marker); !os.IsNotExist(err) {
		t.Fatal("literal argument became shell source", err)
	}
	if err := job.Close(); err != nil {
		t.Fatal(err)
	}
}
