package cli

import (
	"bytes"
	"context"
	"strings"
	"testing"
)

func TestExecuteInfoPreservesEmptyJSONProjection(t *testing.T) {
	var stdout, stderr bytes.Buffer
	env := testEnvironment(&stdout, &stderr, &fakeRunner{})
	env.Info = func(context.Context) (InfoSnapshot, error) {
		return InfoSnapshot{Version: "0.4.18", ModelsDir: "fixture", OS: "test", Arch: "amd64"}, nil
	}
	if code := Execute(context.Background(), []string{"--json", "status"}, env); code != 0 {
		t.Fatalf("exit=%d stderr=%q", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), `"installed": null`) || !strings.Contains(stdout.String(), `"version": "0.4.18"`) {
		t.Fatalf("unexpected JSON projection: %q", stdout.String())
	}
}
