package cli

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

func TestBatchUsesInjectedServerAndProcessor(t *testing.T) {
	var out, errOut bytes.Buffer
	env := testEnvironment(&out, &errOut, &fakeRunner{})
	started, processed, slept := 0, 0, 0
	env.Batch = BatchDependencies{
		EnsureServer: func() error { return errors.New("offline") },
		StartServer:  func() error { started++; return nil },
		Sleep: func(duration time.Duration) {
			if duration != 2*time.Second {
				t.Fatalf("unexpected delay %s", duration)
			}
			slept++
		},
		ProcessFile: func(ctx context.Context, input, output string, concurrency int) error {
			if ctx == nil || input != "input.jsonl" || output != "output.jsonl" || concurrency != 8 {
				t.Fatalf("wrong batch request: %q %q %d", input, output, concurrency)
			}
			processed++
			return nil
		},
	}
	if code := Execute(context.Background(), []string{"batch", "process", "input.jsonl", "output.jsonl", "--concurrency", "8"}, env); code != 0 {
		t.Fatalf("batch exit=%d stderr=%q", code, errOut.String())
	}
	if started != 1 || processed != 1 || slept != 1 || !strings.Contains(out.String(), "Results written to: output.jsonl") {
		t.Fatalf("batch adapters not exercised: start=%d process=%d sleep=%d out=%q", started, processed, slept, out.String())
	}
	out.Reset()
	errOut.Reset()
	if code := Execute(context.Background(), []string{"batch", "invalid"}, env); code != 0 || !strings.Contains(errOut.String(), "Usage: offgrid batch process") {
		t.Fatalf("legacy invalid batch contract changed: code=%d stderr=%q", code, errOut.String())
	}
	out.Reset()
	if code := Execute(context.Background(), []string{"batch"}, env); code != 0 || !strings.Contains(out.String(), "Batch Processing") {
		t.Fatalf("batch help changed: code=%d stdout=%q", code, out.String())
	}
}
