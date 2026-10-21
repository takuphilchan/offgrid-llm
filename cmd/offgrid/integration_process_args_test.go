package main

import (
	"bytes"
	"errors"
	"os/exec"
	"reflect"
	"strings"
	"testing"
)

func TestIntegrationLaunchersRejectUnboundedArgumentsBeforeSpawn(t *testing.T) {
	tooMany := make([]string, 129)
	for index := range tooMany {
		tooMany[index] = "argument"
	}
	for _, args := range [][]string{tooMany, {strings.Repeat("x", 64<<10+1)}, {"bad\x00arg"}} {
		if err := validateIntegrationArgs("fixture", args); err == nil {
			t.Fatalf("accepted invalid arguments: count=%d", len(args))
		}
		if err := runHermesProcess("definitely-not-installed", args, nil, false); err == nil {
			t.Fatal("Hermes attempted an unreviewed launch")
		}
		if _, err := openClawCommand("definitely-not-installed", args); err == nil {
			t.Fatal("OpenClaw attempted an unreviewed launch")
		}
	}
	if err := validateIntegrationArgs("fixture", []string{"agent", "exec", "test prompt"}); err != nil {
		t.Fatal(err)
	}
}

func TestIntegrationProcessDispatchIsInjectableWithoutHostSpawn(t *testing.T) {
	previous := runIntegrationProcess
	t.Cleanup(func() { runIntegrationProcess = previous })
	var captured [][]string
	runIntegrationProcess = func(command *exec.Cmd) error {
		captured = append(captured, append([]string(nil), command.Args...))
		if command.Stdout != nil {
			_, _ = command.Stdout.Write([]byte("fixture output"))
		}
		return nil
	}
	if err := runHermesProcess("fixture-hermes", []string{"model", "list"}, []string{"FIXTURE=1"}, false); err != nil {
		t.Fatal(err)
	}
	var output bytes.Buffer
	if err := runOpenClawCommand("fixture-openclaw", []string{"plugins", "list"}, nil, &output); err != nil {
		t.Fatal(err)
	}
	want := [][]string{{"fixture-hermes", "model", "list"}, {"fixture-openclaw", "plugins", "list"}}
	if !reflect.DeepEqual(captured, want) {
		t.Fatalf("process requests = %#v, want %#v", captured, want)
	}
	if output.String() != "fixture output" {
		t.Fatalf("injected output = %q", output.String())
	}
}

func TestIntegrationProcessFailureIsReturnedWithoutRetry(t *testing.T) {
	previous := runIntegrationProcess
	t.Cleanup(func() { runIntegrationProcess = previous })
	want := errors.New("fixture process failure")
	count := 0
	runIntegrationProcess = func(*exec.Cmd) error {
		count++
		return want
	}
	if err := runHermesProcess("fixture-hermes", []string{"doctor"}, nil, true); !errors.Is(err, want) {
		t.Fatalf("Hermes error = %v", err)
	}
	if err := runOpenClawCommand("fixture-openclaw", []string{"doctor"}, nil, nil); !errors.Is(err, want) {
		t.Fatalf("OpenClaw error = %v", err)
	}
	if count != 2 {
		t.Fatalf("process dispatch count = %d, want 2", count)
	}
}
