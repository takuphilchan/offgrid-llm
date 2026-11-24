package processowner

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"testing"
)

func TestNamespaceNestedRuntimeHelper(t *testing.T) {
	if os.Getenv("OFFGRID_NESTED_RUNTIME") != "1" {
		return
	}
	var runtime Runtime
	if err := json.Unmarshal([]byte(os.Getenv("OFFGRID_NESTED_PIN")), &runtime); err != nil {
		t.Fatal(err)
	}
	command := exec.Command("/bin/sh", "-c", "exit 0")
	command.Env = []string{"PATH=/usr/bin:/bin"}
	job, err := StartRuntime(context.Background(), command, runtime)
	if err != nil {
		t.Fatal(err)
	}
	defer job.Close()
	if err := job.Wait(); err != nil {
		t.Fatal(err)
	}
}

func TestNamespaceNestedRuntime(t *testing.T) {
	_, _ = namespaceFixture(t)
	runtime, err := SystemRuntime(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	encoded, err := json.Marshal(runtime)
	if err != nil {
		t.Fatal(err)
	}
	command := exec.Command(os.Args[0], "-test.run=^TestNamespaceNestedRuntimeHelper$")
	command.Env = append(os.Environ(), "OFFGRID_NESTED_RUNTIME=1", "OFFGRID_NESTED_PIN="+string(encoded))
	var output bytes.Buffer
	command.Stdout, command.Stderr = &output, &output
	job, err := StartRuntime(context.Background(), command, runtime)
	if err != nil {
		t.Fatal(err)
	}
	defer job.Close()
	if err := job.Wait(); err != nil {
		t.Fatal(err, output.String())
	}
}
