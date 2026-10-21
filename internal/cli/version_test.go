package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
)

func TestResolveVersionUsesInjectedFiles(t *testing.T) {
	reads := []string{}
	read := func(path string) ([]byte, error) {
		reads = append(reads, path)
		if path == "/var/lib/offgrid/VERSION" {
			return []byte("0.4.18\n"), nil
		}
		return nil, errors.New("missing")
	}
	if got := ResolveVersion("v0.5.0", read); got != "0.5.0" || len(reads) != 0 {
		t.Fatalf("release got=%q reads=%v", got, reads)
	}
	if got := ResolveVersion("dev", read); got != "0.4.18" || strings.Join(reads, ",") != "VERSION,/var/lib/offgrid/VERSION" {
		t.Fatalf("development got=%q reads=%v", got, reads)
	}
}

func TestRunVersionUsesInjectedOutputAndDevice(t *testing.T) {
	var out, errOut bytes.Buffer
	env := Environment{
		Stdout: &out, Stderr: &errOut, Version: "v0.5.0",
		ReadFile: func(string) ([]byte, error) { t.Fatal("release version read file"); return nil, nil },
		Device:   func() DeviceInfo { return DeviceInfo{GPUName: "Fixture GPU", GPUMemoryMB: 2048} },
		Options:  InvocationOptions{JSON: true},
	}
	presenter := Presenter{Stdout: &out, Stderr: &errOut, JSON: true}
	if err := RunVersion(context.Background(), nil, env, presenter); err != nil {
		t.Fatal(err)
	}
	var value map[string]any
	if err := json.Unmarshal(out.Bytes(), &value); err != nil || value["version"] != "0.5.0" || value["gpu"] != "Fixture GPU" || value["gpu_memory_mb"] != float64(2048) || errOut.Len() != 0 {
		t.Fatalf("json=%q err=%v stderr=%q", out.String(), err, errOut.String())
	}
	out.Reset()
	env.Options.JSON = false
	presenter.JSON = false
	presenter.Style = PresentationStyle{Primary: "P", Bold: "B", Bolt: "*", Reset: "R", Muted: "M"}
	if err := RunVersion(context.Background(), nil, env, presenter); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "PB* OffGrid LLMR") || !strings.Contains(out.String(), "Fixture GPU (2048MB)") {
		t.Fatalf("text=%q", out.String())
	}
}
