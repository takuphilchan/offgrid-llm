package execution

import (
	"context"
	"encoding/json"
	"errors"
	"os/exec"
	"strings"
	"testing"
)

func TestSupervisedRuntimePinsArePrivateAndValidated(t *testing.T) {
	environment, err := supervisedProcessEnvironment()
	if err != nil {
		t.Fatal(err)
	}
	raw := strings.TrimPrefix(environment[0], "OFFGRID_EXECUTOR_PROCESS_RUNTIME=")
	t.Setenv("OFFGRID_EXECUTOR_PROCESS_RUNTIME", raw)
	if captureProcessEnvironment().lookup("OFFGRID_EXECUTOR_PROCESS_RUNTIME") != "" {
		t.Fatal("supervisor pins leaked into task environment")
	}
	var original supervisedRuntime
	if err := json.Unmarshal([]byte(raw), &original); err != nil {
		t.Fatal(err)
	}
	for name, mutate := range map[string]func(*supervisedRuntime){
		"version":  func(r *supervisedRuntime) { r.Version++ },
		"launcher": func(r *supervisedRuntime) { r.Runtime.Launcher.SHA256 = "sha256:" + strings.Repeat("0", 64) },
		"shell":    func(r *supervisedRuntime) { r.Runtime.Shell.SHA256 = "sha256:" + strings.Repeat("0", 64) },
	} {
		t.Run(name, func(t *testing.T) {
			pin := original
			mutate(&pin)
			encoded, _ := json.Marshal(pin)
			if _, err := supervisedProcessStarter(string(encoded)); err == nil {
				t.Fatal("invalid runtime accepted")
			}
		})
	}
	for _, invalid := range []string{`{}`, `{"Version":1,"unknown":true}`, raw + `{}`, strings.Repeat("x", 4097)} {
		if _, err := supervisedProcessStarter(invalid); err == nil {
			t.Fatal("invalid startup input accepted")
		}
	}
	source, _, _ := fixture(t)
	session, err := OpenSupervisedSession(source.config, raw)
	if err != nil {
		t.Fatal(err)
	}
	defer session.Close()
	if session.startProcess == nil {
		t.Fatal("supervised runtime missing")
	}
	encoded, err := json.Marshal(session.config)
	if err != nil || strings.Contains(string(encoded), original.Runtime.Launcher.SHA256) {
		t.Fatal("private runtime entered grant/HTTP configuration", err)
	}
}

func TestWorkingCopyInheritsSupervisedProcessOwner(t *testing.T) {
	endpoint, _, binding, _ := workingCopyFixture(t)
	expected := errors.New("fixture owner deliberately refused startup")
	called := false
	endpoint.Worker.Session.startProcess = func(*exec.Cmd) (func() error, func() error, func() error, error) {
		called = true
		return nil, nil, nil, expected
	}
	child, err := endpoint.PrepareWorker(context.Background(), workingCopyRequest(binding, "runtime-owner"))
	if err != nil {
		t.Fatal(err)
	}
	_, err = child.Endpoint.Worker.Processes.Start(context.Background(), child.Context.Binding, ProcessSpec{Program: "/bin/true"})
	if !called || !errors.Is(err, expected) {
		t.Fatal("child replaced owner", err)
	}
}
