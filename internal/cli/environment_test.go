package cli

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"reflect"
	"strings"
	"testing"
)

type fakeRunner struct{ requests []CommandRequest }

func (runner *fakeRunner) Run(_ context.Context, request CommandRequest) error {
	runner.requests = append(runner.requests, request)
	return nil
}

func testEnvironment(stdout, stderr io.Writer, runner CommandRunner) Environment {
	return Environment{
		Stdin: strings.NewReader("input"), Stdout: stdout, Stderr: stderr,
		Getenv: func(name string) string { return "get:" + name },
		Setenv: func(string, string) error { return nil },
		HTTP:   &http.Client{}, LongHTTP: &http.Client{}, Exec: runner,
	}
}

func TestHasCommandUsesCanonicalRegistry(t *testing.T) {
	for _, name := range []string{"run", "bench", "benchmark-compare", "agents", "audio", "help"} {
		if !HasCommand(name) {
			t.Fatalf("migrated command %q missing", name)
		}
	}
	for _, name := range []string{"serve", "hermes", "unknown"} {
		if HasCommand(name) {
			t.Fatalf("legacy command %q should not dispatch through internal/cli", name)
		}
	}
}

func TestExecuteValidatesDependencies(t *testing.T) {
	var out, errOut bytes.Buffer
	runner := &fakeRunner{}
	base := testEnvironment(&out, &errOut, runner)
	tests := []struct {
		name   string
		mutate func(*Environment)
	}{
		{"stdin", func(e *Environment) { e.Stdin = nil }},
		{"stdout", func(e *Environment) { e.Stdout = nil }},
		{"stderr", func(e *Environment) { e.Stderr = nil }},
		{"getenv", func(e *Environment) { e.Getenv = nil }},
		{"setenv", func(e *Environment) { e.Setenv = nil }},
		{"http", func(e *Environment) { e.HTTP = nil }},
		{"long-http", func(e *Environment) { e.LongHTTP = nil }},
		{"runner", func(e *Environment) { e.Exec = nil }},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			env := base
			test.mutate(&env)
			if code := Execute(context.Background(), []string{"missing"}, env); code != 1 {
				t.Fatalf("code=%d", code)
			}
		})
	}
	if len(runner.requests) != 0 || out.Len() != 0 {
		t.Fatal("validation dispatched or wrote success output")
	}
}

func TestExecuteUsesInjectedEnvironmentAndCopiesArgs(t *testing.T) {
	var out, errOut bytes.Buffer
	runner := &fakeRunner{}
	set := map[string]string{}
	env := testEnvironment(&out, &errOut, runner)
	env.Options = InvocationOptions{JSON: true, NoColor: true}
	env.Setenv = func(k, v string) error { set[k] = v; return nil }
	args := []string{"sample", "value"}
	commands := map[string]Command{"sample": func(ctx context.Context, tail []string, got Environment) error {
		if !reflect.DeepEqual(tail, []string{"value"}) || got.Getenv("K") != "get:K" ||
			got.Stdin != env.Stdin || got.Stdout != env.Stdout || got.Stderr != env.Stderr ||
			got.HTTP != env.HTTP || got.LongHTTP != env.LongHTTP || got.Options != env.Options {
			return errors.New("injected environment mismatch")
		}
		tail[0] = "changed"
		if err := got.Setenv("K", "V"); err != nil {
			return err
		}
		_, _ = io.WriteString(got.Stdout, "ok")
		return got.Exec.Run(ctx, CommandRequest{Capability: "test", Args: []string{"one"}})
	}}
	if code := executeWithCommands(context.Background(), args, env, commands); code != 0 {
		t.Fatalf("code=%d stderr=%q", code, errOut.String())
	}
	if !reflect.DeepEqual(args, []string{"sample", "value"}) || out.String() != "ok" || set["K"] != "V" || len(runner.requests) != 1 {
		t.Fatalf("args=%v out=%q set=%v requests=%v", args, out.String(), set, runner.requests)
	}
}

func TestExecuteCancellationAndNoAmbientFallback(t *testing.T) {
	var out, errOut bytes.Buffer
	env := testEnvironment(&out, &errOut, &fakeRunner{})
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	called := false
	commands := map[string]Command{"sample": func(context.Context, []string, Environment) error { called = true; return nil }}
	if code := executeWithCommands(ctx, []string{"sample"}, env, commands); code != 130 || called {
		t.Fatalf("cancelled code=%d called=%v", code, called)
	}
	commands["sample"] = func(context.Context, []string, Environment) error { return context.Canceled }
	if code := executeWithCommands(context.Background(), []string{"sample"}, env, commands); code != 130 {
		t.Fatalf("command cancelled code=%d", code)
	}
	if code := Execute(context.Background(), []string{"sample"}, env); code != 1 || !strings.Contains(errOut.String(), "unavailable") {
		t.Fatalf("unregistered command code=%d stderr=%q", code, errOut.String())
	}
}

func TestExecuteVersionAliasesUseArgumentOptions(t *testing.T) {
	for _, alias := range []string{"version", "-v", "--version"} {
		var stdout, stderr bytes.Buffer
		env := testEnvironment(&stdout, &stderr, &fakeRunner{})
		env.Version = "v0.5.0"
		env.ReadFile = func(string) ([]byte, error) { t.Fatal("read unexpected"); return nil, nil }
		if code := Execute(context.Background(), []string{"--json", alias}, env); code != 0 || !strings.Contains(stdout.String(), `"version": "0.5.0"`) || stderr.Len() != 0 {
			t.Fatalf("alias=%q code=%d stdout=%q stderr=%q", alias, code, stdout.String(), stderr.String())
		}
	}
}

func TestExecuteHelpAliasesUseInjectedOutput(t *testing.T) {
	for _, alias := range []string{"help", "-h", "--help"} {
		var stdout, stderr bytes.Buffer
		env := testEnvironment(&stdout, &stderr, &fakeRunner{})
		env.Version = "0.5.0"
		if code := Execute(context.Background(), []string{alias}, env); code != 0 || !strings.Contains(stdout.String(), "offgrid <command>") || stderr.Len() != 0 {
			t.Fatalf("alias=%q code=%d stdout=%q stderr=%q", alias, code, stdout.String(), stderr.String())
		}
	}
}
